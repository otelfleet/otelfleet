package opamp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server/types"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	services_int "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	stypes "github.com/otelfleet/otelfleet/pkg/storage/types"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
)

var _ services_int.OpAmpServerHandler = (*ServerAgentHandler)(nil)

type ServerAgentHandler struct {
	ctx context.Context
	// serverAssignedConnID is a temporary ID
	// assigned by the server to identify the agent
	// before we know how to track its instance_uid
	serverAssignedConnID string

	logger *slog.Logger

	// Repository for agent data access
	agentRepo agentdomain.Repository

	// Config store for OpAMP-specific config logic
	assignedConfigStore stypes.KeyValue[*configv1alpha1.Config]

	// unset until we understand who is who
	agentID     *string
	instanceUID *string
}

func NewServerAgentHandler(
	ctx context.Context,
	serverConnID string,
	agentRepo agentdomain.Repository,
	assignedConfigStore stypes.KeyValue[*configv1alpha1.Config],
	logger *slog.Logger,
) *ServerAgentHandler {
	return &ServerAgentHandler{
		ctx:                  ctx,
		serverAssignedConnID: serverConnID,
		logger:               logger.With("server-conn-id", serverConnID),
		agentRepo:            agentRepo,
		instanceUID:          nil,
		assignedConfigStore:  assignedConfigStore,
	}
}

// The following callbacks will never be called concurrently for the same
// connection. They may be called concurrently for different connections.

// OnConnected is called when an incoming OpAMP connection is successfully
// established after OnConnecting() returns.
func (s *ServerAgentHandler) OnConnected(ctx context.Context, conn types.Connection) {
	s.logger.With("addr", conn.Connection().RemoteAddr().String()).Info("agent connected")
}

