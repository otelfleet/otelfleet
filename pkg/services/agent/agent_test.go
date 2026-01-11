package agent_test

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/services/agent"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*agent.AgentServer, storage.KeyValue[*v1alpha1.AgentDescription], opamp.AgentTracker, storage.KeyValue[*protobufs.ComponentHealth], storage.KeyValue[*protobufs.EffectiveConfig], storage.KeyValue[*protobufs.RemoteConfigStatus]) {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default()

	agentStore := storage.NewProtoKV[*v1alpha1.AgentDescription](logger, broker.KeyValue("agents"))
	tracker := opamp.NewAgentTracker()
	healthStore := storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("agent-health"))
	configStore := storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("agent-effective-config"))
	statusStore := storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("agent-remote-config-status"))
	opampDesc := storage.NewProtoKV[*protobufs.AgentDescription](logger, broker.KeyValue("opamp-agent-description"))

	srv := agent.NewAgentServer(logger, agentStore, tracker, healthStore, configStore, statusStore, opampDesc)

	return srv, agentStore, tracker, healthStore, configStore, statusStore
}

func TestAgentServer_Status_ReturnsStoredData(t *testing.T) {
	srv, _, tracker, healthStore, configStore, statusStore := setupTestServer(t)

	ctx := context.Background()
	agentID := "test-agent-123"

	// Set up agent status in tracker
	tracker.PutStatus(agentID, &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AGENT_STATE_CONNECTED,
	})

	// Store health data
	health := &protobufs.ComponentHealth{
		Healthy:           true,
		StartTimeUnixNano: 1234567890,
		Status:            "running",
		ComponentHealthMap: map[string]*protobufs.ComponentHealth{
			"receiver/otlp": {
				Healthy: true,
				Status:  "receiving",
			},
		},
	}
	require.NoError(t, healthStore.Put(ctx, agentID, health))

	// Store effective config
	config := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {
					Body:        []byte("receivers:\n  otlp:"),
					ContentType: "text/yaml",
				},
			},
		},
	}
	require.NoError(t, configStore.Put(ctx, agentID, config))

	// Store remote config status
	remoteStatus := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("hash-abc"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}
	require.NoError(t, statusStore.Put(ctx, agentID, remoteStatus))

	// Call Status RPC
	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: agentID,
	})
	resp, err := srv.Status(ctx, req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_CONNECTED, resp.Msg.Status.State)

	// Check health
	require.NotNil(t, resp.Msg.Status.Health)
	assert.True(t, resp.Msg.Status.Health.Healthy)
	assert.Equal(t, "running", resp.Msg.Status.Health.Status)
	assert.NotNil(t, resp.Msg.Status.Health.ComponentHealthMap["receiver/otlp"])
	assert.True(t, resp.Msg.Status.Health.ComponentHealthMap["receiver/otlp"].Healthy)

	// Check effective config
	require.NotNil(t, resp.Msg.Status.EffectiveConfig)
	require.NotNil(t, resp.Msg.Status.EffectiveConfig.ConfigMap)
	assert.NotNil(t, resp.Msg.Status.EffectiveConfig.ConfigMap.ConfigMap["config.yaml"])
	assert.Equal(t, "text/yaml", resp.Msg.Status.EffectiveConfig.ConfigMap.ConfigMap["config.yaml"].ContentType)

	// Check remote config status
	require.NotNil(t, resp.Msg.Status.RemoteConfigStatus)
	assert.Equal(t, v1alpha1.RemoteConfigStatuses_REMOTE_CONFIG_STATUSES_APPLIED, resp.Msg.Status.RemoteConfigStatus.Status)
}

func TestAgentServer_Status_UnknownAgent(t *testing.T) {
	srv, _, _, _, _, _ := setupTestServer(t)

	ctx := context.Background()

	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: "non-existent-agent",
	})
	resp, err := srv.Status(ctx, req)
	require.NoError(t, err)

	// Should return unknown state when agent not tracked
	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_UNKNOWN, resp.Msg.Status.State)
	// Other fields should be nil since no data stored
	assert.Nil(t, resp.Msg.Status.Health)
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestAgentServer_Status_PartialData(t *testing.T) {
	srv, _, tracker, healthStore, _, _ := setupTestServer(t)

	ctx := context.Background()
	agentID := "partial-agent"

	// Only set up tracker and health
	tracker.PutStatus(agentID, &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AGENT_STATE_CONNECTED,
	})

	health := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "ok",
	}
	require.NoError(t, healthStore.Put(ctx, agentID, health))

	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: agentID,
	})
	resp, err := srv.Status(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_CONNECTED, resp.Msg.Status.State)
	require.NotNil(t, resp.Msg.Status.Health)
	assert.True(t, resp.Msg.Status.Health.Healthy)
	// These should be nil since not stored
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestAgentServer_GetAgent_Found(t *testing.T) {
	srv, agentStore, _, _, _, _ := setupTestServer(t)

	ctx := context.Background()
	agentID := "test-agent-get"

	// Store agent description
	desc := &v1alpha1.AgentDescription{
		Id:           agentID,
		FriendlyName: "Test Agent",
	}
	require.NoError(t, agentStore.Put(ctx, agentID, desc))

	req := connect.NewRequest(&v1alpha1.GetAgentRequest{
		AgentId: agentID,
	})
	resp, err := srv.GetAgent(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, agentID, resp.Msg.Agent.Id)
	assert.Equal(t, "Test Agent", resp.Msg.Agent.FriendlyName)
}

func TestAgentServer_GetAgent_NotFound(t *testing.T) {
	srv, _, _, _, _, _ := setupTestServer(t)

	ctx := context.Background()

	req := connect.NewRequest(&v1alpha1.GetAgentRequest{
		AgentId: "non-existent",
	})
	_, err := srv.GetAgent(ctx, req)
	require.Error(t, err)

	// Should be a NotFound error
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}
