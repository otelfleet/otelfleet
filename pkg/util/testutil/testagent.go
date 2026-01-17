package testutil

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapclient "github.com/otelfleet/otelfleet/pkg/bootstrap/client"
	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

// testIdentity implements ident.Identity for testing purposes.
type testIdentity struct {
	id string
}

func (t *testIdentity) UniqueIdentifier() ident.ID {
	return ident.ID{
		UUID:     t.id,
		Metatada: map[string]string{},
	}
}

// TestAgent wraps a Supervisor with a mock AgentDriver for testing.
// It uses the actual Supervisor code and can optionally go through the bootstrap process.
type TestAgent struct {
	ID          string
	Supervisor  *supervisor.Supervisor
	AgentDriver *MockAgentDriver

	// Internal state
	mu      sync.Mutex
	started bool
	logger  *slog.Logger
	env     *TestEnv
}

// NewAgent creates a new test agent and registers it with the TestEnv.
// The agent is NOT bootstrapped - it's directly registered in the agent store.
// Use NewAgentWithBootstrap for a full bootstrap flow test.
// The agent is not started automatically; call Start() to connect to the server.
func (e *TestEnv) NewAgent(agentID string) *TestAgent {
	return e.NewAgentWithLabels(agentID, nil)
}

// NewAgentWithLabels creates a new test agent with the specified labels.
// Labels are stored in the agent's identifying attributes.
// The agent is directly registered (no bootstrap). Use NewAgentWithBootstrap for bootstrap testing.
func (e *TestEnv) NewAgentWithLabels(agentID string, labels map[string]string) *TestAgent {
	e.t.Helper()

	logger := e.Logger.With("agent_id", agentID)

	// Create mock agent driver
	agentDriver := NewMockAgentDriver(nil)

	// Create test identity
	identity := &testIdentity{id: agentID}

	// Create supervisor with mock agent driver
	sup := supervisor.NewSupervisor(
		logger,
		nil, // no TLS for tests
		e.OpampURL,
		identity,
		agentDriver,
	)

	agent := &TestAgent{
		ID:          agentID,
		Supervisor:  sup,
		AgentDriver: agentDriver,
		logger:      logger,
		env:         e,
	}

	// Register agent directly in the agent store (simulates pre-registered agent)
	ctx := context.Background()
	var attrs []*agentsv1alpha1.KeyValue
	for k, v := range labels {
		attrs = append(attrs, &agentsv1alpha1.KeyValue{
			Key:   k,
			Value: &agentsv1alpha1.AnyValue{Value: &agentsv1alpha1.AnyValue_StringValue{StringValue: v}},
		})
	}
	agentDesc := &agentsv1alpha1.AgentDescription{
		Id:                    agentID,
		IdentifyingAttributes: attrs,
	}
	require.NoError(e.t, e.AgentStore.Put(ctx, agentID, agentDesc))

	// Track the agent
	e.mu.Lock()
	e.agents[agentID] = agent
	e.mu.Unlock()

	return agent
}

// NewAgentWithBootstrap creates a new test agent that goes through the full bootstrap process.
// It creates a bootstrap token, uses the shared bootstrap client package to call the Bootstrap RPC,
// and then creates the supervisor. This tests the complete agent registration flow.
func (e *TestEnv) NewAgentWithBootstrap(agentID string, name string, labels map[string]string) *TestAgent {
	e.t.Helper()
	ctx := context.Background()

	logger := e.Logger.With("agent_id", agentID)

	// Step 1: Create a bootstrap token via the BootstrapServer
	tokenReq := &bootstrapv1alpha1.CreateTokenRequest{
		TTL:    durationpb.New(5 * time.Minute),
		Labels: labels,
	}
	tokenResp, err := e.BootstrapServer.CreateToken(ctx, connect.NewRequest(tokenReq))
	require.NoError(e.t, err, "failed to create bootstrap token")
	token := tokenResp.Msg

	// Step 2: Use the shared bootstrap client package (insecure mode for tests)
	client := bootstrapclient.NewInsecure(bootstrapclient.Config{
		Logger:     logger.With("component", "bootstrap"),
		ServerURL:  e.BaseURL,
		HTTPClient: e.HTTPServer.Client(),
	})

	// Create test identity
	identity := &testIdentity{id: agentID}

	// Call BootstrapAgent which verifies token and registers the agent
	_, err = client.BootstrapAgent(ctx, identity, name, token.GetID())
	require.NoError(e.t, err, "bootstrap failed")

	// Step 3: Verify agent was registered in the store
	storedAgent, err := e.AgentStore.Get(ctx, agentID)
	require.NoError(e.t, err, "agent should be registered after bootstrap")
	require.Equal(e.t, agentID, storedAgent.GetId())

	// Step 4: Create mock agent driver and supervisor
	agentDriver := NewMockAgentDriver(nil)

	sup := supervisor.NewSupervisor(
		logger,
		nil, // no TLS for tests
		e.OpampURL,
		identity,
		agentDriver,
	)

	agent := &TestAgent{
		ID:          agentID,
		Supervisor:  sup,
		AgentDriver: agentDriver,
		logger:      logger,
		env:         e,
	}

	// Track the agent
	e.mu.Lock()
	e.agents[agentID] = agent
	e.mu.Unlock()

	return agent
}

// Start connects the agent to the OpAMP server using the actual Supervisor.Start() method.
func (a *TestAgent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return nil
	}

	if err := a.Supervisor.Start(); err != nil {
		return err
	}

	a.started = true
	return nil
}

// Stop disconnects the agent from the OpAMP server.
func (a *TestAgent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return nil
	}

	if err := a.Supervisor.Shutdown(); err != nil {
		return err
	}

	a.started = false
	return nil
}

// IsStarted returns whether the agent has been started.
func (a *TestAgent) IsStarted() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.started
}

// WaitForConfig waits for the agent to receive a configuration.
func (a *TestAgent) WaitForConfig(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a.AgentDriver.CurrentConfig != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("agent %s did not receive config within %v", a.ID, timeout)
}

// WaitForConfigCount waits until the agent has received at least n config updates.
func (a *TestAgent) WaitForConfigCount(t *testing.T, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a.AgentDriver.GetUpdateCount() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("agent %s did not receive %d configs within %v (got %d)", a.ID, n, timeout, a.AgentDriver.GetUpdateCount())
}
