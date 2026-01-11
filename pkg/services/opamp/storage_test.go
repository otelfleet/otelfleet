package opamp_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/google/go-cmp/cmp"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func setupTestStorage(t *testing.T) storage.KVBroker {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return otelpebble.NewKVBroker(db)
}

func TestAgentHealthStore_PutAndGet(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.ComponentHealth](slog.Default(), broker.KeyValue("agent-health"))

	agentID := "agent-123"
	health := &protobufs.ComponentHealth{
		Healthy:           true,
		StartTimeUnixNano: uint64(time.Now().UnixNano()),
		Status:            "running",
		ComponentHealthMap: map[string]*protobufs.ComponentHealth{
			"receiver/otlp": {
				Healthy: true,
				Status:  "receiving",
			},
		},
	}

	ctx := context.Background()

	// Store health
	err := store.Put(ctx, agentID, health)
	require.NoError(t, err)

	// Retrieve health
	retrieved, err := store.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(health, retrieved, protocmp.Transform()))
}

func TestAgentHealthStore_GetNotFound(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.ComponentHealth](slog.Default(), broker.KeyValue("agent-health"))

	ctx := context.Background()

	// Should return error for non-existent agent
	_, err := store.Get(ctx, "non-existent")
	require.Error(t, err)
}

func TestAgentHealthStore_Update(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.ComponentHealth](slog.Default(), broker.KeyValue("agent-health"))

	agentID := "agent-123"
	ctx := context.Background()

	// Initial health
	health1 := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "starting",
	}
	require.NoError(t, store.Put(ctx, agentID, health1))

	// Update health
	health2 := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "running",
	}
	require.NoError(t, store.Put(ctx, agentID, health2))

	// Should return updated health
	retrieved, err := store.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, "running", retrieved.Status)
}

func TestAgentHealthStore_Delete(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.ComponentHealth](slog.Default(), broker.KeyValue("agent-health"))

	agentID := "agent-123"
	ctx := context.Background()

	health := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "running",
	}
	require.NoError(t, store.Put(ctx, agentID, health))

	// Delete
	err := store.Delete(ctx, agentID)
	require.NoError(t, err)

	// Should be gone
	_, err = store.Get(ctx, agentID)
	require.Error(t, err)
}

func TestAgentHealthStore_ListKeys(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.ComponentHealth](slog.Default(), broker.KeyValue("agent-health"))

	ctx := context.Background()

	// Store health for multiple agents
	require.NoError(t, store.Put(ctx, "agent-1", &protobufs.ComponentHealth{Status: "ok"}))
	require.NoError(t, store.Put(ctx, "agent-2", &protobufs.ComponentHealth{Status: "ok"}))
	require.NoError(t, store.Put(ctx, "agent-3", &protobufs.ComponentHealth{Status: "ok"}))

	// List should return all agent IDs
	ids, err := store.ListKeys(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"agent-1", "agent-2", "agent-3"}, ids)
}

func TestAgentEffectiveConfigStore_PutAndGet(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.EffectiveConfig](slog.Default(), broker.KeyValue("agent-effective-config"))

	agentID := "agent-123"
	config := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {
					Body:        []byte("receivers:\n  otlp:\n    protocols:\n      grpc:"),
					ContentType: "text/yaml",
				},
			},
		},
	}

	ctx := context.Background()

	// Store config
	err := store.Put(ctx, agentID, config)
	require.NoError(t, err)

	// Retrieve config
	retrieved, err := store.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(config, retrieved, protocmp.Transform()))
}

func TestAgentEffectiveConfigStore_GetNotFound(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.EffectiveConfig](slog.Default(), broker.KeyValue("agent-effective-config"))

	ctx := context.Background()

	_, err := store.Get(ctx, "non-existent")
	require.Error(t, err)
}

func TestAgentRemoteConfigStatusStore_PutAndGet(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.RemoteConfigStatus](slog.Default(), broker.KeyValue("agent-remote-config-status"))

	agentID := "agent-123"
	status := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("config-hash-abc"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}

	ctx := context.Background()

	// Store status
	err := store.Put(ctx, agentID, status)
	require.NoError(t, err)

	// Retrieve status
	retrieved, err := store.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(status, retrieved, protocmp.Transform()))
}

func TestAgentRemoteConfigStatusStore_FailedStatus(t *testing.T) {
	broker := setupTestStorage(t)
	store := storage.NewProtoKV[*protobufs.RemoteConfigStatus](slog.Default(), broker.KeyValue("agent-remote-config-status"))

	agentID := "agent-123"
	status := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("bad-config"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
		ErrorMessage:         "invalid configuration: missing required field",
	}

	ctx := context.Background()

	require.NoError(t, store.Put(ctx, agentID, status))

	retrieved, err := store.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, retrieved.Status)
	assert.Equal(t, "invalid configuration: missing required field", retrieved.ErrorMessage)
}
