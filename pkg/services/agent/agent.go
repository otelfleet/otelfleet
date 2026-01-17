package agent

import (
	"context"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1/v1alpha1connect"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util/configsync"
	"github.com/samber/lo"
)

type AgentServer struct {
	logger     *slog.Logger
	agentStore storage.KeyValue[*v1alpha1.AgentDescription]

	connectionStateStore  storage.KeyValue[*v1alpha1.AgentConnectionState]
	configAssignmentStore storage.KeyValue[*configv1alpha1.ConfigAssignment]

	healthStore                storage.KeyValue[*protobufs.ComponentHealth]
	configStore                storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore          storage.KeyValue[*protobufs.RemoteConfigStatus]
	opampAgentDescriptionStore storage.KeyValue[*protobufs.AgentDescription]

	services.Service
}

var _ v1alpha1connect.AgentServiceHandler = (*AgentServer)(nil)

func NewAgentServer(
	logger *slog.Logger,
	agentStore storage.KeyValue[*v1alpha1.AgentDescription],
	connectionStateStore storage.KeyValue[*v1alpha1.AgentConnectionState],
	configAssignmentStore storage.KeyValue[*configv1alpha1.ConfigAssignment],
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	configStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
	opampAgentDescriptionStore storage.KeyValue[*protobufs.AgentDescription],
) *AgentServer {
	a := &AgentServer{
		logger:                     logger,
		agentStore:                 agentStore,
		connectionStateStore:       connectionStateStore,
		configAssignmentStore:      configAssignmentStore,
		healthStore:                healthStore,
		configStore:                configStore,
		remoteStatusStore:          remoteStatusStore,
		opampAgentDescriptionStore: opampAgentDescriptionStore,
	}
	a.Service = services.NewBasicService(nil, a.running, nil)
	return a
}

