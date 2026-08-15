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
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	resourcesv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/config"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	stypes "github.com/otelfleet/otelfleet/pkg/storage/types"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger         *slog.Logger
	opampSrv       server.OpAMPServer
	otlpServerAddr string

	// Repository for agent data access
	agentRepo agentdomain.Repository

	// Keep remoteStatusStore for direct access during config sync checks

	// Connection tracking for active connections (protocol concern)
	mu       sync.RWMutex
	addrToId map[string]string
	idToConn map[string]types.Connection // agentID -> connection

	// Config store for OpAMP-specific config logic
	assignedConfigStore stypes.KeyValue[*configv1alpha1.Config]

	// all active connection handlers
	handlers []*ServerAgentHandler

	collectorConfigs      stypes.KeyValue[*resourcesv1alpha1.CollectorConfig]
	configAssignmentStore stypes.KeyValue[*configv1alpha1.ConfigAssignment]
	configFilterSync      *ConfigFilterSync

	services.Service

	otlpConfig *config.OTLPConfig
}

func NewServer(
	l *slog.Logger,
	agentRepo agentdomain.Repository,
	assignedConfigStore stypes.KeyValue[*configv1alpha1.Config],
	configAssignmentStore stypes.KeyValue[*configv1alpha1.ConfigAssignment],
	resourceStorage schema.SchemaProto,
	otlpServerAddr string,
	otlpConfig *config.OTLPConfig,
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:                l,
		opampSrv:              opampSvr,
		agentRepo:             agentRepo,
		addrToId:              map[string]string{},
		idToConn:              map[string]types.Connection{},
		assignedConfigStore:   assignedConfigStore,
		configAssignmentStore: configAssignmentStore,
		otlpServerAddr:        otlpServerAddr,
		otlpConfig:            otlpConfig,
		collectorConfigs:      storage.NewProtoKVFromSchemaImpl[*resourcesv1alpha1.CollectorConfig](resourceStorage),
		configFilterSync: NewConfigFilterSync(ConfigFilterSyncOptions{
			Logger:   l.With("component", "config-filter-sync"),
			Storage:  resourceStorage,
			Interval: defaultConfigFilterSyncInterval,
		}),
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) running(ctx context.Context) error {
	return s.configFilterSync.running(ctx)
}

func (s *Server) start(ctx context.Context) error {
	if err := s.configFilterSync.start(ctx); err != nil {
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
	s.configFilterSync.stopping()
	return s.opampSrv.Stop(ctxca)
}

func (s *Server) OnConnecting(request *http.Request) types.ConnectionResponse {
	// TODO : handle authentication, authorization,
	// and maybe there is way to customize available capabilities here
	accept := s.handleInitialRequest(request)
	serverConnID := uuid.New().String()

	s.logger.With("server-conn-id", serverConnID).With("remote-addr", request.RemoteAddr).Info("assigned connection ID")

	if accept {
		handler := NewServerAgentHandler(context.TODO(), serverConnID, s.agentRepo, s.assignedConfigStore, s.collectorConfigs, s.configAssignmentStore, s.configFilterSync, s.logger, s.otlpServerAddr, s.otlpConfig)
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

func (s *Server) OnConnectionClose(conn types.Connection) {
	remoteAddr := conn.Connection().RemoteAddr().String()
	logger := s.logger.With("remote_addr", remoteAddr)
	logger.Info("agent disconnected")

	s.mu.Lock()
	agentID, ok := s.addrToId[remoteAddr]
	if ok {
		delete(s.addrToId, remoteAddr)
		delete(s.idToConn, agentID)
	}
	s.mu.Unlock()

	if !ok {
		logger.Error("agent not tracked in addr to persistent ID map")
		return
	}

	// Persist disconnected state
	ctx := context.Background()
	existingState, err := s.agentRepo.GetConnectionState(ctx, agentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			// Agent never had state stored - this shouldn't happen but is not critical
			logger.Warn("no connection state found for disconnected agent")
		} else {
			// Actual storage error - log at error level
			logger.With("err", err).Error("failed to get connection state for disconnected agent")
		}
		return
	}
	now := time.Now()
	existingState.State = agentdomain.StateDisconnected
	existingState.DisconnectedAt = &now
	if err := s.agentRepo.UpdateConnectionState(ctx, agentID, *existingState); err != nil {
		logger.With("err", err).Error("failed to persist disconnected state")
	}
}

// NotifyConfigChange triggers an immediate config push to the specified agent.
// This implements the otelconfig.ConfigChangeNotifier interface.
// If the agent is not connected, this is a no-op (the agent will receive
// the config when it reconnects).
func (s *Server) NotifyConfigChange(agentID string) {
	s.mu.RLock()
	conn, ok := s.idToConn[agentID]
	s.mu.RUnlock()

	if !ok {
		s.logger.With("agent_id", agentID).Debug("agent not connected, config will be sent on reconnect")
		return
	}

	// Send config immediately
	ctx := context.Background()
	for _, handler := range s.handlers {
		if handler.agentID == nil {
			continue
		}
		if agentID == *handler.agentID {
			if err := handler.sendConfig(ctx, conn, agentID); err != nil {
				s.logger.With("agent_id", agentID, "err", err).Error("failed to send config on notify")
			} else {
				s.logger.With("agent_id", agentID).Info("config pushed to agent")
			}
		}
	}
}

// GetConnectionState is needed for tests or external access to connection state.
func (s *Server) GetConnectionState(ctx context.Context, agentID string) (*v1alpha1.AgentConnectionState, error) {
	state, err := s.agentRepo.GetConnectionState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return agentdomain.ConnectionStateToProto(agentID, *state), nil
}
