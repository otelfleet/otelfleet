package agent_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStores struct {
	registry         storage.KeyValue[*agentsv1alpha1.AgentDescription]
	attributes       storage.KeyValue[*protobufs.AgentDescription]
	connection       storage.KeyValue[*agentsv1alpha1.AgentConnectionState]
	health           storage.KeyValue[*protobufs.ComponentHealth]
	effective        storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatus     storage.KeyValue[*protobufs.RemoteConfigStatus]
	configAssignment storage.KeyValue[*configv1alpha1.ConfigAssignment]
}

func setupTest(t *testing.T) (agent.Repository, *testStores) {
	t.Helper()

	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default()

	stores := &testStores{
		registry:         storage.NewProtoKV[*agentsv1alpha1.AgentDescription](logger, broker.KeyValue("registry")),
		attributes:       storage.NewProtoKV[*protobufs.AgentDescription](logger, broker.KeyValue("attributes")),
		connection:       storage.NewProtoKV[*agentsv1alpha1.AgentConnectionState](logger, broker.KeyValue("connection")),
		health:           storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("health")),
		effective:        storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("effective")),
		remoteStatus:     storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("remote-status")),
		configAssignment: storage.NewProtoKV[*configv1alpha1.ConfigAssignment](logger, broker.KeyValue("config-assignment")),
	}

	repo := agent.NewRepository(
		logger,
		stores.registry,
		stores.attributes,
		stores.connection,
		stores.health,
		stores.effective,
		stores.remoteStatus,
		stores.configAssignment,
	)

	return repo, stores
}

func TestRepository_Register_And_Exists(t *testing.T) {
	repo, _ := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-1"
	friendlyName := "Test Agent One"

	// Agent should not exist initially
	exists, err := repo.Exists(ctx, agentID)
	require.NoError(t, err)
	assert.False(t, exists)

	// Register the agent
	err = repo.Register(ctx, agentID, friendlyName)
	require.NoError(t, err)

	// Now agent should exist
	exists, err = repo.Exists(ctx, agentID)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepository_Get_NotFound(t *testing.T) {
	repo, _ := setupTest(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent-agent")
	assert.ErrorIs(t, err, agent.ErrAgentNotFound)
}

func TestRepository_Get_BasicAgent(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-2"
	friendlyName := "Test Agent Two"

	// Store registration directly
	require.NoError(t, stores.registry.Put(ctx, agentID, &agentsv1alpha1.AgentDescription{
		Id:           agentID,
		FriendlyName: friendlyName,
	}))

	// Get agent
	ag, err := repo.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agentID, ag.ID)
	assert.Equal(t, friendlyName, ag.FriendlyName)
}

func TestRepository_Get_WithAttributes(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-attrs"

	// Store registration
	require.NoError(t, stores.registry.Put(ctx, agentID, &agentsv1alpha1.AgentDescription{
		Id:           agentID,
		FriendlyName: "Agent With Attributes",
	}))

	// Store OpAMP attributes
	require.NoError(t, stores.attributes.Put(ctx, agentID, &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "my-service"}}},
			{Key: "service.version", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "1.0.0"}}},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{Key: "host.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "server-1"}}},
		},
	}))

	// Get agent
	ag, err := repo.Get(ctx, agentID)
	require.NoError(t, err)

	// Verify identifying attributes
	assert.Equal(t, "my-service", ag.Attributes.Identifying["service.name"])
	assert.Equal(t, "1.0.0", ag.Attributes.Identifying["service.version"])

	// Verify non-identifying attributes
	assert.Equal(t, "server-1", ag.Attributes.NonIdentifying["host.name"])
}

