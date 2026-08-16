package opamp

// import (
// 	"bytes"
// 	"context"
// 	"errors"
// 	"fmt"
// 	"log/slog"
// 	"net/url"
// 	"path"
// 	"time"

// 	"github.com/open-telemetry/opamp-go/protobufs"
// 	"github.com/open-telemetry/opamp-go/server/types"
// 	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
// 	"github.com/otelfleet/otelfleet/pkg/config"
// 	"github.com/otelfleet/otelfleet/pkg/deployment"
// 	"github.com/otelfleet/otelfleet/pkg/logutil"
// 	services_int "github.com/otelfleet/otelfleet/pkg/services"
// 	"github.com/otelfleet/otelfleet/pkg/services/opamp/sync"
// 	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
// 	"github.com/otelfleet/otelfleet/pkg/util"
// 	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
// 	"google.golang.org/grpc/codes"
// 	"google.golang.org/grpc/status"
// 	"google.golang.org/protobuf/types/known/timestamppb"
// )

// var _ services_int.OpAmpServerHandler = (*ServerAgentHandler)(nil)

// type ServerAgentHandler struct {
// 	ctx            context.Context
// 	logger         *slog.Logger
// 	otlpServerAddr string
// 	otlpConfig     *config.OTLPConfig

// 	// serverAssignedConnID is a temporary ID
// 	// assigned by the server to identify the agent
// 	// before we know how to track its instance_uid
// 	serverAssignedConnID string

// 	inst deployment.Instance

// 	// TODO : we should really segregate the bootstrap lifecycle from the agent handler
// 	mgr deployment.Manager

// 	// Config store for OpAMP-specific config logic
// 	// TODO : I don't think I want to do it this way.
// 	configFilterSync *sync.ConfigFilterSync

// 	// unset until we understand who is who
// 	agentID     *string
// 	instanceUID *string
// }

// func NewServerAgentHandler(
// 	ctx context.Context,
// 	serverConnID string,
// 	configFilterSync *sync.ConfigFilterSync,
// 	logger *slog.Logger,
// 	// FIXME: simple example to pass connection settings
// 	otlpServerAddr string,
// 	otlpConfig *config.OTLPConfig,
// 	mgr deployment.Manager,
// ) *ServerAgentHandler {
// 	return &ServerAgentHandler{
// 		ctx:                  ctx,
// 		serverAssignedConnID: serverConnID,
// 		logger:               logger.With("server-conn-id", serverConnID),
// 		instanceUID:          nil,
// 		configFilterSync:     configFilterSync,
// 		otlpServerAddr:       otlpServerAddr,
// 		otlpConfig:           otlpConfig,
// 		mgr:                  mgr,
// 	}
// }

// // The following callbacks will never be called concurrently for the same
// // connection. They may be called concurrently for different connections.

// // OnConnected is called when an incoming OpAMP connection is successfully
// // established after OnConnecting() returns.
// func (s *ServerAgentHandler) OnConnected(ctx context.Context, conn types.Connection) {
// 	s.logger.With("addr", conn.Connection().RemoteAddr().String()).Info("agent connected")
// }

// // OnMessage is called when a message is received from the connection. Can happen
// // only after OnConnected().
// // When the returned ServerToAgent message is nil, WebSocket will not send a
// // message to the Agent, and the HTTP request will respond to an empty message.
// // If the return is not nil it will be sent as a response to the Agent.
// // For plain HTTP requests once OnMessage returns and the response is sent
// // to the Agent the OnConnectionClose message will be called immediately.
// func (s *ServerAgentHandler) OnMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
// 	instanceUID := fmt.Sprintf("%x", message.InstanceUid)
// 	logger := s.logger.With("instance-uid", instanceUID)
// 	logger.With("sequenceNum", message.SequenceNum).Debug("received message from agent")
// 	ctx = logutil.WithContext(ctx, logger)

