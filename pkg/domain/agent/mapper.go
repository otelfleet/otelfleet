package agent

import (
	"strings"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToAPIAgentRegistration converts domain Agent to API AgentRegistration.
func ToAPIAgentRegistration(agent *Agent) *v1alpha1.AgentRegistration {
	reg := &v1alpha1.AgentRegistration{
		Id:           agent.ID,
		FriendlyName: agent.FriendlyName,
	}

	if len(agent.Attributes.Identifying) > 0 {
		reg.IdentifyingAttributes = toAPIKeyValues(agent.Attributes.Identifying)
	}

	if len(agent.Attributes.NonIdentifying) > 0 {
		reg.NonIdentifyingAttributes = toAPIKeyValues(agent.Attributes.NonIdentifying)
	}

	reg.Capabilities = agent.Connection.Capabilities.ToStringSlice()

	return reg
}

// ToAPIStatus converts domain AgentRuntimeStatus to API AgentStatus.
func ToAPIStatus(agent *Agent) *v1alpha1.AgentStatus {
	status := &v1alpha1.AgentStatus{
		State:            convertToAPIState(agent.Connection.State),
		LastSeen:         timeToTimestamp(agent.Connection.LastSeen),
		ConnectedAt:      timeToTimestamp(agent.Connection.ConnectedAt),
		DisconnectedAt:   timeToTimestamp(agent.Connection.DisconnectedAt),
		ConfigSyncStatus: convertToAPIConfigSync(agent.Status.ConfigSyncStatus),
		ConfigSyncReason: agent.Status.ConfigSyncReason,
	}

	if agent.Status.Health != nil {
		status.Health = convertToAPIHealth(agent.Status.Health)
	}

	if agent.Status.EffectiveConfig != nil {
		status.EffectiveConfig = convertToAPIEffectiveConfig(agent.Status.EffectiveConfig)
	}

	if agent.Status.RemoteConfigStatus != nil {
		status.RemoteConfigStatus = convertToAPIRemoteStatus(agent.Status.RemoteConfigStatus)
	}

	return status
}

// ToAPIAgentView combines agent and status for list/get responses.
func ToAPIAgentView(agent *Agent) *v1alpha1.AgentView {
	return &v1alpha1.AgentView{
		Registration: ToAPIAgentRegistration(agent),
		Status:       ToAPIStatus(agent),
	}
}

// Capabilities helper methods

// HasAcceptsRemoteConfig checks if the agent has the AcceptsRemoteConfig capability.
func (c Capabilities) HasAcceptsRemoteConfig() bool {
	return c.Has(protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig)
}

// Has checks if a specific capability is set.
func (c Capabilities) Has(cap protobufs.AgentCapabilities) bool {
	return c&Capabilities(cap) != 0
}

// ToStringSlice converts capabilities bitmask to human-readable strings.
func (c Capabilities) ToStringSlice() []string {
	var ret []string
	for value, name := range protobufs.AgentCapabilities_name {
		if value != 0 && c&Capabilities(value) != 0 {
			ret = append(ret, strings.TrimPrefix(name, "AgentCapabilities_"))
		}
	}
	return ret
}

// Conversion from OpAMP protobuf types to domain types

// ConvertAttributes converts OpAMP AgentDescription to domain AgentAttributes.
func ConvertAttributes(desc *protobufs.AgentDescription) AgentAttributes {
	return AgentAttributes{
		Identifying:    convertKeyValuesToMap(desc.GetIdentifyingAttributes()),
		NonIdentifying: convertKeyValuesToMap(desc.GetNonIdentifyingAttributes()),
	}
}

// ConvertConnectionState converts v1alpha1 AgentConnectionState to domain ConnectionState.
func ConvertConnectionState(state *v1alpha1.AgentConnectionState) ConnectionState {
	return ConnectionState{
		State:          convertFromAPIState(state.GetState()),
		LastSeen:       timestampToTime(state.GetLastSeen()),
		ConnectedAt:    timestampToTime(state.GetConnectedAt()),
		DisconnectedAt: timestampToTime(state.GetDisconnectedAt()),
		InstanceUID:    state.GetInstanceUid(),
		Capabilities:   Capabilities(state.GetCapabilities()),
		SequenceNum:    state.GetSequenceNum(),
	}
}

// ConnectionStateToProto converts domain ConnectionState to v1alpha1 AgentConnectionState.
func ConnectionStateToProto(agentID string, state ConnectionState) *v1alpha1.AgentConnectionState {
	return &v1alpha1.AgentConnectionState{
		AgentId:        agentID,
		State:          convertToAPIState(state.State),
		LastSeen:       timeToTimestamp(state.LastSeen),
		ConnectedAt:    timeToTimestamp(state.ConnectedAt),
		DisconnectedAt: timeToTimestamp(state.DisconnectedAt),
		InstanceUid:    state.InstanceUID,
		Capabilities:   uint64(state.Capabilities),
		SequenceNum:    state.SequenceNum,
	}
}

// ConvertHealth converts OpAMP ComponentHealth to domain ComponentHealth.
func ConvertHealth(h *protobufs.ComponentHealth) *ComponentHealth {
	if h == nil {
		return nil
	}
	result := &ComponentHealth{
		Healthy:            h.GetHealthy(),
		StartTimeUnixNano:  h.GetStartTimeUnixNano(),
		LastError:          h.GetLastError(),
		Status:             h.GetStatus(),
		StatusTimeUnixNano: h.GetStatusTimeUnixNano(),
	}
	if len(h.GetComponentHealthMap()) > 0 {
		result.ComponentHealthMap = make(map[string]*ComponentHealth, len(h.GetComponentHealthMap()))
		for k, v := range h.GetComponentHealthMap() {
			result.ComponentHealthMap[k] = ConvertHealth(v)
		}
	}
	return result
}

// ConvertEffectiveConfig converts OpAMP EffectiveConfig to domain EffectiveConfig.
func ConvertEffectiveConfig(c *protobufs.EffectiveConfig) *EffectiveConfig {
	if c == nil || c.GetConfigMap() == nil {
		return nil
	}
	result := &EffectiveConfig{
		ConfigMap: make(map[string]*ConfigFile, len(c.GetConfigMap().GetConfigMap())),
	}
	for k, v := range c.GetConfigMap().GetConfigMap() {
		result.ConfigMap[k] = &ConfigFile{
			Body:        v.GetBody(),
			ContentType: v.GetContentType(),
		}
	}
	return result
}

// ConvertRemoteConfigStatus converts OpAMP RemoteConfigStatus to domain RemoteConfigStatus.
func ConvertRemoteConfigStatus(s *protobufs.RemoteConfigStatus) *RemoteConfigStatus {
	if s == nil {
		return nil
	}
	return &RemoteConfigStatus{
		LastRemoteConfigHash: s.GetLastRemoteConfigHash(),
		Status:               convertRemoteConfigStatuses(s.GetStatus()),
		ErrorMessage:         s.GetErrorMessage(),
	}
}

// ConvertConfigSyncStatus converts v1alpha1 ConfigSyncStatus to domain ConfigSyncStatus.
func ConvertConfigSyncStatus(status v1alpha1.ConfigSyncStatus) ConfigSyncStatus {
	switch status {
	case v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_IN_SYNC:
		return ConfigSyncInSync
	case v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC:
		return ConfigSyncOutOfSync
	case v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_APPLYING:
		return ConfigSyncApplying
	case v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_ERROR:
		return ConfigSyncError
	default:
		return ConfigSyncUnknown
	}
}

// Helper functions

func timeToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func timestampToTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func convertFromAPIState(state v1alpha1.AgentState) State {
	switch state {
	case v1alpha1.AgentState_AGENT_STATE_CONNECTED:
		return StateConnected
	case v1alpha1.AgentState_AGENT_STATE_DISCONNECTED:
		return StateDisconnected
	default:
		return StateUnknown
	}
}

func convertToAPIState(state State) v1alpha1.AgentState {
	switch state {
	case StateConnected:
		return v1alpha1.AgentState_AGENT_STATE_CONNECTED
	case StateDisconnected:
		return v1alpha1.AgentState_AGENT_STATE_DISCONNECTED
	default:
		return v1alpha1.AgentState_AGENT_STATE_UNKNOWN
	}
}

func convertToAPIConfigSync(status ConfigSyncStatus) v1alpha1.ConfigSyncStatus {
	switch status {
	case ConfigSyncInSync:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_IN_SYNC
	case ConfigSyncOutOfSync:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC
	case ConfigSyncApplying:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_APPLYING
	case ConfigSyncError:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_ERROR
	default:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_UNKNOWN
	}
}

func convertToAPIHealth(h *ComponentHealth) *v1alpha1.ComponentHealth {
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
			result.ComponentHealthMap[k] = convertToAPIHealth(v)
		}
	}
	return result
}

