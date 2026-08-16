package deployment_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newManager(t *testing.T) deployment.Manager {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	kv := otelpebble.NewKVBroker(db).KeyValue("deployment")
	return deployment.NewManager(schema.NewStorageSchemaProto(kv))
}

func TestManagerRegisterAndList(t *testing.T) {
	ctx := context.Background()
	mgr := newManager(t)

	exists, err := mgr.Exists(ctx, "a")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, mgr.Register(ctx, "a", "agent-a"))
	require.NoError(t, mgr.Register(ctx, "b", "agent-b"))

	exists, err = mgr.Exists(ctx, "a")
	require.NoError(t, err)
	assert.True(t, exists)

	instances, err := mgr.List(ctx)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	desc, err := instances[0].GetDescription(ctx)
	require.NoError(t, err)
	assert.Equal(t, "a", desc.GetId())
	assert.Equal(t, "agent-a", desc.GetFriendlyName())

	require.NoError(t, mgr.Delete(ctx, "a"))
	_, err = mgr.Get(ctx, "a")
	require.Error(t, err)
}

func TestInstanceDescriptionMergesAttributes(t *testing.T) {
	ctx := context.Background()
	mgr := newManager(t)
	require.NoError(t, mgr.Register(ctx, "a", "agent-a"))

	inst := mgr.Instance("a")
	require.NoError(t, inst.SetDescription(ctx, &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{{
			Key:   "service.name",
			Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "collector"}},
		}},
	}))

	desc, err := inst.GetDescription(ctx)
	require.NoError(t, err)
	assert.Equal(t, "agent-a", desc.GetFriendlyName())
	require.Len(t, desc.GetIdentifyingAttributes(), 1)
	assert.Equal(t, "collector", desc.GetIdentifyingAttributes()[0].GetValue().GetStringValue())
}

func TestInstanceStatusAggregates(t *testing.T) {
	ctx := context.Background()
	mgr := newManager(t)
	require.NoError(t, mgr.Register(ctx, "a", "agent-a"))
	inst := mgr.Instance("a")

	status, err := inst.Status(ctx)
	require.NoError(t, err)
	assert.Nil(t, status.GetHealth())

	require.NoError(t, inst.SetHealth(ctx, &protobufs.ComponentHealth{Healthy: true}))
	require.NoError(t, inst.SetRemoteStatus(ctx, &protobufs.RemoteConfigStatus{
		Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}))
	require.NoError(t, inst.SetConnectionState(ctx, &v1alpha1.ConnectionStatus{
		State: v1alpha1.AgentState_AGENT_STATE_CONNECTED,
	}))
	require.NoError(t, inst.SetEffectiveConfig(ctx, &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {Body: []byte("first"), ContentType: "text/yaml"},
			},
		},
	}))

	status, err = inst.Status(ctx)
	require.NoError(t, err)
	assert.True(t, status.GetHealth().GetHealthy())
	assert.Equal(t, v1alpha1.RemoteConfigStatuses_REMOTE_CONFIG_STATUSES_APPLIED, status.GetRemoteConfigStatus().GetStatus())
	assert.Equal(t, v1alpha1.AgentState_AGENT_STATE_CONNECTED, status.GetConnStatus().GetState())
	assert.Equal(t, []byte("first"),
		status.GetEffectiveConfig().GetConfigMap().GetConfigMap()["config.yaml"].GetBody())
}

func TestInstanceHistory(t *testing.T) {
	ctx := context.Background()
	mgr := newManager(t)
	require.NoError(t, mgr.Register(ctx, "a", "agent-a"))
	inst := mgr.Instance("a")

	for _, body := range []string{"first", "second"} {
		require.NoError(t, inst.SetEffectiveConfig(ctx, &protobufs.EffectiveConfig{
			ConfigMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"config.yaml": {Body: []byte(body), ContentType: "text/yaml"},
				},
			},
		}))
	}

	history, err := inst.History(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, []byte("second"), history[0].GetConfigMap().GetConfigMap()["config.yaml"].GetBody())
	assert.Equal(t, []byte("first"), history[1].GetConfigMap().GetConfigMap()["config.yaml"].GetBody())
}
