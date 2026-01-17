package opamp

import (
	"bytes"
	"context"
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
	"github.com/otelfleet/otelfleet/pkg/logutil"
	services_int "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	logger   *slog.Logger
	opampSrv server.OpAMPServer

	agentStore storage.KeyValue[*protobufs.AgentToServer]
	services.Service

	mu       sync.RWMutex
	addrToId map[string]string
	idToConn map[string]types.Connection // agentID -> connection

	// connectionStateStore replaces the in-memory AgentTracker
	connectionStateStore storage.KeyValue[*v1alpha1.AgentConnectionState]

	healthStore           storage.KeyValue[*protobufs.ComponentHealth]
	configStore           storage.KeyValue[*protobufs.EffectiveConfig]
	assignedConfigStore   storage.KeyValue[*configv1alpha1.Config]
	remoteStatusStore     storage.KeyValue[*protobufs.RemoteConfigStatus]
	opampAgentDescription storage.KeyValue[*protobufs.AgentDescription]
}

var _ services_int.OpAmpServerHandler = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	agentStore storage.KeyValue[*protobufs.AgentToServer],
	connectionStateStore storage.KeyValue[*v1alpha1.AgentConnectionState],
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	configStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
	opampAgentDescriptionStore storage.KeyValue[*protobufs.AgentDescription],
	assignedConfigStore storage.KeyValue[*configv1alpha1.Config],
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:                l,
		opampSrv:              opampSvr,
		agentStore:            agentStore,
		addrToId:              map[string]string{},
		idToConn:              map[string]types.Connection{},
		connectionStateStore:  connectionStateStore,
		healthStore:           healthStore,
		configStore:           configStore,
		remoteStatusStore:     remoteStatusStore,
		opampAgentDescription: opampAgentDescriptionStore,
		assignedConfigStore:   assignedConfigStore,
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
		return nil, err
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

	// Update connection state and check for sequence gaps
	var needsFullState bool
	if agentID != "" {
		needsFullState = s.updateConnectionState(ctx, agentID, message)
	}

	resp := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}
	if agentID == "" {
		logger.Error("cannot persist agent data: no agent ID available")
		return resp
	}
	if message.RemoteConfigStatus != nil {
		if err := s.handleRemoteConfigStatus(ctx, conn, agentID, message.RemoteConfigStatus); err != nil {
			logger.With("err", err).Error("failed to handle remote config status message")
		}
	}

	if message.AgentDescription != nil {
		logger.Info("persisting agent description")
		if err := s.opampAgentDescription.Put(ctx, agentID, message.AgentDescription); err != nil {
			logger.With("err", err).Error("failed to persist opamp agent-description")
		}
	}
	if message.Health != nil {
		logger.Info("persisting agent health")
		if err := s.healthStore.Put(ctx, agentID, message.Health); err != nil {
			logger.With("err", err).Error("failed to persist health")
		}
	}

	if message.EffectiveConfig != nil {
		logger.Info("persisting effective config")
		if err := s.configStore.Put(ctx, agentID, message.EffectiveConfig); err != nil {
			logger.With("err", err).Error("failed to persist effective config")
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
	state, err := s.connectionStateStore.Get(ctx, agentID)
	needsFullState := false

	if grpcutil.IsErrorNotFound(err) {
		// First message from this agent - create new state
		state = &v1alpha1.AgentConnectionState{
			AgentId:     agentID,
			State:       v1alpha1.AgentState_AGENT_STATE_CONNECTED,
			ConnectedAt: timestamppb.Now(),
			InstanceUid: msg.InstanceUid,
		}
		// Request full state for first connection
		needsFullState = true
	} else if err != nil {
		s.logger.With("err", err).Error("failed to get the agent state")
		return true
	} else {
		// Check if this is a new instance (agent restarted)
		if !bytes.Equal(state.InstanceUid, msg.InstanceUid) {
			s.logger.With("agent_id", agentID).Info("agent instance changed, requesting full state")
			state.InstanceUid = msg.InstanceUid
			state.ConnectedAt = timestamppb.Now()
			state.SequenceNum = 0
			needsFullState = true
		} else if msg.SequenceNum > 0 {
			// Check for sequence gap (status compression support)
			expectedSeq := state.SequenceNum + 1
			if msg.SequenceNum != expectedSeq {
				needsFullState = true
			}
		}
	}

	// Always update LastSeen on every message
	state.LastSeen = timestamppb.Now()
	state.State = v1alpha1.AgentState_AGENT_STATE_CONNECTED

	// Update capabilities if provided
	if msg.Capabilities != 0 {
		state.Capabilities = msg.Capabilities
	}
	state.SequenceNum = msg.SequenceNum

	if err := s.connectionStateStore.Put(ctx, agentID, state); err != nil {
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
		if err := s.remoteStatusStore.Put(ctx, agentID, remoteConfigStatus); err != nil {
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
	if err := s.remoteStatusStore.Put(ctx, agentID, remoteConfigStatus); err != nil {
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
	state, err := s.connectionStateStore.Get(ctx, agentID)
	if err == nil {
		state.State = v1alpha1.AgentState_AGENT_STATE_DISCONNECTED
		state.DisconnectedAt = timestamppb.Now()
		if err := s.connectionStateStore.Put(ctx, agentID, state); err != nil {
			logger.With("err", err).Error("failed to persist disconnected state")
		}
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
