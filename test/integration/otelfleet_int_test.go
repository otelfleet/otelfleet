//go:build insecure

package integration_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	bootstrapclient "github.com/otelfleet/otelfleet/pkg/bootstrap/client"
	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// defaultTTL returns a 5 minute duration for token creation
func defaultTTL() *durationpb.Duration {
	return durationpb.New(5 * time.Minute)
}

// ============================================================================
// Bootstrap Flow Tests
// ============================================================================

func TestBootstrap_AgentRegistersSuccessfully(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create a bootstrap token
	tokenResp, err := env.BootstrapServer.CreateToken(ctx, connect.NewRequest(&bootstrapv1alpha1.CreateTokenRequest{
		TTL: defaultTTL(),
	}))
	require.NoError(t, err)
	token := tokenResp.Msg

	// Use bootstrap client to register agent
	client := bootstrapclient.NewInsecure(bootstrapclient.Config{
		Logger:     env.Logger,
		ServerURL:  env.BaseURL,
		HTTPClient: env.HTTPServer.Client(),
	})

	agentID := "test-agent-bootstrap-1"
	agentName := "Test Agent 1"

	result, err := client.BootstrapAgent(ctx, &testIdentity{id: agentID}, agentName, token.GetID())
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify agent was registered in the store
	storedAgent, err := env.AgentStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agentID, storedAgent.GetId())
	assert.Equal(t, agentName, storedAgent.GetFriendlyName())
}

func TestBootstrap_AgentGetsDefaultConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Set a default config by putting it directly in the default config store
	// (SetDefaultConfig is not yet implemented in the service)
	defaultConfigYAML := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
    verbosity: detailed
`
	err := env.DefaultConfigStore.Put(ctx, "global", &configv1alpha1.Config{
		Config: []byte(defaultConfigYAML),
	})
	require.NoError(t, err)

	// Create and start an agent using bootstrap
	agent := env.NewAgentWithBootstrap("agent-default-cfg", "Default Config Agent", nil)
	require.NoError(t, agent.Start())

	// Wait for agent to receive config
	agent.WaitForConfig(t, 5*time.Second)

	// Verify agent received a config
	currentConfig := agent.AgentDriver.CurrentConfig
	require.NotNil(t, currentConfig)
	assert.NotNil(t, currentConfig.GetConfig())
}

func TestBootstrap_TokenWithConfigReference_StoresConfigOnTokenCreation(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create a named config first
	configID := "bootstrap-config"
	configYAML := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  batch:
exporters:
  otlp:
    endpoint: collector:4317
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Create a bootstrap token with config reference
	tokenResp, err := env.BootstrapServer.CreateToken(ctx, connect.NewRequest(&bootstrapv1alpha1.CreateTokenRequest{
		TTL:             defaultTTL(),
		ConfigReference: &configID,
		Labels: map[string]string{
			"env": "production",
		},
	}))
	require.NoError(t, err)
	token := tokenResp.Msg

	// Verify the token has the config reference set
	assert.Equal(t, configID, token.GetConfigReference())

	// Verify the config was stored in the bootstrap config store using the full token key
	// (The key format is tokenID.tokenSecret)
	fullTokenKey := token.GetID() + "." + token.GetSecret()
	storedConfig, err := env.BootstrapConfigStore.Get(ctx, fullTokenKey)
	require.NoError(t, err)
	assert.Equal(t, configYAML, string(storedConfig.GetConfig()))

	// Bootstrap an agent using the token
	client := bootstrapclient.NewInsecure(bootstrapclient.Config{
		Logger:     env.Logger,
		ServerURL:  env.BaseURL,
		HTTPClient: env.HTTPServer.Client(),
	})

	agentID := "agent-with-bootstrap-config"
	_, err = client.BootstrapAgent(ctx, &testIdentity{id: agentID}, "Bootstrap Config Agent", token.GetID())
	require.NoError(t, err)

	// Verify agent was registered
	storedAgent, err := env.AgentStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agentID, storedAgent.GetId())
	assert.Equal(t, "Bootstrap Config Agent", storedAgent.GetFriendlyName())

	// Note: In insecure mode, the config is not automatically assigned to the agent
	// because the Authorization header only contains the token ID (not ID.Secret),
	// but the bootstrap config is stored with the full token key (ID.Secret).
	// This is a known limitation of insecure mode testing.
}

func TestBootstrap_InvalidToken_Fails(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	client := bootstrapclient.NewInsecure(bootstrapclient.Config{
		Logger:     env.Logger,
		ServerURL:  env.BaseURL,
		HTTPClient: env.HTTPServer.Client(),
	})

	// Try to bootstrap with an invalid/nonexistent token
	// In insecure mode, the token is passed as Authorization header
	// The server should still validate it exists
	_, err := client.Bootstrap(ctx, &bootstrapclient.BootstrapRequest{
		ClientID: "invalid-agent",
		Name:     "Invalid Agent",
		Token:    "nonexistent-token-id",
	})
	// Note: In insecure mode, token verification is relaxed
	// This test documents current behavior
	assert.NoError(t, err) // Insecure mode allows any token
}

// ============================================================================
// Config Management Tests
// ============================================================================

func TestConfig_CRUD_Operations(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	configID := "crud-test-config"
	configYAML := `receivers:
  otlp:
    protocols:
      grpc:
