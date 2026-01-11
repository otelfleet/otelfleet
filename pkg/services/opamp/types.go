package opamp

import (
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// AgentFullState represents the complete state of an agent as reported via OpAMP.
// It aggregates all status information: description, health, configuration, and more.
type AgentFullState struct {
	InstanceUID        []byte
	Description        *protobufs.AgentDescription
	Health             *protobufs.ComponentHealth
	EffectiveConfig    *protobufs.EffectiveConfig
	RemoteConfigStatus *protobufs.RemoteConfigStatus
	Capabilities       uint64
	SequenceNum        uint64
	LastSeen           time.Time
}
