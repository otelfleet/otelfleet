package opamp_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/google/go-cmp/cmp"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func setupTestServer(t *testing.T) (*opamp.Server, storage.KeyValue[*protobufs.ComponentHealth], storage.KeyValue[*protobufs.EffectiveConfig], storage.KeyValue[*protobufs.RemoteConfigStatus]) {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default()

	healthStore := storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("agent-health"))
	configStore := storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("agent-effective-config"))
	statusStore := storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("agent-remote-config-status"))
	opampDesc := storage.NewProtoKV[*protobufs.AgentDescription](logger, broker.KeyValue("opamp-agent-description"))

	server := opamp.NewServer(
		logger,
		nil,
		opamp.NewAgentTracker(),
		healthStore,
		configStore,
		statusStore,
		opampDesc,
	)

	return server, healthStore, configStore, statusStore
}

func TestServer_OnMessage_PersistsHealth(t *testing.T) {
	server, healthStore, _, _ := setupTestServer(t)

	instanceUID := []byte("test-agent-health-persist")
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

	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		Health:      health,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := server.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify health was persisted to storage
	stored, err := healthStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(health, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsEffectiveConfig(t *testing.T) {
	server, _, configStore, _ := setupTestServer(t)

	instanceUID := []byte("test-agent-config-persist")
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

	msg := &protobufs.AgentToServer{
		InstanceUid:     instanceUID,
		EffectiveConfig: config,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := server.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify config was persisted to storage
	stored, err := configStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(config, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsRemoteConfigStatus(t *testing.T) {
	server, _, _, statusStore := setupTestServer(t)

	instanceUID := []byte("test-agent-status-persist")
	status := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("config-hash-123"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}

	msg := &protobufs.AgentToServer{
		InstanceUid:        instanceUID,
		RemoteConfigStatus: status,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := server.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify status was persisted to storage
	stored, err := statusStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(status, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsAllFields(t *testing.T) {
	server, healthStore, configStore, statusStore := setupTestServer(t)

	instanceUID := []byte("test-agent-all-fields")
	health := &protobufs.ComponentHealth{
		Healthy: true,
		Status:  "running",
	}
	config := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {Body: []byte("key: value")},
			},
		},
	}
	status := &protobufs.RemoteConfigStatus{
		Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}

	msg := &protobufs.AgentToServer{
		InstanceUid:        instanceUID,
		Health:             health,
		EffectiveConfig:    config,
		RemoteConfigStatus: status,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	resp := server.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify all fields were persisted
	storedHealth, err := healthStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.True(t, storedHealth.Healthy)

	storedConfig, err := configStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.NotNil(t, storedConfig.ConfigMap)

	storedStatus, err := statusStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, storedStatus.Status)
}

func TestServer_OnMessage_OnlyPersistsNonNilFields(t *testing.T) {
	server, healthStore, configStore, statusStore := setupTestServer(t)

	instanceUID := []byte("test-agent-partial")
	ctx := context.Background()

	// Message with only health
	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		Health: &protobufs.ComponentHealth{
			Healthy: true,
			Status:  "running",
		},
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	resp := server.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Health should be persisted
	_, err := healthStore.Get(ctx, string(instanceUID))
	require.NoError(t, err)

	// Config and status should not exist
	_, err = configStore.Get(ctx, string(instanceUID))
	require.Error(t, err)

	_, err = statusStore.Get(ctx, string(instanceUID))
	require.Error(t, err)
}

// Mock connection for tests
type testMockConnection struct {
	instanceUID []byte
}

func (m *testMockConnection) Connection() net.Conn {
	return &testMockNetConn{}
}

func (m *testMockConnection) Send(ctx context.Context, msg *protobufs.ServerToAgent) error {
	return nil
}

func (m *testMockConnection) Disconnect() error {
	return nil
}

type testMockNetConn struct{}

func (m *testMockNetConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *testMockNetConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *testMockNetConn) Close() error                       { return nil }
func (m *testMockNetConn) LocalAddr() net.Addr                { return &testMockAddr{} }
func (m *testMockNetConn) RemoteAddr() net.Addr               { return &testMockAddr{addr: "127.0.0.1:55555"} }
func (m *testMockNetConn) SetDeadline(t time.Time) error      { return nil }
func (m *testMockNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *testMockNetConn) SetWriteDeadline(t time.Time) error { return nil }

type testMockAddr struct {
	addr string
}

func (m *testMockAddr) Network() string { return "tcp" }
func (m *testMockAddr) String() string {
	if m.addr == "" {
		return "127.0.0.1:4320"
	}
	return m.addr
}