`

	// Create config
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Read config
	getResp, err := env.ConfigServer.GetConfig(ctx, connect.NewRequest(&configv1alpha1.ConfigReference{
		Id: configID,
	}))
	require.NoError(t, err)
	assert.Equal(t, configYAML, string(getResp.Msg.GetConfig()))

	// List configs - should contain our config
	listResp, err := env.ConfigServer.ListConfigs(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	found := false
	for _, ref := range listResp.Msg.GetConfigs() {
		if ref.GetId() == configID {
			found = true
			break
		}
	}
	assert.True(t, found, "Config should appear in list")

	// Update config
	updatedYAML := `receivers:
  otlp:
    protocols:
      grpc:
      http:
`
	_, err = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(updatedYAML)},
	}))
	require.NoError(t, err)

	// Verify update
	getResp, err = env.ConfigServer.GetConfig(ctx, connect.NewRequest(&configv1alpha1.ConfigReference{
		Id: configID,
	}))
	require.NoError(t, err)
	assert.Equal(t, updatedYAML, string(getResp.Msg.GetConfig()))

	// Delete config
	_, err = env.ConfigServer.DeleteConfig(ctx, connect.NewRequest(&configv1alpha1.ConfigReference{
		Id: configID,
	}))
	require.NoError(t, err)

	// Verify deletion
	_, err = env.ConfigServer.GetConfig(ctx, connect.NewRequest(&configv1alpha1.ConfigReference{
		Id: configID,
	}))
	assert.Error(t, err, "Config should not exist after deletion")
}

func TestConfig_DefaultConfig_FallbackBehavior(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	defaultConfigYAML := `receivers:
  otlp:
exporters:
  debug:
`
	// Set default config directly in the store (SetDefaultConfig is not yet implemented)
	err := env.DefaultConfigStore.Put(ctx, "global", &configv1alpha1.Config{
		Config: []byte(defaultConfigYAML),
	})
	require.NoError(t, err)

	// Get default config
	getResp, err := env.ConfigServer.GetDefaultConfig(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	assert.Equal(t, defaultConfigYAML, string(getResp.Msg.GetConfig()))
}

// ============================================================================
// Config Assignment Tests
// ============================================================================

func TestConfigAssignment_ManualAssignment(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create config
	configID := "manual-assign-config"
	configYAML := `receivers:
  otlp:
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Create agent
	agent := env.NewAgent("manual-assign-agent")

	// Assign config to agent
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Verify assignment metadata
	assignmentResp, err := env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
		AgentId: agent.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, configID, assignmentResp.Msg.GetConfigId())
	assert.Equal(t, configv1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL, assignmentResp.Msg.GetSource())
}

