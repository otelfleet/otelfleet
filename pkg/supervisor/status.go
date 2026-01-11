package supervisor

import (
	"runtime"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/util"
)

// BuildAgentDescription creates a complete AgentDescription with identifying
// and non-identifying attributes following semantic conventions.
func BuildAgentDescription(agentID string) *protobufs.AgentDescription {
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			util.KeyVal(AttributeOtelfleetAgentId, agentID),
			util.KeyVal("service.name", "otelfleet-agent"),
			util.KeyVal("service.instance.id", agentID),
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			util.KeyVal("os.type", runtime.GOOS),
			util.KeyVal("host.arch", runtime.GOARCH),
			util.KeyVal("process.runtime.name", "go"),
			util.KeyVal("process.runtime.version", runtime.Version()),
		},
	}
}

// BuildComponentHealth creates a ComponentHealth message with basic health info.
func BuildComponentHealth(healthy bool, status string, startTime time.Time) *protobufs.ComponentHealth {
	return &protobufs.ComponentHealth{
		Healthy:            healthy,
		Status:             status,
		StartTimeUnixNano:  uint64(startTime.UnixNano()),
		StatusTimeUnixNano: uint64(time.Now().UnixNano()),
	}
}

// BuildComponentHealthWithError creates a ComponentHealth with error information.
func BuildComponentHealthWithError(healthy bool, status, lastError string, startTime time.Time) *protobufs.ComponentHealth {
	return &protobufs.ComponentHealth{
		Healthy:            healthy,
		Status:             status,
		LastError:          lastError,
		StartTimeUnixNano:  uint64(startTime.UnixNano()),
		StatusTimeUnixNano: uint64(time.Now().UnixNano()),
	}
}

// BuildComponentHealthWithComponents creates a ComponentHealth with nested component health.
func BuildComponentHealthWithComponents(
	healthy bool,
	status string,
	startTime time.Time,
	components map[string]*protobufs.ComponentHealth,
) *protobufs.ComponentHealth {
	return &protobufs.ComponentHealth{
		Healthy:            healthy,
		Status:             status,
		StartTimeUnixNano:  uint64(startTime.UnixNano()),
		StatusTimeUnixNano: uint64(time.Now().UnixNano()),
		ComponentHealthMap: components,
	}
}

// GetCapabilities returns the full set of capabilities for the supervisor agent.
func GetCapabilities() uint64 {
	return uint64(
		protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus |
			protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig,
	)
}
