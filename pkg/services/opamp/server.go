package opamp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	services_int "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger   *slog.Logger
	opampSrv server.OpAMPServer

	// Repository for agent data access
	agentRepo agentdomain.Repository

	// Keep remoteStatusStore for direct access during config sync checks

	// Connection tracking for active connections (protocol concern)
	mu       sync.RWMutex
	addrToId map[string]string
	idToConn map[string]types.Connection // agentID -> connection

	// Config store for OpAMP-specific config logic
	assignedConfigStore storage.KeyValue[*configv1alpha1.Config]

	services.Service
}

var _ services_int.OpAmpServerHandler = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	agentRepo agentdomain.Repository,
	assignedConfigStore storage.KeyValue[*configv1alpha1.Config],
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:              l,
		opampSrv:            opampSvr,
		agentRepo:           agentRepo,
		addrToId:            map[string]string{},
		idToConn:            map[string]types.Connection{},
		assignedConfigStore: assignedConfigStore,
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) start(ctx context.Context) error {
	addr := "127.0.0.1:4320"
	s.logger.With("addr", addr).Info("starting opamp server")
	settings := server.StartSettings{
		ListenEndpoint: addr,
		HTTPMiddleware: otelhttp.NewMiddleware("v1/opamp"),
		Settings: server.Settings{
			Callbacks: types.Callbacks{
				OnConnecting: func(request *http.Request) types.ConnectionResponse {
					return types.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: types.ConnectionCallbacks{
							OnConnected:        s.OnConnected,
							OnMessage:          s.OnMessage,
							OnConnectionClose:  s.OnConnectionClose,
							OnReadMessageError: s.OnReadMessageError,
						},
					}
				},
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
	return s.opampSrv.Stop(ctxca)
}

func (s *Server) OnConnected(ctx context.Context, conn types.Connection) {
	s.logger.With("addr", conn.Connection().LocalAddr().String()).Info("agent connected")
}

func (s *Server) calculateHash(agentToConfigMap *protobufs.AgentConfigMap) []byte {
	return util.HashAgentConfigMap(agentToConfigMap)
}

func (s *Server) constructConfig(ctx context.Context, agentID string) (*protobufs.AgentConfigMap, error) {
	logger := logutil.FromContext(ctx)
	assignedConfig, err := s.assignedConfigStore.Get(ctx, agentID)
	if grpcutil.IsErrorNotFound(err) {
		logger.Info("no assigned config, falling back to default config")
		return &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {
					ContentType: "text/yaml",
					Body:        []byte(otelconfig.DefaultOtelConfig),
				},
			},
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get assigned config: %w", err)
	}
	logger.Info("agent has an assigned config")
	// Use the same helper as ConfigServer for consistent config map structure
	return util.ProtoConfigToAgentConfigMap(assignedConfig), nil
}

func (s *Server) sendConfig(ctx context.Context, conn types.Connection, agentID string) error {
	s.logger.Log(ctx, logutil.LevelTrace, "sending config to agent")
	configMap, err := s.constructConfig(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to construct config : %w", err)
	}
	hash := s.calculateHash(configMap)

	return conn.Send(ctx, &protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config:     configMap,
			ConfigHash: hash,
		},
	})
}

func (s *Server) OnReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
	s.logger.
		With("remote-addr", conn.Connection().RemoteAddr().String()).
		With("msg", string(msgByte)).
		With("err", err).
		Error("failed to read / deserialize agent message")
}

