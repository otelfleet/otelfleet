package opamp_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *opamp.Server {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default()

	return opamp.NewServer(
		logger,
		nil,
		opamp.NewAgentTracker(),
		storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("agent-health")),
		storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("agent-effective-config")),
		storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("agent-remote-config-status")),
	)
}

func TestServer_SequenceNumTracking_Sequential(t *testing.T) {
	server := newTestServer(t)

	instanceUID := []byte("test-agent-seq")
	conn := &seqMockConnection{instanceUID: instanceUID}

	// First message (seq 0)
	msg1 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 0,
	}
	resp1 := server.OnMessage(context.Background(), conn, msg1)
	require.NotNil(t, resp1)
	assert.Equal(t, uint64(0), resp1.Flags, "First message should not request full state")

	// Second message (seq 1) - sequential
	msg2 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 1,
	}
	resp2 := server.OnMessage(context.Background(), conn, msg2)
	require.NotNil(t, resp2)
	assert.Equal(t, uint64(0), resp2.Flags, "Sequential message should not request full state")

	// Third message (seq 2) - sequential
	msg3 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 2,
	}
	resp3 := server.OnMessage(context.Background(), conn, msg3)
	require.NotNil(t, resp3)
	assert.Equal(t, uint64(0), resp3.Flags, "Sequential message should not request full state")
}

func TestServer_SequenceNumTracking_Gap(t *testing.T) {
	server := newTestServer(t)

	instanceUID := []byte("test-agent-gap")
	conn := &seqMockConnection{instanceUID: instanceUID}

	// First message (seq 0)
	msg1 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 0,
	}
	resp1 := server.OnMessage(context.Background(), conn, msg1)
	require.NotNil(t, resp1)

	// Second message (seq 1)
	msg2 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 1,
	}
	resp2 := server.OnMessage(context.Background(), conn, msg2)
	require.NotNil(t, resp2)

	// Skip to seq 5 (gap)
	msg3 := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 5,
	}
	resp3 := server.OnMessage(context.Background(), conn, msg3)
	require.NotNil(t, resp3)

	// Should request full state
	expectedFlag := uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
	assert.Equal(t, expectedFlag, resp3.Flags, "Gap in sequence should request full state")
}

func TestServer_SequenceNumTracking_NewAgent(t *testing.T) {
	server := newTestServer(t)

	instanceUID := []byte("test-new-agent")
	conn := &seqMockConnection{instanceUID: instanceUID}

	// First message from new agent starting with seq 0
	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 0,
		AgentDescription: &protobufs.AgentDescription{
			IdentifyingAttributes: []*protobufs.KeyValue{
				{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "test"}}},
			},
		},
	}
	resp := server.OnMessage(context.Background(), conn, msg)
	require.NotNil(t, resp)

	// New agent should not need full state if starting from 0
	assert.Equal(t, uint64(0), resp.Flags, "New agent starting at seq 0 should not need full state")
}

func TestServer_SequenceNumTracking_ResponseContainsInstanceUID(t *testing.T) {
	server := newTestServer(t)

	instanceUID := []byte("test-agent-uid")
	conn := &seqMockConnection{instanceUID: instanceUID}

	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 0,
	}
	resp := server.OnMessage(context.Background(), conn, msg)
	require.NotNil(t, resp)
	assert.Equal(t, instanceUID, resp.InstanceUid, "Response should contain the agent's instance UID")
}

func TestServer_SequenceNumTracking_MultipleAgents(t *testing.T) {
	server := newTestServer(t)

	agent1 := []byte("agent-1")
	agent2 := []byte("agent-2")
	conn1 := &seqMockConnection{instanceUID: agent1}
	conn2 := &seqMockConnection{instanceUID: agent2}

	// Agent 1 sends seq 0, 1, 2
	for seq := range uint64(3) {
		msg := &protobufs.AgentToServer{
			InstanceUid: agent1,
			SequenceNum: seq,
		}
		resp := server.OnMessage(context.Background(), conn1, msg)
		assert.Equal(t, uint64(0), resp.Flags)
	}

	// Agent 2 sends seq 0, 1
	for seq := range uint64(2) {
		msg := &protobufs.AgentToServer{
			InstanceUid: agent2,
			SequenceNum: seq,
		}
		resp := server.OnMessage(context.Background(), conn2, msg)
		assert.Equal(t, uint64(0), resp.Flags)
	}

	// Agent 1 skips to seq 10 (gap)
	msg := &protobufs.AgentToServer{
		InstanceUid: agent1,
		SequenceNum: 10,
	}
	resp := server.OnMessage(context.Background(), conn1, msg)
	expectedFlag := uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
	assert.Equal(t, expectedFlag, resp.Flags, "Agent 1 should request full state due to gap")

	// Agent 2 continues normally (seq 2)
	msg2 := &protobufs.AgentToServer{
		InstanceUid: agent2,
		SequenceNum: 2,
	}
	resp2 := server.OnMessage(context.Background(), conn2, msg2)
	assert.Equal(t, uint64(0), resp2.Flags, "Agent 2 should not need full state")
}

// Mock connection for sequence tests
type seqMockConnection struct {
	instanceUID []byte
}

func (m *seqMockConnection) Connection() net.Conn {
	return &seqMockNetConn{}
}

func (m *seqMockConnection) Send(ctx context.Context, msg *protobufs.ServerToAgent) error {
	return nil
}

func (m *seqMockConnection) Disconnect() error {
	return nil
}

type seqMockNetConn struct{}

func (m *seqMockNetConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *seqMockNetConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *seqMockNetConn) Close() error                       { return nil }
func (m *seqMockNetConn) LocalAddr() net.Addr                { return &seqMockAddr{} }
func (m *seqMockNetConn) RemoteAddr() net.Addr               { return &seqMockAddr{addr: "127.0.0.1:11111"} }
func (m *seqMockNetConn) SetDeadline(t time.Time) error      { return nil }
func (m *seqMockNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *seqMockNetConn) SetWriteDeadline(t time.Time) error { return nil }

type seqMockAddr struct {
	addr string
}

func (m *seqMockAddr) Network() string { return "tcp" }
func (m *seqMockAddr) String() string {
	if m.addr == "" {
		return "127.0.0.1:4320"
	}
	return m.addr
}