// 	// bootstrap
// 	if s.instanceUID == nil {
// 		if msg := s.bootstrap(ctx, logger, instanceUID, message); msg != nil {
// 			return msg
// 		}
// 	}
// 	resp := &protobufs.ServerToAgent{
// 		InstanceUid: message.InstanceUid,
// 	}
// 	// Update connection state and check for sequence gaps
// 	needsFullState := s.updateConnectionState(ctx, message)

// 	if err := s.persistAgentInformation(ctx, message); err != nil {
// 		return ErrorResponse(message.InstanceUid, NewUnavailableError(err.Error()))
// 	}

// 	// TODO : set these conditionally
// 	connSettings, err := s.buildTelemetryOptions(ctx, conn, message)
// 	if err != nil {
// 		return ErrorResponse(message.InstanceUid, NewUnavailableError(err.Error()))
// 	}
// 	resp.ConnectionSettings = connSettings

// 	if remoteConfig := s.pendingRemoteConfig(ctx, *s.agentID, message); remoteConfig != nil {
// 		resp.RemoteConfig = remoteConfig
// 	}

// 	if needsFullState {
// 		resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
// 		logger.Info("requesting full state report due to sequence gap")
// 	}
// 	return resp
// }

// func (s *ServerAgentHandler) persistAgentInformation(ctx context.Context, msg *protobufs.AgentToServer) error {
// 	logger := logutil.FromContext(ctx)
// 	errs := []error{}
// 	if msg.RemoteConfigStatus != nil {
// 		if err := s.handleRemoteConfigStatus(ctx, msg.RemoteConfigStatus); err != nil {
// 			logger.With("err", err).Error("failed to handle remote config status message")
// 			// ignore status update from returned errors
// 		}
// 	}

// 	if msg.AgentDescription != nil {
// 		logger.Info("persisting agent description")
// 		if err := s.inst.SetDescription(ctx, msg.AgentDescription); err != nil {
// 			logger.With("err", err).Error("failed to persist opamp agent-description")
// 			errs = append(errs, err)
// 		}
// 	}
// 	if msg.Health != nil {
// 		logger.Info("persisting agent health")
// 		if err := s.inst.SetHealth(ctx, msg.Health); err != nil {
// 			logger.With("err", err).Error("failed to persist health")
// 			errs = append(errs, err)
// 		}
// 	}

// 	if msg.EffectiveConfig != nil {
// 		logger.Info("persisting effective config")
// 		if err := s.inst.SetEffectiveConfig(ctx, msg.EffectiveConfig); err != nil {
// 			logger.With("err", err).Error("failed to persist effective config")
// 			errs = append(errs, err)
// 		}
// 	}

// 	return errors.Join(errs...)
// }

// func (s *ServerAgentHandler) buildTelemetryOptions(ctx context.Context, _ types.Connection, message *protobufs.AgentToServer) (*protobufs.ConnectionSettingsOffers, error) {
// 	logger := logutil.FromContext(ctx)
// 	resp := &protobufs.ConnectionSettingsOffers{}
// 	if message.Capabilities&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnLogs) != 0 {
// 		logsAddr := &url.URL{
// 			Scheme: "http",
// 			Host:   s.otlpServerAddr,
// 			Path:   path.Join(s.otlpConfig.BasePath, s.otlpConfig.LogsAPIPath),
// 		}
// 		logger.With("supplied-addr", logsAddr.String()).Debug("agent supports reporting own logs")
// 		resp.OwnLogs = &protobufs.TelemetryConnectionSettings{
// 			DestinationEndpoint: logsAddr.String(),
// 		}
// 	}
// 	if message.Capabilities&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics) != 0 {
// 		metricsAddr := &url.URL{
// 			Scheme: "http",
// 			Host:   s.otlpServerAddr,
// 			Path:   path.Join(s.otlpConfig.BasePath, s.otlpConfig.MetricsAPIPath),
// 		}
// 		logger.With("supplied-addr", metricsAddr.String()).Debug("agent supports reporting own metrics")
// 		resp.OwnMetrics = &protobufs.TelemetryConnectionSettings{
// 			DestinationEndpoint: metricsAddr.String(),
// 		}
// 	}
// 	if message.Capabilities&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnTraces) != 0 {
// 		traceAddr := &url.URL{
// 			Scheme: "http",
// 			Host:   s.otlpServerAddr,
// 			Path:   path.Join(s.otlpConfig.BasePath, s.otlpConfig.TraceAPIPath),
// 		}
// 		logger.With("supplied-addr", traceAddr.String()).Debug("agent supports reporting own tracess")
// 		resp.OwnTraces = &protobufs.TelemetryConnectionSettings{
// 			DestinationEndpoint: traceAddr.String(),
// 		}
// 	}