func (a *AgentServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (a *AgentServer) ConfigureHTTP(mux *mux.Router) {
	a.logger.Info("configuring routes")
	v1alpha1connect.RegisterAgentServiceHandler(mux, a)
}

func (a *AgentServer) ListAgents(
	ctx context.Context, req *connect.Request[v1alpha1.ListAgentsRequest],
) (*connect.Response[v1alpha1.ListAgentsResponse], error) {
	agents, err := a.agentStore.List(ctx)
	if err != nil {
		return nil, err
	}
	a.logger.With("numAgents", len(agents)).Debug("found agents")
	descAndStatus := lo.Map(agents, func(agent *v1alpha1.AgentDescription, _ int) *v1alpha1.AgentDescriptionAndStatus {
		return &v1alpha1.AgentDescriptionAndStatus{
			Agent: agent,
		}
	})
	if req.Msg.GetWithStatus() {
		for idx, entry := range descAndStatus {
			agentID := entry.Agent.GetId()
			desc, err := a.opampAgentDescriptionStore.Get(ctx, agentID)
			if err != nil {
				a.logger.With("err", err).Error("failed to get opamp remote agent description")
			}
			newEntry := &v1alpha1.AgentDescriptionAndStatus{
				Agent: entry.Agent,
			}
			// Get capabilities from connection state store
			capabilities := a.getCapabilities(ctx, agentID)
			enrichAgentDescription(newEntry.Agent, desc, capabilities)
			newEntry.Status = a.status(ctx, agentID)
			descAndStatus[idx] = newEntry
		}
	}
	resp := &v1alpha1.ListAgentsResponse{
		Agents: descAndStatus,
	}
	return connect.NewResponse(resp), nil
}

func (a *AgentServer) GetAgent(ctx context.Context, req *connect.Request[v1alpha1.GetAgentRequest]) (*connect.Response[v1alpha1.GetAgentResponse], error) {
	agentID := req.Msg.GetAgentId()
	agent, err := a.agentStore.Get(ctx, agentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	remoteDesc, err := a.opampAgentDescriptionStore.Get(ctx, agentID)
	if err != nil {
		a.logger.With("err", err).Error("failed to get opamp remote agent description")
	}
	// Get capabilities from connection state store
	capabilities := a.getCapabilities(ctx, agentID)
	enrichAgentDescription(agent, remoteDesc, capabilities)

	return connect.NewResponse(&v1alpha1.GetAgentResponse{Agent: agent}), nil
}

func (a *AgentServer) Status(ctx context.Context, req *connect.Request[v1alpha1.GetAgentStatusRequest]) (*connect.Response[v1alpha1.GetAgentStatusResponse], error) {
	agentID := req.Msg.GetAgentId()

	resp := a.status(ctx, agentID)

	return connect.NewResponse(&v1alpha1.GetAgentStatusResponse{
		Status: resp,
	}), nil
}

func (a *AgentServer) status(ctx context.Context, agentID string) *v1alpha1.AgentStatus {
	resp := &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AGENT_STATE_UNKNOWN,
	}

	// Get connection state from persistent store
	if connState, err := a.connectionStateStore.Get(ctx, agentID); err == nil {
		resp.State = connState.State
		resp.LastSeen = connState.LastSeen
		resp.ConnectedAt = connState.ConnectedAt
		resp.DisconnectedAt = connState.DisconnectedAt
	}

	if health, err := a.healthStore.Get(ctx, agentID); err == nil {
		resp.Health = convertHealth(health)
	}

	if config, err := a.configStore.Get(ctx, agentID); err == nil {
		resp.EffectiveConfig = convertEffectiveConfig(config)
	}

	if status, err := a.remoteStatusStore.Get(ctx, agentID); err == nil {
		resp.RemoteConfigStatus = convertRemoteConfigStatus(status)
	}

	// Compute config sync status using shared logic
	resp.ConfigSyncStatus, resp.ConfigSyncReason = a.computeConfigSyncStatus(ctx, agentID)

	return resp
}

// computeConfigSyncStatus computes the config sync status for an agent.
func (a *AgentServer) computeConfigSyncStatus(ctx context.Context, agentID string) (v1alpha1.ConfigSyncStatus, string) {
	assignment, err := a.configAssignmentStore.Get(ctx, agentID)
	if err != nil {
		// No assignment = unknown sync status
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_UNKNOWN, ""
	}

	return configsync.ComputeConfigSyncStatus(ctx, agentID, assignment.GetConfigHash(), a.remoteStatusStore)
}

// getCapabilities returns the capabilities for an agent from the connection state store.
func (a *AgentServer) getCapabilities(ctx context.Context, agentID string) uint64 {
	if state, err := a.connectionStateStore.Get(ctx, agentID); err == nil {
		return state.Capabilities
	}
	return 0
}

func convertHealth(h *protobufs.ComponentHealth) *v1alpha1.ComponentHealth {
	if h == nil {
		return nil
	}
	result := &v1alpha1.ComponentHealth{
		Healthy:            h.Healthy,
		StartTimeUnixNano:  h.StartTimeUnixNano,
		LastError:          h.LastError,
		Status:             h.Status,
		StatusTimeUnixNano: h.StatusTimeUnixNano,
	}
	if len(h.ComponentHealthMap) > 0 {
		result.ComponentHealthMap = make(map[string]*v1alpha1.ComponentHealth, len(h.ComponentHealthMap))
		for k, v := range h.ComponentHealthMap {
			result.ComponentHealthMap[k] = convertHealth(v)
		}
	}
	return result
}

func convertEffectiveConfig(c *protobufs.EffectiveConfig) *v1alpha1.EffectiveConfig {
	if c == nil || c.ConfigMap == nil {
		return nil
	}
	result := &v1alpha1.EffectiveConfig{
		ConfigMap: &v1alpha1.AgentConfigMap{},
	}
	if len(c.ConfigMap.ConfigMap) > 0 {
		result.ConfigMap.ConfigMap = make(map[string]*v1alpha1.AgentConfigFile, len(c.ConfigMap.ConfigMap))
		for k, v := range c.ConfigMap.ConfigMap {
			result.ConfigMap.ConfigMap[k] = &v1alpha1.AgentConfigFile{
				Body:        v.Body,
				ContentType: v.ContentType,
			}
		}
	}
	return result
}

func convertRemoteConfigStatus(s *protobufs.RemoteConfigStatus) *v1alpha1.RemoteConfigStatus {
	if s == nil {
		return nil
	}
	return &v1alpha1.RemoteConfigStatus{
		LastRemoteConfigHash: s.LastRemoteConfigHash,
		Status:               v1alpha1.RemoteConfigStatuses(s.Status),
		ErrorMessage:         s.ErrorMessage,
	}
}

// enrichAgentDescription populates the agent description with live data from the tracker.
func enrichAgentDescription(agent *v1alpha1.AgentDescription, desc *protobufs.AgentDescription, capabilities uint64) {
	if desc == nil {
		return
	}

	// Set capabilities from the tracked state by checking each bit in the bitmask
	var ret []string
	for value, name := range protobufs.AgentCapabilities_name {
		if value != 0 && capabilities&uint64(value) != 0 {
			ret = append(ret, strings.TrimPrefix(name, "AgentCapabilities_"))
		}
	}
	agent.Capabilities = ret
	agent.IdentifyingAttributes = convertKeyValues(desc.GetIdentifyingAttributes())
	agent.NonIdentifyingAttributes = convertKeyValues(desc.GetNonIdentifyingAttributes())
}

// convertKeyValues converts OpAMP KeyValue slice to v1alpha1 KeyValue slice.
func convertKeyValues(kvs []*protobufs.KeyValue) []*v1alpha1.KeyValue {
	if kvs == nil {
		return nil
	}
	result := make([]*v1alpha1.KeyValue, len(kvs))
	for i, kv := range kvs {
		result[i] = &v1alpha1.KeyValue{
			Key:   kv.Key,
			Value: convertAnyValue(kv.Value),
		}
	}
	return result
}

// convertAnyValue converts an OpAMP AnyValue to v1alpha1 AnyValue.
func convertAnyValue(v *protobufs.AnyValue) *v1alpha1.AnyValue {
	if v == nil {
		return nil
	}

	switch val := v.Value.(type) {
	case *protobufs.AnyValue_StringValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_StringValue{StringValue: val.StringValue}}
	case *protobufs.AnyValue_BoolValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BoolValue{BoolValue: val.BoolValue}}
	case *protobufs.AnyValue_IntValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_IntValue{IntValue: val.IntValue}}
	case *protobufs.AnyValue_DoubleValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_DoubleValue{DoubleValue: val.DoubleValue}}
	case *protobufs.AnyValue_BytesValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BytesValue{BytesValue: val.BytesValue}}
	case *protobufs.AnyValue_ArrayValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_ArrayValue{ArrayValue: convertArrayValue(val.ArrayValue)}}
	case *protobufs.AnyValue_KvlistValue:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_KvlistValue{KvlistValue: convertKeyValueList(val.KvlistValue)}}
	default:
		return nil
	}
}

// convertArrayValue converts an OpAMP ArrayValue to v1alpha1 ArrayValue.
func convertArrayValue(arr *protobufs.ArrayValue) *v1alpha1.ArrayValue {
	if arr == nil {
		return nil
	}
	result := &v1alpha1.ArrayValue{
		Values: make([]*v1alpha1.AnyValue, len(arr.Values)),
	}
	for i, v := range arr.Values {
		result.Values[i] = convertAnyValue(v)
	}
	return result
}

// convertKeyValueList converts an OpAMP KeyValueList to v1alpha1 KeyValueList.
func convertKeyValueList(kvl *protobufs.KeyValueList) *v1alpha1.KeyValueList {
	if kvl == nil {
		return nil
	}
	return &v1alpha1.KeyValueList{
		Values: convertKeyValues(kvl.Values),
	}
}
