package opamp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	resourcesv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/config"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/opamp/handler"
	opampsync "github.com/otelfleet/otelfleet/pkg/services/opamp/sync"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger         *slog.Logger
	opampSrv       server.OpAMPServer
	otlpServerAddr string

	// Keep remoteStatusStore for direct access during config sync checks

	// Connection tracking for active connections (protocol concern)
	mu       sync.RWMutex
	addrToId map[string]string
	idToConn map[string]types.Connection // agentID -> connection

	// all active connection handlers
	handlers []*handler.CollectorHandler

	// Config store for OpAMP-specific config logic
	collectorConfigs object.KeyValue[*resourcesv1alpha1.CollectorConfig]
	configFilterSync *opampsync.ConfigFilterSync

	deployMgr deployment.Manager
	services.Service
	otlpConfig *config.OTLPConfig
}

func NewServer(
	l *slog.Logger,
	resourceStorage object.TypeURLStore,
	otlpServerAddr string,
	otlpConfig *config.OTLPConfig,
	deployMgr deployment.Manager,
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:           l,
		deployMgr:        deployMgr,
		opampSrv:         opampSvr,
		addrToId:         map[string]string{},
		idToConn:         map[string]types.Connection{},
		otlpServerAddr:   otlpServerAddr,
		otlpConfig:       otlpConfig,
		collectorConfigs: object.NewKeyValueAdapter[*resourcesv1alpha1.CollectorConfig](resourceStorage),
		configFilterSync: opampsync.NewConfigFilterSync(opampsync.ConfigFilterSyncOptions{
			Logger:   l.With("component", "config-filter-sync"),
			Storage:  resourceStorage,
			Interval: opampsync.DefaultConfigFilterSyncInterval,
		}),
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) running(ctx context.Context) error {
	return s.configFilterSync.Running(ctx)
}

func (s *Server) start(ctx context.Context) error {
	if err := s.configFilterSync.Start(ctx); err != nil {
		return fmt.Errorf("failed to start config filter sync: %w", err)
	}

	addr := "127.0.0.1:4320"
	s.logger.With("addr", addr).Info("starting opamp server")
	settings := server.StartSettings{
		ListenEndpoint: addr,
		HTTPMiddleware: otelhttp.NewMiddleware("v1/opamp"),
		Settings: server.Settings{
			Callbacks: types.Callbacks{
				OnConnecting: s.OnConnecting,
			},
		},
	}
	if err := s.opampSrv.Start(settings); err != nil {
		s.logger.With("err", err.Error()).Error("failed to start opamp server")
		return fmt.Errorf("failed to start opamp server: %w", err)
	}

	return nil
}

func (s *Server) stop(failureCase error) error {
	ctxca, ca := context.WithTimeout(context.TODO(), time.Second)
	defer ca()
	s.configFilterSync.Stopping()
	return s.opampSrv.Stop(ctxca)
}

func (s *Server) OnConnecting(request *http.Request) types.ConnectionResponse {
	// TODO : handle authentication, authorization,
	// and maybe there is way to customize available capabilities here
	accept := s.handleInitialRequest(request)
	serverConnID := uuid.New().String()

	s.logger.With("server-conn-id", serverConnID).With("remote-addr", request.RemoteAddr).Info("assigned connection ID")

	if accept {
		handler := handler.NewCollectorHandler(
			context.TODO(),
			serverConnID,
			s.configFilterSync,
			s.logger,
			s.otlpServerAddr,
			s.otlpConfig,
			s.deployMgr,
			s.collectorConfigs,
		)
		return types.ConnectionResponse{
			Accept: accept,
			ConnectionCallbacks: types.ConnectionCallbacks{
				OnConnected:            handler.OnConnected,
				OnMessage:              handler.OnMessage,
				OnConnectionClose:      handler.OnConnectionClose,
				OnReadMessageError:     handler.OnReadMessageError,
				OnMessageResponseError: handler.OnMessageResponseError,
			},
		}
	}

	return types.ConnectionResponse{
		Accept: false,
	}

}

func (s *Server) handleInitialRequest(r *http.Request) bool {
	// TODO : handle authentication, authorization,
	customAuthHeader := r.Header.Get("X-Auth")
	if customAuthHeader != "" {
		s.logger.Info("agent requested custom authentication method")
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		s.logger.Info("agent requested http authentication method")
		return true
	}
	s.logger.With("type", authHeader).Info("agent could not be authenticated")
	return false
}