// OnMessage is called when a message is received from the connection. Can happen
// only after OnConnected().
// When the returned ServerToAgent message is nil, WebSocket will not send a
// message to the Agent, and the HTTP request will respond to an empty message.
// If the return is not nil it will be sent as a response to the Agent.
// For plain HTTP requests once OnMessage returns and the response is sent
// to the Agent the OnConnectionClose message will be called immediately.
func (s *ServerAgentHandler) OnMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {

	instanceUID := fmt.Sprintf("%x", message.InstanceUid)
	logger := s.logger.With("instance-uid", instanceUID)
	logger.With("sequenceNum", message.SequenceNum).Debug("received message from agent")

	// bootstrap
	if s.instanceUID == nil {
		if msg := s.bootstrap(ctx, logger, instanceUID, message); msg != nil {
			return msg
		}
	}
	resp := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}
	// Update connection state and check for sequence gaps
	needsFullState := s.updateConnectionState(ctx, *s.agentID, message)
	if message.RemoteConfigStatus != nil {
		if err := s.handleRemoteConfigStatus(ctx, conn, *s.agentID, message.RemoteConfigStatus); err != nil {
			logger.With("err", err).Error("failed to handle remote config status message")
		}
	}

	if message.AgentDescription != nil {
		logger.Info("persisting agent description")
		if err := s.agentRepo.UpdateAttributes(ctx, *s.agentID, message.AgentDescription); err != nil {
			logger.With("err", err).Error("failed to persist opamp agent-description")
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to persist agent description"))
		}
	}
	if message.Health != nil {
		logger.Info("persisting agent health")
		if err := s.agentRepo.UpdateHealth(ctx, *s.agentID, message.Health); err != nil {
			logger.With("err", err).Error("failed to persist health")
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to persist agent health"))
		}
	}

	if message.EffectiveConfig != nil {
		logger.Info("persisting effective config")
		if err := s.agentRepo.UpdateEffectiveConfig(ctx, *s.agentID, message.EffectiveConfig); err != nil {
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

func (s *ServerAgentHandler) bootstrap(
	ctx context.Context,
	logger *slog.Logger,
	instanceUID string,
	message *protobufs.AgentToServer,
) *protobufs.ServerToAgent {
	flags := message.GetFlags()
	if flags&uint64(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid) == 1 {
		// server assigns new instance UID
		instanceUID = util.NewUUID()
		// TODO : without custom methods - can we uniquely assign an ID to a remote agent?
		logger.With("temp-id", fmt.Sprintf("%x", message.InstanceUid), "assigned-id", instanceUID).Info(
			"agent requested an instanceUID from server",
		)
		// TODO : do we need to keep track of the temporary ID? idk
	}
	ok, err := s.agentRepo.Exists(ctx, instanceUID)
	if err != nil {
		logger.With("err", err).Error("failed to verify agent is registered")
		return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to verify agent is registered"))
	}
	if !ok {
		logger.Info("registering agent")
		// TODO : generate new friendly name or infer from original http request headers
		// Or : from custom auth flows
		agentID := instanceUID // TODO : same as above
		if err := s.agentRepo.Register(ctx, agentID, "foo"); err != nil {
			// handle potential error codes here
			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to register agent"))
		}
		now := time.Now()
		s.agentID = &agentID
		s.instanceUID = &instanceUID
		if err := s.agentRepo.UpdateConnectionState(ctx, agentID, agentdomain.ConnectionState{
			State:          agentdomain.StateConnected,
			LastSeen:       &now,
			ConnectedAt:    &now,
			DisconnectedAt: nil,
			// Store the raw wire instance UID so updateConnectionState (which
			// compares against message.InstanceUid) does not see a spurious
			// instance change on the very next call.
			InstanceUID: message.InstanceUid,
			SequenceNum: message.SequenceNum,
			// TODO
			// Capabilities:   agentdomain.Capabilities(),
		}); err != nil {
			logger.With("err", err).Error("failed to update connection state")
		}
	}
	// TODO : inconsistent UID handling with above. need to iron that out
	s.agentID = &instanceUID
	s.instanceUID = &instanceUID
	return nil
}

// OnConnectionClose is called when the OpAMP connection is closed.
func (s *ServerAgentHandler) OnConnectionClose(conn types.Connection) {
	remoteAddr := conn.Connection().RemoteAddr().String()
	logger := s.logger.With("remote_addr", remoteAddr)
	logger.Info("agent disconnected")

	if s.agentID == nil {
		return
	}

	// Persist disconnected state
	ctx := context.Background()
	existingState, err := s.agentRepo.GetConnectionState(ctx, *s.agentID)
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
	if err := s.agentRepo.UpdateConnectionState(ctx, *s.agentID, *existingState); err != nil {
		logger.With("err", err).Error("failed to persist disconnected state")
	}
}

// OnReadMessageError is called when an error occurs while reading or deserializing a message.
func (s *ServerAgentHandler) OnReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
	s.logger.
		With("remote-addr", conn.Connection().RemoteAddr().String()).
		With("msg", string(msgByte)).
		With("err", err).
		Error("failed to read / deserialize agent message")
}

// OnMessageResponseError is called when an error occurs while sending the response message from the OnMessage loop.
func (s *ServerAgentHandler) OnMessageResponseError(conn types.Connection, message *protobufs.ServerToAgent, err error) {
	s.logger.
		With("remote-addr", conn.Connection().RemoteAddr().String()).
		With("msg", string(message.String())).
		With("err", err).
		Error("failed to send server message to agent")
}

// updateConnectionState updates the persisted connection state for an agent.
// Returns true if a full state report is needed (sequence gap or instance change detected).
func (s *ServerAgentHandler) updateConnectionState(ctx context.Context, agentID string, msg *protobufs.AgentToServer) bool {
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

func (s *ServerAgentHandler) handleRemoteConfigStatus(
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

func (s *ServerAgentHandler) calculateHash(agentToConfigMap *protobufs.AgentConfigMap) []byte {
	return util.HashAgentConfigMap(agentToConfigMap)
}

func (s *ServerAgentHandler) constructConfig(ctx context.Context, agentID string) (*protobufs.AgentConfigMap, error) {
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

func (s *ServerAgentHandler) sendConfig(ctx context.Context, conn types.Connection, agentID string) error {
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