// 	hash, err := util.ProtoHash(resp)
// 	if err != nil {
// 		return nil, err
// 	}
// 	resp.Hash = hash
// 	return resp, nil
// }

// func (s *ServerAgentHandler) bootstrap(
// 	ctx context.Context,
// 	logger *slog.Logger,
// 	instanceUID string,
// 	message *protobufs.AgentToServer,
// ) *protobufs.ServerToAgent {
// 	flags := message.GetFlags()
// 	if flags&uint64(protobufs.AgentToServerFlags_AgentToServerFlags_RequestInstanceUid) == 1 {
// 		// server assigns new instance UID
// 		instanceUID = util.NewUUID()
// 		// TODO : without custom methods - can we uniquely assign an ID to a remote agent?
// 		logger.With("temp-id", fmt.Sprintf("%x", message.InstanceUid), "assigned-id", instanceUID).Info(
// 			"agent requested an instanceUID from server",
// 		)
// 		// TODO : do we need to keep track of the temporary ID? idk
// 	}
// 	ok, err := s.mgr.Exists(ctx, instanceUID)
// 	if err != nil {
// 		logger.With("err", err).Error("failed to verify agent is registered")
// 		return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to verify agent is registered"))
// 	}
// 	if !ok {
// 		logger.Info("registering agent")
// 		// TODO : generate new friendly name or infer from original http request headers
// 		// Or : from custom auth flows
// 		agentID := instanceUID // TODO : same as above
// 		if err := s.mgr.Register(ctx, agentID, "foo"); err != nil {
// 			// handle potential error codes here
// 			return ErrorResponse(message.InstanceUid, NewUnavailableError("failed to register agent"))
// 		}
// 		now := time.Now()
// 		s.agentID = &agentID
// 		s.instanceUID = &instanceUID
// 		s.inst = s.mgr.Instance(agentID)
// 		if err := s.inst.SetConnectionState(ctx, &v1alpha1.ConnectionStatus{
// 			State:          v1alpha1.AgentState_AGENT_STATE_CONNECTED,
// 			LastSeen:       timestamppb.New(now),
// 			ConnectedAt:    timestamppb.New(now),
// 			DisconnectedAt: nil,
// 			// TODO
// 		}); err != nil {
// 			logger.With("err", err).Error("failed to update connection state")
// 		}
// 	}
// 	// TODO : inconsistent UID handling with above. need to iron that out
// 	s.agentID = &instanceUID
// 	s.instanceUID = &instanceUID
// 	return nil
// }

// // OnConnectionClose is called when the OpAMP connection is closed.
// func (s *ServerAgentHandler) OnConnectionClose(conn types.Connection) {
// 	remoteAddr := conn.Connection().RemoteAddr().String()
// 	logger := s.logger.With("remote_addr", remoteAddr)
// 	logger.Info("agent disconnected")

// 	if s.agentID == nil {
// 		return
// 	}

