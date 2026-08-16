package deployment

import (
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
)

func convertHealth(h *protobufs.ComponentHealth) *v1alpha1.ComponentHealth {
	if h == nil {
		return nil
	}
	out := &v1alpha1.ComponentHealth{
		Healthy:            h.GetHealthy(),
		StartTimeUnixNano:  h.GetStartTimeUnixNano(),
		LastError:          h.GetLastError(),
		Status:             h.GetStatus(),
		StatusTimeUnixNano: h.GetStatusTimeUnixNano(),
	}
	if len(h.GetComponentHealthMap()) > 0 {
		out.ComponentHealthMap = make(map[string]*v1alpha1.ComponentHealth, len(h.GetComponentHealthMap()))
		for k, v := range h.GetComponentHealthMap() {
			out.ComponentHealthMap[k] = convertHealth(v)
		}
	}
	return out
}

func convertEffectiveConfig(c *protobufs.EffectiveConfig) *v1alpha1.EffectiveConfig {
	if c == nil {
		return nil
	}
	files := c.GetConfigMap().GetConfigMap()
	out := &v1alpha1.EffectiveConfig{
		ConfigMap: &v1alpha1.AgentConfigMap{
			ConfigMap: make(map[string]*v1alpha1.AgentConfigFile, len(files)),
		},
	}
	for name, file := range files {
		out.ConfigMap.ConfigMap[name] = &v1alpha1.AgentConfigFile{
			Body:        file.GetBody(),
			ContentType: file.GetContentType(),
		}
	}
	return out
}

func convertRemoteStatus(s *protobufs.RemoteConfigStatus) *v1alpha1.RemoteConfigStatus {
	if s == nil {
		return nil
	}
	return &v1alpha1.RemoteConfigStatus{
		LastRemoteConfigHash: s.GetLastRemoteConfigHash(),
		Status:               v1alpha1.RemoteConfigStatuses(s.GetStatus()),
		ErrorMessage:         s.GetErrorMessage(),
	}
}

func convertKeyValues(kvs []*protobufs.KeyValue) []*v1alpha1.KeyValue {
	if len(kvs) == 0 {
		return nil
	}
	out := make([]*v1alpha1.KeyValue, 0, len(kvs))
	for _, kv := range kvs {
		out = append(out, &v1alpha1.KeyValue{
			Key:   kv.GetKey(),
			Value: convertAnyValue(kv.GetValue()),
		})
	}
	return out
}

func convertAnyValue(v *protobufs.AnyValue) *v1alpha1.AnyValue {
	switch val := v.GetValue().(type) {
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
		values := val.ArrayValue.GetValues()
		arr := &v1alpha1.ArrayValue{Values: make([]*v1alpha1.AnyValue, 0, len(values))}
		for _, item := range values {
			arr.Values = append(arr.Values, convertAnyValue(item))
		}
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_ArrayValue{ArrayValue: arr}}
	case *protobufs.AnyValue_KvlistValue:
		list := &v1alpha1.KeyValueList{Values: convertKeyValues(val.KvlistValue.GetValues())}
		return &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_KvlistValue{KvlistValue: list}}
	default:
		return nil
	}
}
