package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util"
)

// MockAgentDriver simulates config application without spawning processes.
// It implements supervisor.AgentDriver for use in tests.
type MockAgentDriver struct {
	mu sync.Mutex

	// CurrentConfig holds the most recently applied configuration.
	CurrentConfig *protobufs.AgentRemoteConfig

	// CurrentHash holds the hash of the currently applied configuration.
	CurrentHash []byte

	// ConfigHistory stores all configurations that have been applied.
	ConfigHistory []*protobufs.AgentRemoteConfig

	// ReportHealthFn is called when health should be reported.
	ReportHealthFn func(healthy bool, status, lastError string)

	// FailNextUpdate causes the next Update call to return an error.
	FailNextUpdate bool

	// FailUpdateError is the error to return when FailNextUpdate is true.
	FailUpdateError error

	// UpdateDelay adds artificial delay to Update calls.
	UpdateDelay time.Duration

	// UpdateCount tracks the number of successful updates.
	UpdateCount int
}

// Ensure MockAgentDriver implements AgentDriver.
var _ supervisor.AgentDriver = (*MockAgentDriver)(nil)

// NewMockAgentDriver creates a new MockAgentDriver with the given health reporting function.
func NewMockAgentDriver(reportFn func(bool, string, string)) *MockAgentDriver {
	return &MockAgentDriver{
		ReportHealthFn:  reportFn,
		ConfigHistory:   make([]*protobufs.AgentRemoteConfig, 0),
		FailUpdateError: errors.New("mock update failure"),
	}
}

// Update applies a new configuration.
func (m *MockAgentDriver) Update(ctx context.Context, incoming *protobufs.AgentRemoteConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.UpdateDelay > 0 {
		select {
		case <-time.After(m.UpdateDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.FailNextUpdate {
		m.FailNextUpdate = false
		return m.FailUpdateError
	}

	// Skip if hash matches
	incomingHash := incoming.GetConfigHash()
	if len(m.CurrentHash) > 0 && len(incomingHash) > 0 {
		if string(m.CurrentHash) == string(incomingHash) {
			return nil
		}
	}

	m.CurrentConfig = incoming
	m.CurrentHash = util.HashAgentConfigMap(incoming.GetConfig())
	m.ConfigHistory = append(m.ConfigHistory, incoming)
	m.UpdateCount++

	return nil
}

// GetConfigMap returns the current effective configuration.
func (m *MockAgentDriver) GetConfigMap() (*protobufs.AgentConfigMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CurrentConfig == nil {
		return &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"default": {
					Body:        []byte("none"),
					ContentType: "text/yaml",
				},
			},
		}, nil
	}

	return m.CurrentConfig.GetConfig(), nil
}

// GetCurrentHash returns the hash of the currently applied configuration.
func (m *MockAgentDriver) GetCurrentHash() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CurrentHash
}

// Shutdown is a no-op for the mock.
func (m *MockAgentDriver) Shutdown() error {
	return nil
}

// GetUpdateCount returns the number of successful updates.
func (m *MockAgentDriver) GetUpdateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.UpdateCount
}

// GetConfigHistory returns a copy of the config history.
func (m *MockAgentDriver) GetConfigHistory() []*protobufs.AgentRemoteConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*protobufs.AgentRemoteConfig, len(m.ConfigHistory))
	copy(result, m.ConfigHistory)
	return result
}

// Reset clears all state in the mock.
func (m *MockAgentDriver) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CurrentConfig = nil
	m.CurrentHash = nil
	m.ConfigHistory = make([]*protobufs.AgentRemoteConfig, 0)
	m.UpdateCount = 0
	m.FailNextUpdate = false
}