// 	// Persist disconnected state
// 	ctx := context.Background()
// 	existingState, err := s.inst.GetConnectionState(ctx)
// 	if err != nil {
// 		if grpcutil.IsErrorNotFound(err) {
// 			// Agent never had state stored - this shouldn't happen but is not critical
// 			logger.Warn("no connection state found for disconnected agent")
// 		} else {
// 			// Actual storage error - log at error level
// 			logger.With("err", err).Error("failed to get connection state for disconnected agent")
// 		}
// 		return
// 	}
// 	now := time.Now()
// 	existingState.State = v1alpha1.AgentState_AGENT_STATE_DISCONNECTED
// 	existingState.DisconnectedAt = timestamppb.New(now)
// 	if err := s.inst.SetConnectionState(ctx, existingState); err != nil {
// 		logger.With("err", err).Error("failed to persist disconnected state")
// 	}
// }

// // OnReadMessageError is called when an error occurs while reading or deserializing a message.
// func (s *ServerAgentHandler) OnReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
// 	s.logger.
// 		With("remote-addr", conn.Connection().RemoteAddr().String()).
// 		With("msg", string(msgByte)).
// 		With("err", err).
// 		Error("failed to read / deserialize agent message")
// }

// // OnMessageResponseError is called when an error occurs while sending the response message from the OnMessage loop.
// func (s *ServerAgentHandler) OnMessageResponseError(conn types.Connection, message *protobufs.ServerToAgent, err error) {
// 	s.logger.
// 		With("remote-addr", conn.Connection().RemoteAddr().String()).
// 		With("msg", string(message.String())).
// 		With("err", err).
// 		Error("failed to send server message to agent")
// }

// // updateConnectionState updates the persisted connection state for an agent.
// // Returns true if a full state report is needed (sequence gap or instance change detected).
// func (s *ServerAgentHandler) updateConnectionState(ctx context.Context, msg *protobufs.AgentToServer) bool {
// 	// Try to get existing state from repository
// 	existingState, err := s.inst.GetConnectionState(ctx)
// 	needsFullState := false

// 	now := time.Now()

// 	if err != nil && status.Code(err) == codes.NotFound {
// 		s.inst.SetConnectionState(
// 			ctx,
// 			&v1alpha1.ConnectionStatus{
// 				State:       v1alpha1.AgentState_AGENT_STATE_CONNECTED,
// 				LastSeen:    timestamppb.New(now),
// 				ConnectedAt: timestamppb.New(now),
// 				Sequence:    msg.SequenceNum,
// 			},
// 		)
// 	} else if err != nil {
// 		// Actual storage error - log and request full state to be safe
// 		s.logger.With("err", err).Error("failed to get connection state")
// 		return true
// 	}

// 	// Check for sequence gap (status compression support)
// 	if msg.SequenceNum > 0 && existingState.Sequence+1 != msg.SequenceNum {
// 		needsFullState = true
// 	}

// 	// Always update LastSeen on every message
// 	existingState.LastSeen = timestamppb.New(now)
// 	existingState.State = v1alpha1.AgentState_AGENT_STATE_CONNECTED
// 	existingState.Sequence = msg.SequenceNum

// 	if err := s.inst.SetConnectionState(ctx, existingState); err != nil {
// 		s.logger.With("err", err, "agent_id", *s.agentID).Error("failed to persist connection state")
// 	}

// 	return needsFullState
// }

// func (s *ServerAgentHandler) handleRemoteConfigStatus(
// 	ctx context.Context,
// 	remoteConfigStatus *protobufs.RemoteConfigStatus,
// ) error {
// 	if err := s.inst.SetRemoteStatus(ctx, remoteConfigStatus); err != nil {
// 		return fmt.Errorf("failed to persist remote config status: %w", err)
// 	}
// 	return nil
// }

// // pendingRemoteConfig returns the config to push when the agent is not already
// // running the config the server currently resolves for it.
// func (s *ServerAgentHandler) pendingRemoteConfig(ctx context.Context, agentID string, msg *protobufs.AgentToServer) *protobufs.AgentRemoteConfig {
// 	logger := logutil.FromContext(ctx)
// 	if !deployment.Capabilities(msg.GetCapabilities()).HasAcceptsRemoteConfig() {
// 		return nil
// 	}

