package util

import (
	"strconv"

	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/router"
)

func CollectorDescriptionToLabels(desc *v1alpha1.CollectorDescription) router.CollectorLabels {
	return router.CollectorLabels{
		Identifying:    toStringLabels(desc.GetIdentifyingAttributes()),
		NonIdentifying: toStringLabels(desc.GetNonIdentifyingAttributes()),
	}
}

func toStringLabels(attrs []*v1alpha1.KeyValue) map[string]string {
	labels := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		value, ok := labelValue(attr.GetValue())
		if !ok {
			continue
		}
		labels[attr.GetKey()] = value
	}
	return labels
}

// labelValue reports false for composite values, which cannot be matched by label selectors.
func labelValue(v *v1alpha1.AnyValue) (string, bool) {
	switch val := v.GetValue().(type) {
	case *v1alpha1.AnyValue_StringValue:
		return val.StringValue, true
	case *v1alpha1.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue), true
	case *v1alpha1.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10), true
	case *v1alpha1.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'g', -1, 64), true
	case *v1alpha1.AnyValue_BytesValue:
		return string(val.BytesValue), true
	}
	return "", false
}
