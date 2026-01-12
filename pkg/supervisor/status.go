package supervisor

import (
	"runtime"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/util"
)

// BuildAgentDescription creates a complete AgentDescription with identifying
// and non-identifying attributes following semantic conventions.
func (s *Supervisor) buildAgentDescription(agentID string) *protobufs.AgentDescription {
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
func (s *Supervisor) buildComponentHealth(healthy bool, status, lastError string, startTime time.Time) *protobufs.ComponentHealth {
	return &protobufs.ComponentHealth{
		Healthy: healthy,
		Status:  status,
		ComponentHealthMap: map[string]*protobufs.ComponentHealth{
			"example": {
				Healthy:           true,
				StartTimeUnixNano: uint64(s.startTime.UnixNano()),
				Status:            "some details here",
			},
		},
		StartTimeUnixNano:  uint64(startTime.UnixNano()),
		StatusTimeUnixNano: uint64(time.Now().UnixNano()),
		LastError:          lastError,
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
