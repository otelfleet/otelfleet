package deployment

import (
	"strings"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// Capabilities wraps the bitmask with helper methods.
type Capabilities uint64

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
