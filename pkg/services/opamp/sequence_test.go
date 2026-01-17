//go:build insecure

package opamp_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeSeqAgentDescription creates an AgentDescription with the required otelfleet.agent.id attribute
func makeSeqAgentDescription(agentID string) *protobufs.AgentDescription {
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key:   supervisor.AttributeOtelfleetAgentId,
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: agentID}},
			},
		},
	}
}

func TestServer_SequenceNumTracking_Sequential(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-seq"
	instanceUID := []byte(agentID)
	conn := &seqMockConnection{instanceUID: instanceUID}
	desc := makeSeqAgentDescription(agentID)

	// First message (seq 0)
	msg1 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      0,
	}
	resp1 := env.OpampServer.OnMessage(context.Background(), conn, msg1)
	require.NotNil(t, resp1)
	assert.Equal(t, uint64(0), resp1.Flags, "First message should not request full state")

	// Second message (seq 1) - sequential
	msg2 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      1,
	}
	resp2 := env.OpampServer.OnMessage(context.Background(), conn, msg2)
	require.NotNil(t, resp2)
	assert.Equal(t, uint64(0), resp2.Flags, "Sequential message should not request full state")

	// Third message (seq 2) - sequential
	msg3 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      2,
	}
	resp3 := env.OpampServer.OnMessage(context.Background(), conn, msg3)
	require.NotNil(t, resp3)
	assert.Equal(t, uint64(0), resp3.Flags, "Sequential message should not request full state")
}

func TestServer_SequenceNumTracking_Gap(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-gap"
	instanceUID := []byte(agentID)
	conn := &seqMockConnection{instanceUID: instanceUID}
	desc := makeSeqAgentDescription(agentID)

	// First message (seq 0)
	msg1 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      0,
	}
	resp1 := env.OpampServer.OnMessage(context.Background(), conn, msg1)
	require.NotNil(t, resp1)

	// Second message (seq 1)
	msg2 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      1,
	}
	resp2 := env.OpampServer.OnMessage(context.Background(), conn, msg2)
	require.NotNil(t, resp2)

	// Skip to seq 5 (gap)
	msg3 := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: desc,
		SequenceNum:      5,
	}
	resp3 := env.OpampServer.OnMessage(context.Background(), conn, msg3)
	require.NotNil(t, resp3)

	// Should request full state
	expectedFlag := uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
	assert.Equal(t, expectedFlag, resp3.Flags, "Gap in sequence should request full state")
}

func TestServer_SequenceNumTracking_NewAgent(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-new-agent"
	instanceUID := []byte(agentID)
	conn := &seqMockConnection{instanceUID: instanceUID}

	// First message from new agent starting with seq 0
	// Include both the otelfleet.agent.id and the service.name
	msg := &protobufs.AgentToServer{
		InstanceUid: instanceUID,
		SequenceNum: 0,
		AgentDescription: &protobufs.AgentDescription{
			IdentifyingAttributes: []*protobufs.KeyValue{
				{Key: supervisor.AttributeOtelfleetAgentId, Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: agentID}}},
				{Key: "service.name", Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "test"}}},
			},
		},
	}
	resp := env.OpampServer.OnMessage(context.Background(), conn, msg)
	require.NotNil(t, resp)

	// New agent should not need full state if starting from 0
	assert.Equal(t, uint64(0), resp.Flags, "New agent starting at seq 0 should not need full state")
}

func TestServer_SequenceNumTracking_ResponseContainsInstanceUID(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID := "test-agent-uid"
	instanceUID := []byte(agentID)
	conn := &seqMockConnection{instanceUID: instanceUID}

	msg := &protobufs.AgentToServer{
		InstanceUid:      instanceUID,
		AgentDescription: makeSeqAgentDescription(agentID),
		SequenceNum:      0,
	}
	resp := env.OpampServer.OnMessage(context.Background(), conn, msg)
	require.NotNil(t, resp)
	assert.Equal(t, instanceUID, resp.InstanceUid, "Response should contain the agent's instance UID")
}

func TestServer_SequenceNumTracking_MultipleAgents(t *testing.T) {
	env := testutil.NewTestEnv(t)

	agentID1 := "agent-1"
	agentID2 := "agent-2"
	agent1 := []byte(agentID1)
	agent2 := []byte(agentID2)
	conn1 := &seqMockConnection{instanceUID: agent1}
	conn2 := &seqMockConnection{instanceUID: agent2}
	desc1 := makeSeqAgentDescription(agentID1)
	desc2 := makeSeqAgentDescription(agentID2)

	// Agent 1 sends seq 0, 1, 2
	for seq := range uint64(3) {
		msg := &protobufs.AgentToServer{
			InstanceUid:      agent1,
			AgentDescription: desc1,
			SequenceNum:      seq,
		}
		resp := env.OpampServer.OnMessage(context.Background(), conn1, msg)
		assert.Equal(t, uint64(0), resp.Flags)
	}

	// Agent 2 sends seq 0, 1
	for seq := range uint64(2) {
		msg := &protobufs.AgentToServer{
			InstanceUid:      agent2,
			AgentDescription: desc2,
			SequenceNum:      seq,
		}
		resp := env.OpampServer.OnMessage(context.Background(), conn2, msg)
		assert.Equal(t, uint64(0), resp.Flags)
	}

	// Agent 1 skips to seq 10 (gap)
	msg := &protobufs.AgentToServer{
		InstanceUid:      agent1,
		AgentDescription: desc1,
		SequenceNum:      10,
	}
	resp := env.OpampServer.OnMessage(context.Background(), conn1, msg)
	expectedFlag := uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
	assert.Equal(t, expectedFlag, resp.Flags, "Agent 1 should request full state due to gap")

	// Agent 2 continues normally (seq 2)
	msg2 := &protobufs.AgentToServer{
		InstanceUid:      agent2,
		AgentDescription: desc2,
		SequenceNum:      2,
	}
	resp2 := env.OpampServer.OnMessage(context.Background(), conn2, msg2)
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
