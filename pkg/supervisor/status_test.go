package supervisor_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAgentDescription(t *testing.T) {
	agentID := "test-agent-123"
	desc := supervisor.BuildAgentDescription(agentID)

	require.NotNil(t, desc)
	require.NotEmpty(t, desc.IdentifyingAttributes)

	// Check for required identifying attributes
	attrs := keyValueSliceToMap(desc.IdentifyingAttributes)

	// Must have agent ID
	assert.Contains(t, attrs, supervisor.AttributeOtelfleetAgentId)
	assert.Equal(t, agentID, attrs[supervisor.AttributeOtelfleetAgentId])

	// Should have service name
	assert.Contains(t, attrs, "service.name")

	// Check non-identifying attributes
	require.NotEmpty(t, desc.NonIdentifyingAttributes)
	nonIdAttrs := keyValueSliceToMap(desc.NonIdentifyingAttributes)

	// Should have OS info
	assert.Contains(t, nonIdAttrs, "os.type")
	assert.Equal(t, runtime.GOOS, nonIdAttrs["os.type"])

	// Should have architecture
	assert.Contains(t, nonIdAttrs, "host.arch")
	assert.Equal(t, runtime.GOARCH, nonIdAttrs["host.arch"])
}

func TestBuildAgentDescription_HasServiceInstanceId(t *testing.T) {
	agentID := "agent-instance-test"
	desc := supervisor.BuildAgentDescription(agentID)

	attrs := keyValueSliceToMap(desc.IdentifyingAttributes)
	assert.Contains(t, attrs, "service.instance.id")
	assert.Equal(t, agentID, attrs["service.instance.id"])
}

func TestBuildComponentHealth(t *testing.T) {
	startTime := time.Now()
	health := supervisor.BuildComponentHealth(true, "running", startTime)

	require.NotNil(t, health)
	assert.True(t, health.Healthy)
	assert.Equal(t, "running", health.Status)
	assert.Equal(t, uint64(startTime.UnixNano()), health.StartTimeUnixNano)
	assert.NotZero(t, health.StatusTimeUnixNano)
}

func TestBuildComponentHealth_Unhealthy(t *testing.T) {
	startTime := time.Now().Add(-time.Hour)
	health := supervisor.BuildComponentHealth(false, "error", startTime)

	require.NotNil(t, health)
	assert.False(t, health.Healthy)
	assert.Equal(t, "error", health.Status)
}

func TestBuildComponentHealthWithError(t *testing.T) {
	startTime := time.Now()
	health := supervisor.BuildComponentHealthWithError(false, "failed", "connection timeout", startTime)

	require.NotNil(t, health)
	assert.False(t, health.Healthy)
	assert.Equal(t, "failed", health.Status)
	assert.Equal(t, "connection timeout", health.LastError)
}

func TestBuildComponentHealthWithComponents(t *testing.T) {
	startTime := time.Now()
	components := map[string]*protobufs.ComponentHealth{
		"receiver/otlp": {
			Healthy: true,
			Status:  "receiving",
		},
		"exporter/otlp": {
			Healthy: false,
			Status:  "connection_error",
		},
	}

	health := supervisor.BuildComponentHealthWithComponents(true, "running", startTime, components)

	require.NotNil(t, health)
	assert.True(t, health.Healthy)
	require.NotNil(t, health.ComponentHealthMap)
	assert.Len(t, health.ComponentHealthMap, 2)

	receiver, ok := health.ComponentHealthMap["receiver/otlp"]
	require.True(t, ok)
	assert.True(t, receiver.Healthy)

	exporter, ok := health.ComponentHealthMap["exporter/otlp"]
	require.True(t, ok)
	assert.False(t, exporter.Healthy)
}

func TestGetCapabilities(t *testing.T) {
	caps := supervisor.GetCapabilities()

	// Should report status
	assert.True(t, caps&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus) != 0)

	// Should accept remote config
	assert.True(t, caps&uint64(protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig) != 0)

	// Should report remote config
	assert.True(t, caps&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig) != 0)

	// Should report health
	assert.True(t, caps&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth) != 0)

	// Should report effective config
	assert.True(t, caps&uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig) != 0)
}

// Helper to convert KeyValue slice to map for easier testing
func keyValueSliceToMap(kvs []*protobufs.KeyValue) map[string]string {
	result := make(map[string]string)
	for _, kv := range kvs {
		if sv := kv.Value.GetStringValue(); sv != "" {
			result[kv.Key] = sv
		}
	}
	return result
}