func convertToAPIEffectiveConfig(c *EffectiveConfig) *v1alpha1.EffectiveConfig {
	if c == nil {
		return nil
	}
	result := &v1alpha1.EffectiveConfig{
		ConfigMap: &v1alpha1.AgentConfigMap{},
	}
	if len(c.ConfigMap) > 0 {
		result.ConfigMap.ConfigMap = make(map[string]*v1alpha1.AgentConfigFile, len(c.ConfigMap))
		for k, v := range c.ConfigMap {
			result.ConfigMap.ConfigMap[k] = &v1alpha1.AgentConfigFile{
				Body:        v.Body,
				ContentType: v.ContentType,
			}
		}
	}
	return result
}

func convertToAPIRemoteStatus(s *RemoteConfigStatus) *v1alpha1.RemoteConfigStatus {
	if s == nil {
		return nil
	}
	return &v1alpha1.RemoteConfigStatus{
		LastRemoteConfigHash: s.LastRemoteConfigHash,
		Status:               v1alpha1.RemoteConfigStatuses(s.Status),
		ErrorMessage:         s.ErrorMessage,
	}
}

func convertRemoteConfigStatuses(status protobufs.RemoteConfigStatuses) RemoteConfigStatuses {
	switch status {
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED:
		return RemoteConfigStatusApplied
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING:
		return RemoteConfigStatusApplying
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED:
		return RemoteConfigStatusFailed
	default:
		return RemoteConfigStatusUnset
	}
}