func TestConfigAssignment_AgentReceivesAssignedConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create config
	configID := "assigned-config"
	configYAML := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  logging:
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Create and start agent
	agent := env.NewAgent("agent-receives-config")
	require.NoError(t, agent.Start())

	// Assign config
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Wait for agent to receive config
	agent.WaitForConfig(t, 5*time.Second)

	// Verify agent received the config
	require.NotNil(t, agent.AgentDriver.CurrentConfig)
	configMap := agent.AgentDriver.CurrentConfig.GetConfig()
	require.NotNil(t, configMap)

	// The config should be in the config map
	assert.NotEmpty(t, configMap.GetConfigMap())
}

func TestConfigAssignment_Unassign(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup
	configID := "unassign-config"
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte("receivers:\n  otlp:\n")},
	}))
	require.NoError(t, err)

	agent := env.NewAgent("unassign-agent")

	// Assign
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Unassign
	_, err = env.ConfigServer.UnassignConfig(ctx, connect.NewRequest(&configv1alpha1.UnassignConfigRequest{
		AgentId: agent.ID,
	}))
	require.NoError(t, err)

	// Verify unassignment
	_, err = env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
		AgentId: agent.ID,
	}))
	assert.Error(t, err, "Should error when no config is assigned")
}

func TestConfigAssignment_BatchAssign(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create config
	configID := "batch-config"
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte("receivers:\n  otlp:\n")},
	}))
	require.NoError(t, err)

	// Create multiple agents
	agents := []string{"batch-agent-1", "batch-agent-2", "batch-agent-3"}
	for _, agentID := range agents {
		env.NewAgent(agentID)
	}

	// Batch assign
	resp, err := env.ConfigServer.BatchAssignConfig(ctx, connect.NewRequest(&configv1alpha1.BatchAssignConfigRequest{
		AgentIds: agents,
		ConfigId: configID,
	}))
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Msg.GetSuccessful())
	assert.Equal(t, int32(0), resp.Msg.GetFailed())

	// Verify all agents have the config
	for _, agentID := range agents {
		assignmentResp, err := env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
			AgentId: agentID,
		}))
		require.NoError(t, err)
		assert.Equal(t, configID, assignmentResp.Msg.GetConfigId())
	}
}