func (s *Server) OnMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	instanceUID := string(message.InstanceUid)
	agentAddr := conn.Connection().RemoteAddr().String()

	// Resolve the persistent agentID: extract from description or use cached mapping
	// FIXME: AgentDescription may not always be set
	agentID := s.resolveAgentID(ctx, agentAddr, conn, message.AgentDescription)
	logger := s.logger.With("agent-id", agentID, "instance-uid", instanceUID)
	logger.With("sequenceNum", message.SequenceNum).Debug("received message from agent")

	ctx = logutil.WithContext(ctx, logger)

	resp := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}
	if agentID == "" {
		logger.Error("cannot persist agent data: no agent ID available")
		return resp
	}

	// Verify agent is registered before processing any messages
	registered, err := s.agentRepo.Exists(ctx, agentID)
	if err != nil {
		logger.With("err", err).Error("failed to check agent registration")
		return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to verify agent registration"))
	}
	if !registered {
		logger.Warn("rejecting message from unregistered agent")
		return ErrorResponse(message.InstanceUid, NewBadRequestError("agent not registered"))
	}

	// Update connection state and check for sequence gaps
	needsFullState := s.updateConnectionState(ctx, agentID, message)
	if message.RemoteConfigStatus != nil {
		if err := s.handleRemoteConfigStatus(ctx, conn, agentID, message.RemoteConfigStatus); err != nil {
			logger.With("err", err).Error("failed to handle remote config status message")
		}
	}

	if message.AgentDescription != nil {
		logger.Info("persisting agent description")
		if err := s.agentRepo.UpdateAttributes(ctx, agentID, message.AgentDescription); err != nil {
			logger.With("err", err).Error("failed to persist opamp agent-description")
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to persist agent description"))
		}
	}
	if message.Health != nil {
		logger.Info("persisting agent health")
		if err := s.agentRepo.UpdateHealth(ctx, agentID, message.Health); err != nil {
			logger.With("err", err).Error("failed to persist health")
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to persist agent health"))
		}
	}

	if message.EffectiveConfig != nil {
		logger.Info("persisting effective config")
		if err := s.agentRepo.UpdateEffectiveConfig(ctx, agentID, message.EffectiveConfig); err != nil {
			logger.With("err", err).Error("failed to persist effective config")
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to persist effective config"))
		}
	}
	if needsFullState {
		resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
		logger.Info("requesting full state report due to sequence gap")
	}
	return resp
}

// updateConnectionState updates the persisted connection state for an agent.
// Returns true if a full state report is needed (sequence gap or instance change detected).
func (s *Server) updateConnectionState(ctx context.Context, agentID string, msg *protobufs.AgentToServer) bool {
	// Try to get existing state from repository
	existingState, err := s.agentRepo.GetConnectionState(ctx, agentID)
	needsFullState := false

	now := time.Now()

	if errors.Is(err, agentdomain.ErrAgentNotFound) {
		// First message from this agent - create new state
		newState := agentdomain.ConnectionState{
			State:        agentdomain.StateConnected,
			ConnectedAt:  &now,
			LastSeen:     &now,
			InstanceUID:  msg.InstanceUid,
			Capabilities: agentdomain.Capabilities(msg.Capabilities),
			SequenceNum:  msg.SequenceNum,
		}
		if err := s.agentRepo.UpdateConnectionState(ctx, agentID, newState); err != nil {
			s.logger.With("err", err, "agent_id", agentID).Error("failed to persist connection state")
		}
		// Only request full state if the agent didn't start at sequence 0
		// A new agent starting at 0 is a clean start and doesn't need full state
		return msg.SequenceNum != 0
	} else if err != nil {
		// Actual storage error - log and request full state to be safe
		s.logger.With("err", err, "agent_id", agentID).Error("failed to get connection state")
		return true
	}

	// Check if this is a new instance (agent restarted)
	if !bytes.Equal(existingState.InstanceUID, msg.InstanceUid) {
		s.logger.With("agent_id", agentID).Info("agent instance changed, requesting full state")
		existingState.InstanceUID = msg.InstanceUid
		existingState.ConnectedAt = &now
		existingState.SequenceNum = 0
		needsFullState = true
	} else if msg.SequenceNum > 0 {
		// Check for sequence gap (status compression support)
		expectedSeq := existingState.SequenceNum + 1
		if msg.SequenceNum != expectedSeq {
			needsFullState = true
		}
	}

	// Always update LastSeen on every message
	existingState.LastSeen = &now
	existingState.State = agentdomain.StateConnected

	// Update capabilities if provided
	if msg.Capabilities != 0 {
		existingState.Capabilities = agentdomain.Capabilities(msg.Capabilities)
	}
	existingState.SequenceNum = msg.SequenceNum

	if err := s.agentRepo.UpdateConnectionState(ctx, agentID, *existingState); err != nil {
		s.logger.With("err", err, "agent_id", agentID).Error("failed to persist connection state")
	}

	return needsFullState
}

