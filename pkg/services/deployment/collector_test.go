package deployment_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCollectorServer_Status_ReturnsStoredData(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	collectorID := "test-collector-123"

	// Register the collector first (required by repository)
	require.NoError(t, env.CollectorRepo.Register(ctx, collectorID, "Test Collector"))

	// Set up agent connection state in store
	require.NoError(t, env.ConnectionStateStore.Put(ctx, collectorID, &v1alpha1.CollectorConnectionState{
		CollectorId:     collectorID,
		State:       v1alpha1.CollectorState_COLLECTOR_STATE_CONNECTED,
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
	require.NoError(t, env.HealthStore.Put(ctx, collectorID, health))

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
	require.NoError(t, env.EffectiveConfigStore.Put(ctx, collectorID, config))

	// Store remote config status
	remoteStatus := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("hash-abc"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}
	require.NoError(t, env.RemoteStatusStore.Put(ctx, collectorID, remoteStatus))

	// Call Status RPC
	req := connect.NewRequest(&v1alpha1.GetCollectorStatusRequest{
		CollectorId: collectorID,
	})
	resp, err := env.CollectorServer.Status(ctx, req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, v1alpha1.CollectorState_COLLECTOR_STATE_CONNECTED, resp.Msg.Status.State)

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

func TestCollectorServer_Status_UnknownAgent(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	collectorID := "non-existent-collector"

	// Register the collector but don't add any status data
	require.NoError(t, env.CollectorRepo.Register(ctx, collectorID, "Unknown Collector"))

	req := connect.NewRequest(&v1alpha1.GetCollectorStatusRequest{
		CollectorId: collectorID,
	})
	resp, err := env.CollectorServer.Status(ctx, req)
	require.NoError(t, err)

	// Should return unknown state when agent has no connection state
	assert.Equal(t, v1alpha1.CollectorState_COLLECTOR_STATE_UNKNOWN, resp.Msg.Status.State)
	// Other fields should be nil since no data stored
	assert.Nil(t, resp.Msg.Status.Health)
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestCollectorServer_Status_PartialData(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	collectorID := "partial-collector"

	// Register the collector first (required by repository)
	require.NoError(t, env.CollectorRepo.Register(ctx, collectorID, "Partial Collector"))

	// Only set up connection state and health
	require.NoError(t, env.ConnectionStateStore.Put(ctx, collectorID, &v1alpha1.CollectorConnectionState{
		CollectorId:     collectorID,
		State:       v1alpha1.CollectorState_COLLECTOR_STATE_CONNECTED,
		ConnectedAt: timestamppb.Now(),
		LastSeen:    timestamppb.Now(),
	}))

	health := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "ok",
	}
	require.NoError(t, env.HealthStore.Put(ctx, collectorID, health))

	req := connect.NewRequest(&v1alpha1.GetCollectorStatusRequest{
		CollectorId: collectorID,
	})
	resp, err := env.CollectorServer.Status(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, v1alpha1.CollectorState_COLLECTOR_STATE_CONNECTED, resp.Msg.Status.State)
	require.NotNil(t, resp.Msg.Status.Health)
	assert.True(t, resp.Msg.Status.Health.Healthy)
	// These should be nil since not stored
	assert.Nil(t, resp.Msg.Status.EffectiveConfig)
	assert.Nil(t, resp.Msg.Status.RemoteConfigStatus)
}

func TestCollectorServer_GetCollector_Found(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()
	collectorID := "test-collector-get"

	// Store agent description
	desc := &v1alpha1.CollectorDescription{
		Id:           collectorID,
		FriendlyName: "Test Collector",
	}
	require.NoError(t, env.CollectorStore.Put(ctx, collectorID, desc))

	req := connect.NewRequest(&v1alpha1.GetCollectorRequest{
		CollectorId: collectorID,
	})
	resp, err := env.CollectorServer.GetCollector(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, collectorID, resp.Msg.Collector.Id)
	assert.Equal(t, "Test Collector", resp.Msg.Collector.FriendlyName)
}

func TestCollectorServer_GetCollector_NotFound(t *testing.T) {
	env := testutil.NewTestEnv(t)
	ctx := context.Background()

	req := connect.NewRequest(&v1alpha1.GetCollectorRequest{
		CollectorId: "non-existent",
	})
	_, err := env.CollectorServer.GetCollector(ctx, req)
	require.Error(t, err)

	// Should be a NotFound error
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}
