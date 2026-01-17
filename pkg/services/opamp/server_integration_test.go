//go:build insecure

package opamp_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

// makeAgentDescription creates an AgentDescription with the required otelfleet.agent.id attribute
func makeAgentDescription(agentID string) *protobufs.AgentDescription {
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key:   supervisor.AttributeOtelfleetAgentId,
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: agentID}},
			},
		},
	}
}

func TestServer_OnMessage_PersistsHealth(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-health-persist"
	instanceUID := []byte(agentID)
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
		InstanceUid:      instanceUID,
		AgentDescription: makeAgentDescription(agentID),
		Health:           health,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := env.OpampServer.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify health was persisted to storage
	stored, err := env.HealthStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(health, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsEffectiveConfig(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-config-persist"
	instanceUID := []byte(agentID)
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
		InstanceUid:      instanceUID,
		AgentDescription: makeAgentDescription(agentID),
		EffectiveConfig:  config,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := env.OpampServer.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify config was persisted to storage
	stored, err := env.EffectiveConfigStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(config, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsRemoteConfigStatus(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-status-persist"
	instanceUID := []byte(agentID)
	status := &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("config-hash-123"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	}

	msg := &protobufs.AgentToServer{
		InstanceUid:        instanceUID,
		AgentDescription:   makeAgentDescription(agentID),
		RemoteConfigStatus: status,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	// Process message
	resp := env.OpampServer.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify status was persisted to storage
	stored, err := env.RemoteStatusStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(status, stored, protocmp.Transform()))
}

func TestServer_OnMessage_PersistsAllFields(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-all-fields"
	instanceUID := []byte(agentID)
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
		AgentDescription:   makeAgentDescription(agentID),
		Health:             health,
		EffectiveConfig:    config,
		RemoteConfigStatus: status,
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	ctx := context.Background()

	resp := env.OpampServer.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Verify all fields were persisted
	storedHealth, err := env.HealthStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.True(t, storedHealth.Healthy)

	storedConfig, err := env.EffectiveConfigStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.NotNil(t, storedConfig.ConfigMap)

	storedStatus, err := env.RemoteStatusStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, storedStatus.Status)
}

func TestServer_OnMessage_OnlyPersistsNonNilFields(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-partial"
	instanceUID := []byte(agentID)
	ctx := context.Background()

	// Message with only health (and required AgentDescription)
	msg := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: makeAgentDescription(agentID),
		Health: &protobufs.ComponentHealth{
			Healthy: true,
			Status:  "running",
		},
	}

	conn := &testMockConnection{instanceUID: instanceUID}
	resp := env.OpampServer.OnMessage(ctx, conn, msg)
	require.NotNil(t, resp)

	// Health should be persisted
	_, err := env.HealthStore.Get(ctx, agentID)
	require.NoError(t, err)

	// Config and status should not exist
	_, err = env.EffectiveConfigStore.Get(ctx, agentID)
	require.Error(t, err)

	_, err = env.RemoteStatusStore.Get(ctx, agentID)
	require.Error(t, err)
}

// Mock connection for tests - needed to call OnMessage directly
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
