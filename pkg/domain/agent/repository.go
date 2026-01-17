package agent

import (
	"context"
	"errors"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// Common domain errors.
var (
	ErrAgentNotFound = errors.New("agent not found")
)

// Repository provides unified access to agent data.
// It abstracts the underlying storage complexity by assembling
// complete Agent aggregates from multiple stores.
type Repository interface {
	// Query operations - assemble complete Agent from multiple stores
	Get(ctx context.Context, agentID string) (*Agent, error)
	List(ctx context.Context) ([]*Agent, error)
	Exists(ctx context.Context, agentID string) (bool, error)

	// Registration operations
	Register(ctx context.Context, id, friendlyName string) error

	// Update operations - update specific aspects
	UpdateAttributes(ctx context.Context, agentID string, desc *protobufs.AgentDescription) error
	UpdateConnectionState(ctx context.Context, agentID string, state ConnectionState) error
	UpdateHealth(ctx context.Context, agentID string, health *protobufs.ComponentHealth) error
	UpdateEffectiveConfig(ctx context.Context, agentID string, config *protobufs.EffectiveConfig) error
	UpdateRemoteConfigStatus(ctx context.Context, agentID string, status *protobufs.RemoteConfigStatus) error

	// GetConnectionState retrieves only connection state (for OpAMP server optimization)
	GetConnectionState(ctx context.Context, agentID string) (*ConnectionState, error)
}
