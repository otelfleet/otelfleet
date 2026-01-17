package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util/configsync"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
)

// repository implements the Repository interface using existing storage.KeyValue stores.
type repository struct {
	logger *slog.Logger

	// Existing stores (same as current services)
	registryStore        storage.KeyValue[*v1alpha1.AgentDescription]
	attributesStore      storage.KeyValue[*protobufs.AgentDescription]
	connectionStore      storage.KeyValue[*v1alpha1.AgentConnectionState]
	healthStore          storage.KeyValue[*protobufs.ComponentHealth]
	effectiveStore       storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore    storage.KeyValue[*protobufs.RemoteConfigStatus]
	configAssignmentStore storage.KeyValue[*configv1alpha1.ConfigAssignment]
}

// NewRepository creates a new agent repository with the specified stores.
func NewRepository(
	logger *slog.Logger,
	registryStore storage.KeyValue[*v1alpha1.AgentDescription],
	attributesStore storage.KeyValue[*protobufs.AgentDescription],
	connectionStore storage.KeyValue[*v1alpha1.AgentConnectionState],
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	effectiveStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
	configAssignmentStore storage.KeyValue[*configv1alpha1.ConfigAssignment],
) Repository {
	return &repository{
		logger:               logger,
		registryStore:        registryStore,
		attributesStore:      attributesStore,
		connectionStore:      connectionStore,
		healthStore:          healthStore,
		effectiveStore:       effectiveStore,
		remoteStatusStore:    remoteStatusStore,
		configAssignmentStore: configAssignmentStore,
	}
}

// Get assembles the complete Agent domain model from multiple stores.
func (r *repository) Get(ctx context.Context, agentID string) (*Agent, error) {
	// 1. Get core registration data (required)
	registration, err := r.registryStore.Get(ctx, agentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent registration: %w", err)
	}

	agent := &Agent{
		ID:           registration.GetId(),
		FriendlyName: registration.GetFriendlyName(),
	}

	// 2. Enrich with attributes (optional - may not exist yet)
	if attrs, err := r.attributesStore.Get(ctx, agentID); err == nil {
		agent.Attributes = ConvertAttributes(attrs)
	} else if !grpcutil.IsErrorNotFound(err) {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get agent attributes")
	}

	// 3. Enrich with connection state (optional)
	if conn, err := r.connectionStore.Get(ctx, agentID); err == nil {
		agent.Connection = ConvertConnectionState(conn)
	} else if !grpcutil.IsErrorNotFound(err) {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get connection state")
	}

	// 4. Enrich with status information (all optional)
	agent.Status = r.assembleStatus(ctx, agentID)

	return agent, nil
}

