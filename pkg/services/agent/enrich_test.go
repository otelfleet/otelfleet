package agent

import (
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestEnrichAgentDescription(t *testing.T) {
	tests := []struct {
		name         string
		agent        *v1alpha1.AgentDescription
		desc         *protobufs.AgentDescription
		capabilities uint64
		wantCaps     []string
	}{
		{
			name:         "nil description does nothing",
			agent:        &v1alpha1.AgentDescription{},
			desc:         nil,
			capabilities: uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus),
			wantCaps:     nil,
		},
		{
			name:         "single capability",
			agent:        &v1alpha1.AgentDescription{},
			desc:         &protobufs.AgentDescription{},
			capabilities: uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus),
			wantCaps: []string{
				"ReportsStatus",
			},
		},
		{
			name:  "multiple capabilities",
			agent: &v1alpha1.AgentDescription{},
			desc:  &protobufs.AgentDescription{},
			capabilities: uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus) |
				uint64(protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig) |
				uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth),
			wantCaps: []string{
				"ReportsStatus",
				"AcceptsRemoteConfig",
				"ReportsHealth",
			},
		},
		{
			name:         "zero capabilities",
			agent:        &v1alpha1.AgentDescription{},
			desc:         &protobufs.AgentDescription{},
			capabilities: 0,
			wantCaps:     nil,
		},
		{
			name:  "populates attributes from description",
			agent: &v1alpha1.AgentDescription{},
			desc: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "test-service"}}},
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					{Key: "os.type", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "linux"}}},
				},
			},
			capabilities: 0,
			wantCaps:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enrichAgentDescription(tt.agent, tt.desc, tt.capabilities)

			if tt.wantCaps == nil {
				assert.Empty(t, tt.agent.Capabilities)
			} else {
				assert.ElementsMatch(t, tt.wantCaps, tt.agent.Capabilities)
			}

			// Check attributes are populated when desc is not nil
			if tt.desc != nil {
				if len(tt.desc.IdentifyingAttributes) > 0 {
					assert.NotNil(t, tt.agent.IdentifyingAttributes)
					assert.Equal(t, len(tt.desc.IdentifyingAttributes), len(tt.agent.IdentifyingAttributes))
				}
				if len(tt.desc.NonIdentifyingAttributes) > 0 {
					assert.NotNil(t, tt.agent.NonIdentifyingAttributes)
					assert.Equal(t, len(tt.desc.NonIdentifyingAttributes), len(tt.agent.NonIdentifyingAttributes))
				}
			}
		})
	}
}