func (s *Server) handleRemoteConfigStatus(
	ctx context.Context,
	conn types.Connection,
	agentID string,
	remoteConfigStatus *protobufs.RemoteConfigStatus,
) error {
	logger := logutil.FromContext(ctx)

	// Get the assigned config and calculate its expected hash
	assignedConfigMap, err := s.constructConfig(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to construct assigned config: %w", err)
	}
	expectedHash := s.calculateHash(assignedConfigMap)

	// Compare agent's reported hash against the assigned config hash
	incomingHash := remoteConfigStatus.GetLastRemoteConfigHash()

	if bytes.Equal(expectedHash, incomingHash) {
		logger.Info("agent remote config up-to-date")
		// Persist the status
		if err := s.agentRepo.UpdateRemoteConfigStatus(ctx, agentID, remoteConfigStatus); err != nil {
			return fmt.Errorf("failed to persist remote config status: %w", err)
		}
		return nil
	}

	logger.Info("need to send remote config to agent",
		"expected_hash", fmt.Sprintf("%x", expectedHash),
		"agent_hash", fmt.Sprintf("%x", incomingHash))

	if err := s.sendConfig(ctx, conn, agentID); err != nil {
		return fmt.Errorf("failed to send config to remote: %w", err)
	}
	if err := s.agentRepo.UpdateRemoteConfigStatus(ctx, agentID, remoteConfigStatus); err != nil {
		return fmt.Errorf("failed to persist remote config status: %w", err)
	}
	return nil
}

// resolveAgentID returns the persistent agent ID, either by extracting it from the
// agent description or by looking it up from the address mapping.
// It also stores the connection for later use by NotifyConfigChange.
func (s *Server) resolveAgentID(ctx context.Context, agentAddr string, conn types.Connection, desc *protobufs.AgentDescription) string {
	// Try to extract from description first
	if desc != nil {
		if agentID := extractAgentID(desc); agentID != "" {
			s.mu.Lock()
			s.addrToId[agentAddr] = agentID
			s.idToConn[agentID] = conn
			s.mu.Unlock()
			// Note: Connection state is now updated in updateConnectionState
			return agentID
		}
	}
	// Fall back to cached mapping
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addrToId[agentAddr]
}

// extractAgentID extracts the persistent otelfleet agent ID from the agent description.
func extractAgentID(desc *protobufs.AgentDescription) string {
	for _, entry := range desc.IdentifyingAttributes {
		if entry.Key == supervisor.AttributeOtelfleetAgentId {
			return entry.Value.GetStringValue()
		}
	}
	return ""
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
	if err := s.sendConfig(ctx, conn, agentID); err != nil {
		s.logger.With("agent_id", agentID, "err", err).Error("failed to send config on notify")
	} else {
		s.logger.With("agent_id", agentID).Info("config pushed to agent")
	}
}

// Ensure Server implements ConfigChangeNotifier
var _ otelconfig.ConfigChangeNotifier = (*Server)(nil)

// GetConnectionState is needed for tests or external access to connection state.
func (s *Server) GetConnectionState(ctx context.Context, agentID string) (*v1alpha1.AgentConnectionState, error) {
	state, err := s.agentRepo.GetConnectionState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return agentdomain.ConnectionStateToProto(agentID, *state), nil
}
