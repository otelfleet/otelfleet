// Package agent provides the domain layer for agent data management.
// It abstracts the underlying storage complexity and provides a unified
// interface for accessing agent data from multiple stores.
package agent

import (
	"time"
)

// Agent is the aggregate root containing all agent-related data.
// It assembles data from multiple stores into a cohesive domain model.
type Agent struct {
	// Core Identity (from bootstrap registration)
	ID           string
	FriendlyName string

	// OpAMP-Reported Metadata (from attributes store)
	Attributes AgentAttributes

	// Runtime State (from connection store)
	Connection ConnectionState

	// Status Information (from various stores)
	Status AgentRuntimeStatus
}

// AgentAttributes encapsulates identifying and non-identifying attributes
// reported by the agent via OpAMP.
type AgentAttributes struct {
	Identifying    map[string]any
	NonIdentifying map[string]any
}

// ConnectionState represents connection lifecycle information.
type ConnectionState struct {
	State          State
	LastSeen       *time.Time
	ConnectedAt    *time.Time
	DisconnectedAt *time.Time
	InstanceUID    []byte
	Capabilities   Capabilities
	SequenceNum    uint64
}

// Capabilities wraps the bitmask with helper methods.
type Capabilities uint64

// State represents connection state.
type State int

const (
	StateUnknown State = iota
	StateConnected
	StateDisconnected
)

// AgentRuntimeStatus represents runtime status from multiple sources.
type AgentRuntimeStatus struct {
	Health             *ComponentHealth
	EffectiveConfig    *EffectiveConfig
	RemoteConfigStatus *RemoteConfigStatus
	ConfigSyncStatus   ConfigSyncStatus
	ConfigSyncReason   string
}

// ConfigSyncStatus represents the unified config synchronization status.
type ConfigSyncStatus int

const (
	ConfigSyncUnknown ConfigSyncStatus = iota
	ConfigSyncInSync
	ConfigSyncOutOfSync
	ConfigSyncApplying
	ConfigSyncError
)

// ComponentHealth represents the health status of an agent component.
type ComponentHealth struct {
	Healthy            bool
	StartTimeUnixNano  uint64
	LastError          string
	Status             string
	StatusTimeUnixNano uint64
	ComponentHealthMap map[string]*ComponentHealth
}

// EffectiveConfig represents the current effective configuration of an agent.
type EffectiveConfig struct {
	ConfigMap map[string]*ConfigFile
}

// ConfigFile represents a single configuration file.
type ConfigFile struct {
	Body        []byte
	ContentType string
}

// RemoteConfigStatus represents the status of a remote configuration on an agent.
type RemoteConfigStatus struct {
	LastRemoteConfigHash []byte
	Status               RemoteConfigStatuses
	ErrorMessage         string
}

// RemoteConfigStatuses represents the remote config status enum.
type RemoteConfigStatuses int

const (
	RemoteConfigStatusUnset RemoteConfigStatuses = iota
	RemoteConfigStatusApplied
	RemoteConfigStatusApplying
	RemoteConfigStatusFailed
)

// IsConnected returns true if the agent is currently connected.
func (a *Agent) IsConnected() bool {
	return a.Connection.State == StateConnected
}

// CanReceiveConfig returns true if the agent has the capability to receive remote config.
func (a *Agent) CanReceiveConfig() bool {
	return a.Connection.Capabilities.HasAcceptsRemoteConfig()
}

// MatchesLabels checks if the agent's attributes match all the specified selector labels.
// Returns false if the selector is empty (to prevent accidentally matching all agents).
func (a *Agent) MatchesLabels(selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}

	// Build a map of agent attributes for easier lookup
	agentLabels := make(map[string]string)
	for k, v := range a.Attributes.Identifying {
		if str, ok := v.(string); ok {
			agentLabels[k] = str
		}
	}
	for k, v := range a.Attributes.NonIdentifying {
		if str, ok := v.(string); ok {
			agentLabels[k] = str
		}
	}

	// Check if all selector labels match
	for key, value := range selector {
		if agentLabels[key] != value {
			return false
		}
	}
	return true
}