func TestRepository_Get_WithStatus(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-status"

	// Store registration
	require.NoError(t, stores.registry.Put(ctx, agentID, &agentsv1alpha1.AgentDescription{
		Id: agentID,
	}))

	// Store health
	require.NoError(t, stores.health.Put(ctx, agentID, &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "running",
	}))

	// Store effective config
	require.NoError(t, stores.effective.Put(ctx, agentID, &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {Body: []byte("test: config")},
			},
		},
	}))

	// Get agent
	ag, err := repo.Get(ctx, agentID)
	require.NoError(t, err)

	// Verify health
	require.NotNil(t, ag.Status.Health)
	assert.True(t, ag.Status.Health.Healthy)
	assert.Equal(t, "running", ag.Status.Health.Status)

	// Verify effective config
	require.NotNil(t, ag.Status.EffectiveConfig)
	assert.Contains(t, ag.Status.EffectiveConfig.ConfigMap, "config.yaml")
}

func TestRepository_List(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	// Register multiple agents
	agents := []string{"agent-1", "agent-2", "agent-3"}
	for _, id := range agents {
		require.NoError(t, stores.registry.Put(ctx, id, &agentsv1alpha1.AgentDescription{
			Id:           id,
			FriendlyName: "Agent " + id,
		}))
	}

	// List agents
	result, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify all agents are present
	ids := make([]string, len(result))
	for i, ag := range result {
		ids[i] = ag.ID
	}
	for _, expected := range agents {
		assert.Contains(t, ids, expected)
	}
}

func TestRepository_UpdateAttributes(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-update"

	// Register agent first
	require.NoError(t, repo.Register(ctx, agentID, "Test Agent"))

	// Update attributes
	err := repo.UpdateAttributes(ctx, agentID, &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{Key: "env", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "production"}}},
		},
	})
	require.NoError(t, err)

	// Verify via direct store access
	attrs, err := stores.attributes.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Len(t, attrs.IdentifyingAttributes, 1)
	assert.Equal(t, "env", attrs.IdentifyingAttributes[0].Key)
}

func TestRepository_UpdateConnectionState(t *testing.T) {
	repo, _ := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-conn"

	// Register agent first
	require.NoError(t, repo.Register(ctx, agentID, "Test Agent"))

	// Update connection state
	err := repo.UpdateConnectionState(ctx, agentID, agent.ConnectionState{
		State: agent.StateConnected,
	})
	require.NoError(t, err)

	// Get agent and verify connection state
	ag, err := repo.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agent.StateConnected, ag.Connection.State)
}

func TestRepository_UpdateHealth(t *testing.T) {
	repo, _ := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-health"

	// Register agent first
	require.NoError(t, repo.Register(ctx, agentID, "Test Agent"))

	// Update health
	err := repo.UpdateHealth(ctx, agentID, &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "healthy",
		ComponentHealthMap: map[string]*protobufs.ComponentHealth{
			"receiver/otlp": {Healthy: true, Status: "receiving"},
		},
	})
	require.NoError(t, err)

	// Get agent and verify health
	ag, err := repo.Get(ctx, agentID)
	require.NoError(t, err)
	require.NotNil(t, ag.Status.Health)
	assert.True(t, ag.Status.Health.Healthy)
	assert.Equal(t, "healthy", ag.Status.Health.Status)
}

func TestRepository_GetConnectionState(t *testing.T) {
	repo, stores := setupTest(t)
	ctx := context.Background()

	agentID := "test-agent-get-conn"

	// Store connection state directly
	require.NoError(t, stores.connection.Put(ctx, agentID, &agentsv1alpha1.AgentConnectionState{
		AgentId: agentID,
		State:   agentsv1alpha1.AgentState_AGENT_STATE_CONNECTED,
	}))

	// Get connection state
	state, err := repo.GetConnectionState(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agent.StateConnected, state.State)
}

func TestRepository_GetConnectionState_NotFound(t *testing.T) {
	repo, _ := setupTest(t)
	ctx := context.Background()

	_, err := repo.GetConnectionState(ctx, "nonexistent")
	assert.ErrorIs(t, err, agent.ErrAgentNotFound)
}
