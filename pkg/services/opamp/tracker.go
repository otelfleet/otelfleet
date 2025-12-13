package opamp

import (
	"sync"

	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
)

type AgentTracker interface {
	Get(id string) (*v1alpha1.AgentStatus, bool)
	Put(id string, status *v1alpha1.AgentStatus)
}

type inMemAgentTracker struct {
	agentsMu *sync.RWMutex
	agents   map[string]*v1alpha1.AgentStatus
}

func NewAgentTracker() AgentTracker {
	return &inMemAgentTracker{
		&sync.RWMutex{},
		map[string]*v1alpha1.AgentStatus{},
	}
}

func (i *inMemAgentTracker) Get(id string) (*v1alpha1.AgentStatus, bool) {
	i.agentsMu.RLock()
	defer i.agentsMu.RUnlock()
	got, ok := i.agents[id]
	return got, ok
}

func (i *inMemAgentTracker) Put(id string, status *v1alpha1.AgentStatus) {
	i.agentsMu.Lock()
	defer i.agentsMu.Unlock()
	i.agents[id] = status
}