func convertKeyValuesToMap(kvs []*protobufs.KeyValue) map[string]any {
	if kvs == nil {
		return nil
	}
	result := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		result[kv.GetKey()] = convertAnyValueToInterface(kv.GetValue())
	}
	return result
}

func convertAnyValueToInterface(v *protobufs.AnyValue) any {
	if v == nil {
		return nil
	}
	switch val := v.Value.(type) {
	case *protobufs.AnyValue_StringValue:
		return val.StringValue
	case *protobufs.AnyValue_BoolValue:
		return val.BoolValue
	case *protobufs.AnyValue_IntValue:
		return val.IntValue
	case *protobufs.AnyValue_DoubleValue:
		return val.DoubleValue
	case *protobufs.AnyValue_BytesValue:
		return val.BytesValue
	case *protobufs.AnyValue_ArrayValue:
		if val.ArrayValue == nil {
			return nil
		}
		arr := make([]any, len(val.ArrayValue.Values))
		for i, item := range val.ArrayValue.Values {
			arr[i] = convertAnyValueToInterface(item)
		}
		return arr
	case *protobufs.AnyValue_KvlistValue:
		if val.KvlistValue == nil {
			return nil
		}
		return convertKeyValuesToMap(val.KvlistValue.Values)
	default:
		return nil
	}
}

func toAPIKeyValues(m map[string]any) []*v1alpha1.KeyValue {
	if m == nil {
		return nil
	}
	result := make([]*v1alpha1.KeyValue, 0, len(m))
	for k, v := range m {
		result = append(result, &v1alpha1.KeyValue{
			Key:   k,
			Value: toAPIAnyValue(v),
		})
	}
	return result
}

func toAPIAnyValue(v any) *v1alpha1.AnyValue {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_StringValue{StringValue: val}}
	case bool:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BoolValue{BoolValue: val}}
	case int64:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_IntValue{IntValue: val}}
	case float64:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_DoubleValue{DoubleValue: val}}
	case []byte:
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BytesValue{BytesValue: val}}
	case []any:
		arr := &v1alpha1.ArrayValue{Values: make([]*v1alpha1.AnyValue, len(val))}
		for i, item := range val {
			arr.Values[i] = toAPIAnyValue(item)
		}
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_ArrayValue{ArrayValue: arr}}
	case map[string]any:
		kvlist := &v1alpha1.KeyValueList{Values: toAPIKeyValues(val)}
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_KvlistValue{KvlistValue: kvlist}}
	default:
		return nil
	}
}