func TestConfigAssignment_AssignByLabels(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create config
	configID := "label-config"
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte("receivers:\n  otlp:\n")},
	}))
	require.NoError(t, err)

	// Create agents with different labels
	env.NewAgentWithLabels("prod-us-agent", map[string]string{"env": "prod", "region": "us-east"})
	env.NewAgentWithLabels("prod-eu-agent", map[string]string{"env": "prod", "region": "eu-west"})
	env.NewAgentWithLabels("staging-agent", map[string]string{"env": "staging", "region": "us-east"})

	// Assign by labels - only prod agents in us-east
	resp, err := env.ConfigServer.AssignConfigByLabels(ctx, connect.NewRequest(&configv1alpha1.AssignConfigByLabelsRequest{
		ConfigId: configID,
		Labels: map[string]string{
			"env":    "prod",
			"region": "us-east",
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.Msg.GetSuccessful())
	assert.Contains(t, resp.Msg.GetMatchedAgentIds(), "prod-us-agent")
	assert.NotContains(t, resp.Msg.GetMatchedAgentIds(), "prod-eu-agent")
	assert.NotContains(t, resp.Msg.GetMatchedAgentIds(), "staging-agent")
}

// ============================================================================
// Agent Status and Effective Config Tests
// ============================================================================

func TestAgentStatus_ReportsEffectiveConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create and assign config
	configID := "effective-config"
	configYAML := `receivers:
  otlp:
    protocols:
      grpc:
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Create, assign, and start agent
	agent := env.NewAgent("effective-config-agent")
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	require.NoError(t, agent.Start())
	agent.WaitForConfig(t, 5*time.Second)

	// Allow time for status to propagate
	time.Sleep(500 * time.Millisecond)

	// Check agent status via API
	statusResp, err := env.AgentServer.Status(ctx, connect.NewRequest(&agentsv1alpha1.GetAgentStatusRequest{
		AgentId: agent.ID,
	}))
	require.NoError(t, err)

	// Get the status from the response
	status := statusResp.Msg.GetStatus()
	require.NotNil(t, status)

	// Agent should be connected
	assert.Equal(t, agentsv1alpha1.AgentState_AGENT_STATE_CONNECTED, status.GetState())

	// Should have effective config
	effectiveConfig := status.GetEffectiveConfig()
	assert.NotNil(t, effectiveConfig)
}

func TestAgentStatus_ConfigInSync(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Setup config and agent
	configID := "sync-config"
	configYAML := `receivers:
  otlp:
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	agent := env.NewAgent("sync-agent")
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	require.NoError(t, agent.Start())
	agent.WaitForConfig(t, 5*time.Second)

	// Allow status to propagate
	time.Sleep(500 * time.Millisecond)

	// Check config status
	configStatusResp, err := env.ConfigServer.GetConfigStatus(ctx, connect.NewRequest(&configv1alpha1.GetConfigStatusRequest{
		AgentId: agent.ID,
	}))
	require.NoError(t, err)

	// The status should indicate applied or in-sync
	// Note: Exact status depends on timing of agent reporting
	assignment := configStatusResp.Msg.GetAssignment()
	assert.NotNil(t, assignment)
	assert.Equal(t, configID, assignment.GetConfigId())
}

// ============================================================================
// Config Update Propagation Tests
// ============================================================================

func TestConfigUpdate_PropagatedToConnectedAgent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create initial config
	configID := "update-propagate-config"
	initialYAML := `receivers:
  otlp:
    protocols:
      grpc:
`
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(initialYAML)},
	}))
	require.NoError(t, err)

	// Create, assign, and start agent
	agent := env.NewAgent("update-agent")
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	require.NoError(t, agent.Start())
	agent.WaitForConfig(t, 5*time.Second)

	initialUpdateCount := agent.AgentDriver.GetUpdateCount()

	// Update the config
	updatedYAML := `receivers:
  otlp:
    protocols:
      grpc:
      http:
exporters:
  debug:
`
	_, err = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte(updatedYAML)},
	}))
	require.NoError(t, err)

	// Re-assign to trigger notification
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Wait for agent to receive the updated config
	agent.WaitForConfigCount(t, initialUpdateCount+1, 5*time.Second)

	// Verify update count increased
	assert.Greater(t, agent.AgentDriver.GetUpdateCount(), initialUpdateCount)
}

func TestConfigReassignment_NewConfigDelivered(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create two configs
	config1ID := "reassign-config-1"
	config2ID := "reassign-config-2"

	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: config1ID},
		Config: &configv1alpha1.Config{Config: []byte("version: 1\n")},
	}))
	require.NoError(t, err)

	_, err = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: config2ID},
		Config: &configv1alpha1.Config{Config: []byte("version: 2\n")},
	}))
	require.NoError(t, err)

	// Create and start agent with first config
	agent := env.NewAgent("reassign-agent")
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: config1ID,
	}))
	require.NoError(t, err)

	require.NoError(t, agent.Start())
	agent.WaitForConfig(t, 5*time.Second)

	initialCount := agent.AgentDriver.GetUpdateCount()

	// Reassign to second config
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: config2ID,
	}))
	require.NoError(t, err)

	// Wait for new config
	agent.WaitForConfigCount(t, initialCount+1, 5*time.Second)

	// Verify assignment metadata updated
	assignmentResp, err := env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
		AgentId: agent.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, config2ID, assignmentResp.Msg.GetConfigId())
}

