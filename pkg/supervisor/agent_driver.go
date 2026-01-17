package supervisor

import (
	"context"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// AgentDriver defines the interface for managing OpenTelemetry collectors
// in a particular environment like baremetal, docker, kubernetes, etc...
// This allows for different implementations, including mocks for testing.
type AgentDriver interface {
	// Update applies a new configuration to the managed collector.
	// It should skip the update if the config hash matches the current hash.
	Update(ctx context.Context, incoming *protobufs.AgentRemoteConfig) error

	// GetConfigMap returns the current effective configuration as an AgentConfigMap.
	GetConfigMap() (*protobufs.AgentConfigMap, error)

	// GetCurrentHash returns the hash of the currently applied configuration.
	GetCurrentHash() []byte

	// Shutdown gracefully stops the running agent
	Shutdown() error
}
