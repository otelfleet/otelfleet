package handler

import (
	"testing"

	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestToStringLabels(t *testing.T) {
	attrs := []*v1alpha1.KeyValue{
		{Key: "service.name", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_StringValue{StringValue: "collector"}}},
		{Key: "host.privileged", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BoolValue{BoolValue: true}}},
		{Key: "host.cpus", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_IntValue{IntValue: 8}}},
		{Key: "host.load", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_DoubleValue{DoubleValue: 1.5}}},
		{Key: "host.id", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_BytesValue{BytesValue: []byte("abc")}}},
		{Key: "unset"},
		{Key: "nested", Value: &v1alpha1.AnyValue{Value: &v1alpha1.AnyValue_ArrayValue{ArrayValue: &v1alpha1.ArrayValue{}}}},
	}

	require.Equal(t, map[string]string{
		"service.name":    "collector",
		"host.privileged": "true",
		"host.cpus":       "8",
		"host.load":       "1.5",
		"host.id":         "abc",
	}, toStringLabels(attrs))
}