// List returns all agents with their complete state.
func (r *repository) List(ctx context.Context) ([]*Agent, error) {
	registrations, err := r.registryStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agents := make([]*Agent, 0, len(registrations))
	for _, reg := range registrations {
		agent, err := r.Get(ctx, reg.GetId())
		if err != nil {
			// Log but don't fail the entire list
			r.logger.With("agent_id", reg.GetId(), "err", err).Warn("failed to get agent during list")
			continue
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// Exists checks if an agent is registered.
func (r *repository) Exists(ctx context.Context, agentID string) (bool, error) {
	_, err := r.registryStore.Get(ctx, agentID)
	if grpcutil.IsErrorNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check agent existence: %w", err)
	}
	return true, nil
}

// Register creates the initial agent registration.
func (r *repository) Register(ctx context.Context, id, friendlyName string) error {
	return r.registryStore.Put(ctx, id, &v1alpha1.AgentDescription{
		Id:           id,
		FriendlyName: friendlyName,
	})
}

// UpdateAttributes stores OpAMP-reported agent description.
func (r *repository) UpdateAttributes(ctx context.Context, agentID string, desc *protobufs.AgentDescription) error {
	return r.attributesStore.Put(ctx, agentID, desc)
}

// UpdateConnectionState stores connection lifecycle state.
func (r *repository) UpdateConnectionState(ctx context.Context, agentID string, state ConnectionState) error {
	protoState := ConnectionStateToProto(agentID, state)
	return r.connectionStore.Put(ctx, agentID, protoState)
}

// UpdateHealth stores component health.
func (r *repository) UpdateHealth(ctx context.Context, agentID string, health *protobufs.ComponentHealth) error {
	return r.healthStore.Put(ctx, agentID, health)
}

// UpdateEffectiveConfig stores effective config.
func (r *repository) UpdateEffectiveConfig(ctx context.Context, agentID string, config *protobufs.EffectiveConfig) error {
	return r.effectiveStore.Put(ctx, agentID, config)
}

// UpdateRemoteConfigStatus stores remote config status.
func (r *repository) UpdateRemoteConfigStatus(ctx context.Context, agentID string, status *protobufs.RemoteConfigStatus) error {
	return r.remoteStatusStore.Put(ctx, agentID, status)
}

// GetConnectionState retrieves only connection state (optimized for OpAMP server).
func (r *repository) GetConnectionState(ctx context.Context, agentID string) (*ConnectionState, error) {
	conn, err := r.connectionStore.Get(ctx, agentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get connection state: %w", err)
	}
	state := ConvertConnectionState(conn)
	return &state, nil
}

// assembleStatus gathers all status-related data.
func (r *repository) assembleStatus(ctx context.Context, agentID string) AgentRuntimeStatus {
	status := AgentRuntimeStatus{}

	if health, err := r.healthStore.Get(ctx, agentID); err == nil {
		status.Health = ConvertHealth(health)
	} else if !grpcutil.IsErrorNotFound(err) {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get health")
	}

	if config, err := r.effectiveStore.Get(ctx, agentID); err == nil {
		status.EffectiveConfig = ConvertEffectiveConfig(config)
	} else if !grpcutil.IsErrorNotFound(err) {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get effective config")
	}

	if remoteStatus, err := r.remoteStatusStore.Get(ctx, agentID); err == nil {
		status.RemoteConfigStatus = ConvertRemoteConfigStatus(remoteStatus)
	} else if !grpcutil.IsErrorNotFound(err) {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get remote config status")
	}

	status.ConfigSyncStatus, status.ConfigSyncReason = r.computeConfigSync(ctx, agentID)

	return status
}

// computeConfigSync computes the config sync status using the shared utility.
func (r *repository) computeConfigSync(ctx context.Context, agentID string) (ConfigSyncStatus, string) {
	assignment, err := r.configAssignmentStore.Get(ctx, agentID)
	if grpcutil.IsErrorNotFound(err) {
		return ConfigSyncUnknown, "no assigned config"
	} else if err != nil {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to get config assignment")
		return ConfigSyncUnknown, "internal error"
	}

	v1Status, reason, err := configsync.ComputeConfigSyncStatus(ctx, agentID, assignment.GetConfigHash(), r.remoteStatusStore)
	if err != nil {
		r.logger.With("agent_id", agentID, "err", err).Debug("failed to compute config sync status")
		return ConfigSyncUnknown, "internal error"
	}
	return ConvertConfigSyncStatus(v1Status), reason
}

// Delete removes an agent and all associated data from all stores.
// This is a best-effort operation - it attempts to delete from all stores
// even if some deletions fail. Registry is deleted last to ensure the agent
// remains "visible" until cleanup is complete.
func (r *repository) Delete(ctx context.Context, agentID string) error {
	// Check if agent exists first
	exists, err := r.Exists(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to check agent existence: %w", err)
	}
	if !exists {
		return ErrAgentNotFound
	}

	r.logger.With("agent_id", agentID).Info("deleting agent from all stores")

	// Delete from all stores in reverse dependency order (registry last)
	// Log failures but continue - agent may not have data in all stores
	stores := []struct {
		name  string
		store interface{ Delete(context.Context, string) error }
	}{
		{"configAssignment", r.configAssignmentStore},
		{"remoteStatus", r.remoteStatusStore},
		{"effective", r.effectiveStore},
		{"health", r.healthStore},
		{"connection", r.connectionStore},
		{"attributes", r.attributesStore},
	}

	for _, s := range stores {
		if err := s.store.Delete(ctx, agentID); err != nil {
			if !grpcutil.IsErrorNotFound(err) {
				r.logger.With("agent_id", agentID, "store", s.name, "err", err).Warn("failed to delete from store")
			}
		}
	}

	// Delete registry last - this gates Exists() checks
	if err := r.registryStore.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("failed to delete agent registry: %w", err)
	}

	r.logger.With("agent_id", agentID).Info("agent deleted successfully")
	return nil
}
