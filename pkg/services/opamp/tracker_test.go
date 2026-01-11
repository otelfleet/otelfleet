package opamp_test

import (
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTracker_GetFullState(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	instanceUID := "test-agent-123"

	// Initially should return nil
	state, exists := tracker.GetFullState(instanceUID)
	assert.False(t, exists)
	assert.Nil(t, state)
}

func TestAgentTracker_PutFullState(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	instanceUID := "test-agent-123"

	state := &opamp.AgentFullState{
		InstanceUID: []byte(instanceUID),
		LastSeen:    time.Now(),
		Health: &protobufs.ComponentHealth{
			Healthy: true,
			Status:  "running",
		},
	}

	tracker.PutFullState(instanceUID, state)

	retrieved, exists := tracker.GetFullState(instanceUID)
	require.True(t, exists)
	require.NotNil(t, retrieved)
	assert.Equal(t, []byte(instanceUID), retrieved.InstanceUID)
	assert.True(t, retrieved.Health.Healthy)
}

func TestAgentTracker_UpdateFromMessage(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	instanceUID := "test-agent-123"

	msg := &protobufs.AgentToServer{
		InstanceUid:  []byte(instanceUID),
		SequenceNum:  1,
		Capabilities: uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus),
		AgentDescription: &protobufs.AgentDescription{
			IdentifyingAttributes: []*protobufs.KeyValue{
				{
					Key:   "service.name",
					Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "test-service"}},
				},
			},
		},
		Health: &protobufs.ComponentHealth{
			Healthy:           true,
			StartTimeUnixNano: uint64(time.Now().UnixNano()),
			Status:            "running",
		},
		EffectiveConfig: &protobufs.EffectiveConfig{
			ConfigMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"config.yaml": {Body: []byte("key: value")},
				},
			},
		},
		RemoteConfigStatus: &protobufs.RemoteConfigStatus{
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			LastRemoteConfigHash: []byte("hash"),
		},
	}

	needsFullState := tracker.UpdateFromMessage(instanceUID, msg)

	// First message should not need full state if sequence is 0 or 1
	assert.False(t, needsFullState)

	state, exists := tracker.GetFullState(instanceUID)
	require.True(t, exists)
	require.NotNil(t, state)

	// Verify all fields were updated
	assert.NotNil(t, state.Description)
	assert.NotNil(t, state.Health)
	assert.NotNil(t, state.EffectiveConfig)
	assert.NotNil(t, state.RemoteConfigStatus)
	assert.Equal(t, uint64(1), state.SequenceNum)
}

func TestAgentTracker_SequenceNumTracking(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	instanceUID := "test-agent-123"

	// First message with sequence 0
	msg1 := &protobufs.AgentToServer{
		InstanceUid: []byte(instanceUID),
		SequenceNum: 0,
	}
	needsFullState := tracker.UpdateFromMessage(instanceUID, msg1)
	assert.False(t, needsFullState, "First message should not need full state")

	// Sequential message (seq 1) - should not need full state
	msg2 := &protobufs.AgentToServer{
		InstanceUid: []byte(instanceUID),
		SequenceNum: 1,
	}
	needsFullState = tracker.UpdateFromMessage(instanceUID, msg2)
	assert.False(t, needsFullState, "Sequential message should not need full state")

	// Skip a sequence (seq 5) - should need full state
	msg3 := &protobufs.AgentToServer{
		InstanceUid: []byte(instanceUID),
		SequenceNum: 5,
	}
	needsFullState = tracker.UpdateFromMessage(instanceUID, msg3)
	assert.True(t, needsFullState, "Non-sequential message should need full state")
}

func TestAgentTracker_Delete(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	instanceUID := "test-agent-123"

	state := &opamp.AgentFullState{
		InstanceUID: []byte(instanceUID),
		LastSeen:    time.Now(),
	}
	tracker.PutFullState(instanceUID, state)

	// Verify it exists
	_, exists := tracker.GetFullState(instanceUID)
	require.True(t, exists)

	// Delete it
	tracker.Delete(instanceUID)

	// Verify it's gone
	_, exists = tracker.GetFullState(instanceUID)
	assert.False(t, exists)
}

func TestAgentTracker_ConcurrentAccess(t *testing.T) {
	tracker := opamp.NewAgentTracker()
	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			tracker.PutFullState("agent-1", &opamp.AgentFullState{
				InstanceUID: []byte("agent-1"),
				LastSeen:    time.Now(),
			})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			tracker.GetFullState("agent-1")
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
}