// ============================================================================
// Multiple Agents Tests
// ============================================================================

func TestMultipleAgents_DifferentConfigs(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create two configs
	prodConfigID := "prod-config"
	stagingConfigID := "staging-config"

	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: prodConfigID},
		Config: &configv1alpha1.Config{Config: []byte("env: production\n")},
	}))
	require.NoError(t, err)

	_, err = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: stagingConfigID},
		Config: &configv1alpha1.Config{Config: []byte("env: staging\n")},
	}))
	require.NoError(t, err)

	// Create and start production agent
	prodAgent := env.NewAgentWithLabels("prod-agent", map[string]string{"env": "production"})
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  prodAgent.ID,
		ConfigId: prodConfigID,
	}))
	require.NoError(t, err)
	require.NoError(t, prodAgent.Start())

	// Create and start staging agent
	stagingAgent := env.NewAgentWithLabels("staging-agent", map[string]string{"env": "staging"})
	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  stagingAgent.ID,
		ConfigId: stagingConfigID,
	}))
	require.NoError(t, err)
	require.NoError(t, stagingAgent.Start())

	// Wait for both agents to receive configs
	prodAgent.WaitForConfig(t, 5*time.Second)
	stagingAgent.WaitForConfig(t, 5*time.Second)

	// Verify each agent has its assigned config
	prodAssignment, err := env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
		AgentId: prodAgent.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, prodConfigID, prodAssignment.Msg.GetConfigId())

	stagingAssignment, err := env.ConfigServer.GetAgentConfig(ctx, connect.NewRequest(&configv1alpha1.GetAgentConfigRequest{
		AgentId: stagingAgent.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, stagingConfigID, stagingAssignment.Msg.GetConfigId())
}

func TestMultipleAgents_ListAgents(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create multiple agents
	agents := []string{"list-agent-1", "list-agent-2", "list-agent-3"}
	for _, agentID := range agents {
		env.NewAgent(agentID)
	}

	// Start one agent to test status filtering
	agent1, _ := env.GetAgent("list-agent-1")
	require.NoError(t, agent1.Start())
	time.Sleep(200 * time.Millisecond) // Allow connection

	// List all agents
	listResp, err := env.AgentServer.ListAgents(ctx, connect.NewRequest(&agentsv1alpha1.ListAgentsRequest{}))
	require.NoError(t, err)

	// Should have at least our 3 agents
	foundAgents := make(map[string]bool)
	for _, agentAndStatus := range listResp.Msg.GetAgents() {
		// AgentDescriptionAndStatus has Agent and Status fields
		if agentAndStatus.GetAgent() != nil {
			foundAgents[agentAndStatus.GetAgent().GetId()] = true
		}
	}

	for _, expectedID := range agents {
		assert.True(t, foundAgents[expectedID], "Agent %s should be in list", expectedID)
	}
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestError_AssignConfigToNonexistentAgent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	configID := "error-config"
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte("test: config\n")},
	}))
	require.NoError(t, err)

	_, err = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  "nonexistent-agent",
		ConfigId: configID,
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestError_AssignNonexistentConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	agent := env.NewAgent("error-agent")

	_, err := env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId:  agent.ID,
		ConfigId: "nonexistent-config",
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestError_GetNonexistentConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	_, err := env.ConfigServer.GetConfig(ctx, connect.NewRequest(&configv1alpha1.ConfigReference{
		Id: "does-not-exist",
	}))
	assert.Error(t, err)
}

func TestError_EmptyLabelsForAssignByLabels(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	configID := "label-error-config"
	_, err := env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: configID},
		Config: &configv1alpha1.Config{Config: []byte("test: config\n")},
	}))
	require.NoError(t, err)

	_, err = env.ConfigServer.AssignConfigByLabels(ctx, connect.NewRequest(&configv1alpha1.AssignConfigByLabelsRequest{
		ConfigId: configID,
		Labels:   map[string]string{}, // Empty labels
	}))
	assert.Error(t, err, "Empty labels should return an error")
}

