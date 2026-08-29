package opamp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1"
	resourcesv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/auth/creds"
	"github.com/otelfleet/otelfleet/pkg/config"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	"github.com/otelfleet/otelfleet/pkg/event"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/opamp/handler"
	opampsync "github.com/otelfleet/otelfleet/pkg/services/opamp/sync"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger         *slog.Logger
	opampSrv       server.OpAMPServer
	otlpServerAddr string

	certConfig *config.CertConfig

	collectorCAPEM []byte

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
	reporter   event.EventSink

	nameGen util.NameGenerator

	authenticator   authenticator.Authenticator
	credentialStore creds.Store
}

func NewServer(
	l *slog.Logger,
	resourceStorage object.TypeURLStore,
	otlpServerAddr string,
	otlpConfig *config.OTLPConfig,
	deployMgr deployment.Manager,
	reporter event.EventSink,
	certConfig *config.CertConfig,
	authenticator authenticator.Authenticator,
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:           l,
		certConfig:       certConfig,
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
		reporter:      reporter,
		nameGen:       *util.NewNameGenerator(nil),
		authenticator: authenticator,
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

	addr := "0.0.0.0:4320"
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
	if s.certConfig != nil {
		cert, err := tls.LoadX509KeyPair(
			s.certConfig.Server.CertFile,
			s.certConfig.Server.KeyFile,
		)
		if err != nil {
			return fmt.Errorf("load OpAMP server TLS key pair: %w", err)
		}
		settings.TLSConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}

		s.credentialStore, err = creds.NewInMem(s.certConfig.Collectors)
		if err != nil {
			return fmt.Errorf("initialize collector credential store: %w", err)
		}
		s.collectorCAPEM, err = os.ReadFile(s.certConfig.Collectors.CaCertFile)
		if err != nil {
			return fmt.Errorf("read collector CA certificate: %w", err)
		}
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

func (s *Server) OnConnecting(req *http.Request) types.ConnectionResponse {
	// TODO : handle authentication, authorization,
	// and maybe there is way to customize available capabilities here
	principal, err := s.authenticator.AuthenticateRequest(req.Context(), req)
	if err != nil {
		s.reporter.Error(req.Context(), []*v1alpha1.EventRef{}, &v1alpha1.EventDetails{
			Reason: fmt.Sprintf("rejected agent connection %s : %s", req.RemoteAddr, err.Error()),
		})
		return types.ConnectionResponse{
			Accept: false,
		}
	}
	serverConnID := uuid.New().String()
	s.logger.With("server-conn-id", serverConnID).With("remote-addr", req.RemoteAddr).Info("assigned connection ID")
	handler := handler.NewCollectorHandler(
		context.TODO(),
		serverConnID,
		s.configFilterSync,
		s.logger,
		s.otlpServerAddr,
		s.otlpConfig,
		s.deployMgr,
		s.collectorConfigs,
		s.reporter,
		s.nameGen,
		principal,
		s.credentialStore,
		s.collectorCAPEM,
	)
	return types.ConnectionResponse{
		Accept: true,
		ConnectionCallbacks: types.ConnectionCallbacks{
			OnConnected:            handler.OnConnected,
			OnMessage:              handler.OnMessage,
			OnConnectionClose:      handler.OnConnectionClose,
			OnReadMessageError:     handler.OnReadMessageError,
			OnMessageResponseError: handler.OnMessageResponseError,
		},
	}
}