// 	resolved, err := s.constructConfig(ctx, agentID)
// 	if err != nil {
// 		logger.With("err", err).Error("failed to construct config")
// 		return nil
// 	}
// 	expectedHash := s.calculateHash(resolved.configMap)
// 	s.recordAssignment(ctx, agentID, resolved, expectedHash)

// 	if bytes.Equal(expectedHash, s.appliedConfigHash(ctx, agentID, msg)) {
// 		logger.Debug("agent remote config up-to-date")
// 		return nil
// 	}

// 	logger.Info("pushing remote config to agent", "expected_hash", fmt.Sprintf("%x", expectedHash))
// 	return &protobufs.AgentRemoteConfig{
// 		Config:     resolved.configMap,
// 		ConfigHash: expectedHash,
// 	}
// }

// func (s *ServerAgentHandler) appliedConfigHash(ctx context.Context, agentID string, msg *protobufs.AgentToServer) []byte {
// 	if msg.GetRemoteConfigStatus() != nil {
// 		return msg.GetRemoteConfigStatus().GetLastRemoteConfigHash()
// 	}

// 	lastKnownStatus, err := s.inst.GetRemoteStatus(ctx)
// 	if err != nil {
// 		logutil.FromContext(ctx).With("err", err).Error("failed to load last reported remote config status")
// 		return nil
// 	}
// 	if lastKnownStatus.LastRemoteConfigHash == nil {
// 		return nil
// 	}
// 	return lastKnownStatus.LastRemoteConfigHash
// }

// func (s *ServerAgentHandler) calculateHash(agentToConfigMap *protobufs.AgentConfigMap) []byte {
// 	return util.HashAgentConfigMap(agentToConfigMap)
// }

// func (s *ServerAgentHandler) agentLabels(ctx context.Context, agentID string) (sync.AgentLabels, error) {
// 	agentDescription, err := s.inst.GetDescription(ctx)
// 	if err != nil {
// 		return sync.AgentLabels{}, err
// 	}
// 	_ = agentDescription
// 	return sync.AgentLabels{
// 		// TODO : hook these up
// 		// Identifying:    toStringLabels(agentDescription.IdentifyingAttributes),
// 		// NonIdentifying: toStringLabels(agentDescription.NonIdentifyingAttributes),
// 	}, nil
// }

// func toStringLabels(attrs map[string]any) map[string]string {
// 	labels := make(map[string]string, len(attrs))
// 	for k, v := range attrs {
// 		labels[k] = fmt.Sprint(v)
// 	}
// 	return labels
// }

// type resolvedConfig struct {
// 	configMap *protobufs.AgentConfigMap
// 	configRef string
// }

// func (s *ServerAgentHandler) filteredConfig(ctx context.Context, agentID string) *resolvedConfig {
// 	panic("implement me")
// 	// logger := logutil.FromContext(ctx)
// 	// labels, err := s.agentLabels(ctx, agentID)
// 	// if err != nil {
// 	// 	logger.With("err", err).Error("failed to load agent labels for config filtering")
// 	// 	return nil
// 	// }

// 	// filter := s.configFilterSync.Match(labels)
// 	// configRef := filter.GetCollectorConfig().GetConfigRef()
// 	// if configRef == "" {
// 	// 	return nil
// 	// }
// 	// logger = logger.With("config_ref", configRef)

// 	// collectorConfig, err := s.collectorConfigs.Get(ctx, configRef)
// 	// if err != nil {
// 	// 	logger.With("err", err).Error("config filter references an unreadable collector config")
// 	// 	return nil
// 	// }

// 	// raw := collectorConfig.GetRaw()
// 	// if raw == nil {
// 	// 	// TODO : render CollectorConfig.components into yaml
// 	// 	logger.Error("collector config has no raw configuration")
// 	// 	return nil
// 	// }

