package opamp

import (
	"sync"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
)

// AgentTracker tracks agent state and status.
type AgentTracker interface {
	// proto

	GetStatus(id string) (*v1alpha1.AgentStatus, bool)
	PutStatus(id string, status *v1alpha1.AgentStatus)

	// downstream message aggregation

	GetFullState(id string) (*AgentFullState, bool)
	PutFullState(id string, state *AgentFullState)

	//FIXME: hack
	GetCapabilities(id string) uint64

	// TODO : add as agent cleanup hook
	Delete(id string)

	// Message processing with sequence number tracking
	// Returns true if full state report is needed (sequence gap detected)
	// FIXME: maybe fail in the distributed case of gaps?
	UpdateFromMessage(id string, msg *protobufs.AgentToServer) bool
}

type inMemAgentTracker struct {
	mu         sync.RWMutex
	agents     map[string]*v1alpha1.AgentStatus
	fullStates map[string]*AgentFullState
}

// NewAgentTracker creates a new in-memory agent tracker.
func NewAgentTracker() AgentTracker {
	return &inMemAgentTracker{
		agents:     make(map[string]*v1alpha1.AgentStatus),
		fullStates: make(map[string]*AgentFullState),
	}
}

func (t *inMemAgentTracker) GetCapabilities(id string) uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	got, ok := t.fullStates[id]
	if !ok {
		return 0
	}
	return got.Capabilities
}

func (t *inMemAgentTracker) GetStatus(id string) (*v1alpha1.AgentStatus, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	got, ok := t.agents[id]
	return got, ok
}

func (t *inMemAgentTracker) PutStatus(id string, status *v1alpha1.AgentStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agents[id] = status
}

func (t *inMemAgentTracker) GetFullState(id string) (*AgentFullState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	state, ok := t.fullStates[id]
	return state, ok
}

func (t *inMemAgentTracker) PutFullState(id string, state *AgentFullState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fullStates[id] = state
}

func (t *inMemAgentTracker) Delete(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.agents, id)
	delete(t.fullStates, id)
}

func (t *inMemAgentTracker) ListAgentIDs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ids := make([]string, 0, len(t.fullStates))
	for id := range t.fullStates {
		ids = append(ids, id)
	}
	return ids
}

// UpdateFromMessage updates the agent state from an incoming message.
// Returns true if a full state report is needed (sequence gap detected).
func (t *inMemAgentTracker) UpdateFromMessage(id string, msg *protobufs.AgentToServer) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, exists := t.fullStates[id]
	if !exists {
		state = &AgentFullState{
			InstanceUID: msg.InstanceUid,
			LastSeen:    time.Now(),
		}
		t.fullStates[id] = state
	}

	// Check for sequence gap (status compression support)
	needsFullState := false
	if exists && msg.SequenceNum > 0 {
		expectedSeq := state.SequenceNum + 1
		if msg.SequenceNum != expectedSeq {
			needsFullState = true
		}
	}

	// Update all present fields
	if msg.AgentDescription != nil {
		state.Description = msg.AgentDescription
	}
	if msg.Health != nil {
		state.Health = msg.Health
	}
	if msg.EffectiveConfig != nil {
		state.EffectiveConfig = msg.EffectiveConfig
	}
	if msg.RemoteConfigStatus != nil {
		state.RemoteConfigStatus = msg.RemoteConfigStatus
	}
	if msg.Capabilities != 0 {
		state.Capabilities = msg.Capabilities
	}
	state.SequenceNum = msg.SequenceNum

	return needsFullState
}