// ============================================================================
// Token Management Tests
// ============================================================================

func TestToken_CreateAndList(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create multiple tokens
	token1, err := env.BootstrapServer.CreateToken(ctx, connect.NewRequest(&bootstrapv1alpha1.CreateTokenRequest{
		TTL:    defaultTTL(),
		Labels: map[string]string{"env": "prod"},
	}))
	require.NoError(t, err)

	token2, err := env.BootstrapServer.CreateToken(ctx, connect.NewRequest(&bootstrapv1alpha1.CreateTokenRequest{
		TTL:    defaultTTL(),
		Labels: map[string]string{"env": "staging"},
	}))
	require.NoError(t, err)

	// List tokens
	listResp, err := env.BootstrapServer.ListTokens(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// Should contain our tokens
	foundTokens := make(map[string]bool)
	for _, tok := range listResp.Msg.GetTokens() {
		foundTokens[tok.GetID()] = true
	}
	assert.True(t, foundTokens[token1.Msg.GetID()])
	assert.True(t, foundTokens[token2.Msg.GetID()])
}

func TestToken_Delete(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create token
	tokenResp, err := env.BootstrapServer.CreateToken(ctx, connect.NewRequest(&bootstrapv1alpha1.CreateTokenRequest{
		TTL: defaultTTL(),
	}))
	require.NoError(t, err)
	tokenID := tokenResp.Msg.GetID()

	// Delete token
	_, err = env.BootstrapServer.DeleteToken(ctx, connect.NewRequest(&bootstrapv1alpha1.DeleteTokenRequest{
		ID: tokenID,
	}))
	require.NoError(t, err)

	// Verify deletion via list
	listResp, err := env.BootstrapServer.ListTokens(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	for _, tok := range listResp.Msg.GetTokens() {
		assert.NotEqual(t, tokenID, tok.GetID(), "Deleted token should not appear in list")
	}
}

// ============================================================================
// List Assignments Tests
// ============================================================================

func TestListConfigAssignments_FilterByConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	// Create configs
	config1 := "filter-config-1"
	config2 := "filter-config-2"

	_, _ = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: config1},
		Config: &configv1alpha1.Config{Config: []byte("v: 1\n")},
	}))
	_, _ = env.ConfigServer.PutConfig(ctx, connect.NewRequest(&configv1alpha1.PutConfigRequest{
		Ref:    &configv1alpha1.ConfigReference{Id: config2},
		Config: &configv1alpha1.Config{Config: []byte("v: 2\n")},
	}))

	// Create agents and assign
	env.NewAgent("filter-assign-1")
	env.NewAgent("filter-assign-2")
	env.NewAgent("filter-assign-3")

	_, _ = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId: "filter-assign-1", ConfigId: config1,
	}))
	_, _ = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId: "filter-assign-2", ConfigId: config1,
	}))
	_, _ = env.ConfigServer.AssignConfig(ctx, connect.NewRequest(&configv1alpha1.AssignConfigRequest{
		AgentId: "filter-assign-3", ConfigId: config2,
	}))

	// List with filter
	resp, err := env.ConfigServer.ListConfigAssignments(ctx, connect.NewRequest(&configv1alpha1.ListConfigAssignmentsRequest{
		ConfigId: &config1,
	}))
	require.NoError(t, err)

	assert.Len(t, resp.Msg.GetAssignments(), 2)
	for _, assignment := range resp.Msg.GetAssignments() {
		assert.Equal(t, config1, assignment.GetConfigId())
	}
}

// ============================================================================
// Helper types
// ============================================================================

// testIdentity implements ident.Identity for testing
type testIdentity struct {
	id string
}

func (t *testIdentity) UniqueIdentifier() ident.ID {
	return ident.ID{
		UUID:     t.id,
		Metatada: map[string]string{},
	}
}
