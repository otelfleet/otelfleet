package agent_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAgentServer_Status_ReturnsStoredData(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	agentID := "test-agent-123"

	// Set up agent connection state in store
	require.NoError(t, env.ConnectionStateStore.Put(ctx, agentID, &v1alpha1.AgentConnectionState{
		AgentId:     agentID,
		State:       v1alpha1.AgentState_AGENT_STATE_CONNECTED,
		ConnectedAt: timestamppb.Now(),
		LastSeen:    timestamppb.Now(),
	}))

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
	require.NoError(t, env.HealthStore.Put(ctx, agentID, health))

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
	require.NoError(t, env.EffectiveConfigStore.Put(ctx, agentID, config))

	// Store remote config status
	remoteStatus := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("hash-abc"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}
	require.NoError(t, env.RemoteStatusStore.Put(ctx, agentID, remoteStatus))

	// Call Status RPC
	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: agentID,
	})
	resp, err := env.AgentServer.Status(ctx, req)
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
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: "non-existent-agent",
	})
	resp, err := env.AgentServer.Status(ctx, req)
	require.NoError(t, err)

	// Should return unknown state when agent not in connection state store
	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_UNKNOWN, resp.Msg.Status.State)
	// Other fields should be nil since no data stored
	assert.Nil(t, resp.Msg.Status.Health)
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestAgentServer_Status_PartialData(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	agentID := "partial-agent"

	// Only set up connection state and health
	require.NoError(t, env.ConnectionStateStore.Put(ctx, agentID, &v1alpha1.AgentConnectionState{
		AgentId:     agentID,
		State:       v1alpha1.AgentState_AGENT_STATE_CONNECTED,
		ConnectedAt: timestamppb.Now(),
		LastSeen:    timestamppb.Now(),
	}))

	health := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "ok",
	}
	require.NoError(t, env.HealthStore.Put(ctx, agentID, health))

	req := connect.NewRequest(&v1alpha1.GetAgentStatusRequest{
		AgentId: agentID,
	})
	resp, err := env.AgentServer.Status(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_CONNECTED, resp.Msg.Status.State)
	require.NotNil(t, resp.Msg.Status.Health)
	assert.True(t, resp.Msg.Status.Health.Healthy)
	// These should be nil since not stored
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestAgentServer_GetAgent_Found(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	agentID := "test-agent-get"

	// Store agent description
	desc := &v1alpha1.AgentDescription{
		Id:           agentID,
		FriendlyName: "Test Agent",
	}
	require.NoError(t, env.AgentStore.Put(ctx, agentID, desc))

	req := connect.NewRequest(&v1alpha1.GetAgentRequest{
		AgentId: agentID,
	})
	resp, err := env.AgentServer.GetAgent(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, agentID, resp.Msg.Agent.Id)
	assert.Equal(t, "Test Agent", resp.Msg.Agent.FriendlyName)
}

func TestAgentServer_GetAgent_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	req := connect.NewRequest(&v1alpha1.GetAgentRequest{
		AgentId: "non-existent",
	})
	_, err := env.AgentServer.GetAgent(ctx, req)
	require.Error(t, err)

	// Should be a NotFound error
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}