// 	// logger.Info("agent config selected by config filter")
// 	// return &resolvedConfig{
// 	// 	configRef: configRef,
// 	// 	source:    configv1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL,
// 	// 	configMap: &protobufs.AgentConfigMap{
// 	// 		ConfigMap: map[string]*protobufs.AgentConfigFile{
// 	// 			"config.yaml": {
// 	// 				ContentType: collectorConfig.GetContentType(),
// 	// 				Body:        raw,
// 	// 			},
// 	// 		},
// 	// 	},
// 	// }
// }

// func (s *ServerAgentHandler) constructConfig(ctx context.Context, agentID string) (*resolvedConfig, error) {
// 	logger := logutil.FromContext(ctx)
// 	if filtered := s.filteredConfig(ctx, agentID); filtered != nil {
// 		return filtered, nil
// 	}

// 	assignedConfig, err := s.assignedConfigStore.Get(ctx, agentID)
// 	if grpcutil.IsErrorNotFound(err) {
// 		logger.Info("no assigned config, falling back to default config")
// 		return &resolvedConfig{
// 			source: configv1alpha1.ConfigSource_CONFIG_SOURCE_DEFAULT,
// 			configMap: &protobufs.AgentConfigMap{
// 				ConfigMap: map[string]*protobufs.AgentConfigFile{
// 					"config.yaml": {
// 						ContentType: "text/yaml",
// 						Body:        []byte(otelconfig.DefaultOtelConfig),
// 					},
// 				},
// 			},
// 		}, nil
// 	} else if err != nil {
// 		return nil, fmt.Errorf("failed to get assigned config: %w", err)
// 	}
// 	logger.Info("agent has an assigned config")
// 	return &resolvedConfig{
// 		configRef: agentID,
// 		source:    configv1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL,
// 		configMap: util.ProtoConfigToAgentConfigMap(assignedConfig),
// 	}, nil
// }

// // recordAssignment persists what the server resolved for the agent so config
// // sync status can be computed against it.
// func (s *ServerAgentHandler) recordAssignment(ctx context.Context, agentID string, resolved *resolvedConfig, hash []byte) {
// 	panic("implement me")
// 	// logger := logutil.FromContext(ctx)
// 	// existing, err := s.configAssignmentStore.Get(ctx, agentID)
// 	// if err != nil && !grpcutil.IsErrorNotFound(err) {
// 	// 	logger.With("err", err).Error("failed to read config assignment")
// 	// 	return
// 	// }
// 	// if existing != nil && bytes.Equal(existing.GetConfigHash(), hash) {
// 	// 	return
// 	// }

// 	// assignment := &configv1alpha1.ConfigAssignment{
// 	// 	AgentId:    agentID,
// 	// 	ConfigId:   resolved.configRef,
// 	// 	Source:     resolved.source,
// 	// 	AssignedAt: timestamppb.Now(),
// 	// 	ConfigHash: hash,
// 	// }
// 	// if err := s.configAssignmentStore.Put(ctx, agentID, assignment); err != nil {
// 	// 	logger.With("err", err).Error("failed to persist config assignment")
// 	// }
// }

// func (s *ServerAgentHandler) sendConfig(ctx context.Context, conn types.Connection, agentID string) error {
// 	s.logger.Log(ctx, logutil.LevelTrace, "sending config to agent")
// 	resolved, err := s.constructConfig(ctx, agentID)
// 	if err != nil {
// 		return fmt.Errorf("failed to construct config : %w", err)
// 	}
// 	hash := s.calculateHash(resolved.configMap)
// 	s.recordAssignment(ctx, agentID, resolved, hash)

// 	return conn.Send(ctx, &protobufs.ServerToAgent{
// 		RemoteConfig: &protobufs.AgentRemoteConfig{
// 			Config:     resolved.configMap,
// 			ConfigHash: hash,
// 		},
// 	})
// }
