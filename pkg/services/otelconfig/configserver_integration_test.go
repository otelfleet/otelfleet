package otelconfig_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockNotifier tracks config change notifications for testing
type mockNotifier struct {
	mu            sync.Mutex
	notifications []string
}

func (m *mockNotifier) NotifyConfigChange(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, agentID)
}

func (m *mockNotifier) getNotifications() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func (m *mockNotifier) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = nil
}

// testEnv provides all stores needed for ConfigServer testing
type testEnv struct {
	server                *otelconfig.ConfigServer
	configStore           storage.KeyValue[*v1alpha1.Config]
	defaultConfigStore    storage.KeyValue[*v1alpha1.Config]
	assignedConfigStore   storage.KeyValue[*v1alpha1.Config]
	configAssignmentStore storage.KeyValue[*v1alpha1.ConfigAssignment]
	agentStore            storage.KeyValue[*agentsv1alpha1.AgentDescription]
	effectiveConfigStore  storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore     storage.KeyValue[*protobufs.RemoteConfigStatus]
	notifier              *mockNotifier
}

func setupTestEnv(t *testing.T) *testEnv {
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

	h := &testEnv{
		configStore:           storage.NewProtoKV[*v1alpha1.Config](logger, broker.KeyValue("configs")),
		defaultConfigStore:    storage.NewProtoKV[*v1alpha1.Config](logger, broker.KeyValue("default-configs")),
		assignedConfigStore:   storage.NewProtoKV[*v1alpha1.Config](logger, broker.KeyValue("assigned-configs")),
		configAssignmentStore: storage.NewProtoKV[*v1alpha1.ConfigAssignment](logger, broker.KeyValue("config-assignments")),
		agentStore:            storage.NewProtoKV[*agentsv1alpha1.AgentDescription](logger, broker.KeyValue("agents")),
		effectiveConfigStore:  storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("effective-configs")),
		remoteStatusStore:     storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("remote-status")),
		notifier:              &mockNotifier{},
	}

	h.server = otelconfig.NewConfigServer(
		logger,
		h.configStore,
		h.defaultConfigStore,
		h.assignedConfigStore,
		h.configAssignmentStore,
		h.agentStore,
		h.effectiveConfigStore,
		h.remoteStatusStore,
	)
	h.server.SetNotifier(h.notifier)

	return h
}

// createTestAgent creates an agent in the store
func (h *testEnv) createTestAgent(ctx context.Context, t *testing.T, agentID string, labels map[string]string) {
	t.Helper()
	var attrs []*agentsv1alpha1.KeyValue
	for k, v := range labels {
		attrs = append(attrs, &agentsv1alpha1.KeyValue{
			Key:   k,
			Value: &agentsv1alpha1.AnyValue{Value: &agentsv1alpha1.AnyValue_StringValue{StringValue: v}},
		})
	}
	agent := &agentsv1alpha1.AgentDescription{
		Id:                       agentID,
		IdentifyingAttributes:    attrs,
		NonIdentifyingAttributes: []*agentsv1alpha1.KeyValue{},
	}
	require.NoError(t, h.agentStore.Put(ctx, agentID, agent))
}

// createTestConfig creates a config in the store
func (h *testEnv) createTestConfig(ctx context.Context, t *testing.T, configID string, configYAML string) *v1alpha1.Config {
	t.Helper()
	config := &v1alpha1.Config{
		Config: []byte(configYAML),
	}
	require.NoError(t, h.configStore.Put(ctx, configID, config))
	return config
}

// ============================================================================
// Test: Hash Consistency Between Assignment and Status Checking
// ============================================================================

// TestHashConsistency_AssignedHashMatchesStatusCheck verifies that the hash
// computed during assignment is the same hash used when checking config status.
// This catches bugs where different hash computations are used in different places.
func TestHashConsistency_AssignedHashMatchesStatusCheck(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	// Setup
	agentID := "agent-hash-test"
	configID := "config-hash-test"
	configYAML := "receivers:\n  otlp:\n    protocols:\n      grpc:\n"

	h.createTestAgent(ctx, t, agentID, nil)
	config := h.createTestConfig(ctx, t, configID, configYAML)

	// Assign config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Get the stored assignment to find the hash
	assignment, err := h.configAssignmentStore.Get(ctx, agentID)
	require.NoError(t, err)
	assignedHash := assignment.GetConfigHash()

	// Compute the expected hash using the same method
	expectedHash := util.HashAgentConfigMap(util.ProtoConfigToAgentConfigMap(config))

	// Verify hash consistency
	assert.Equal(t, expectedHash, assignedHash,
		"Hash computed during assignment should match expected hash computation")

	// Now simulate agent reporting config applied with the same hash
	err = h.remoteStatusStore.Put(ctx, agentID, &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: assignedHash,
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	})
	require.NoError(t, err)

	// Get status - it should show APPLIED because hashes match
	statusResp, err := h.server.GetConfigStatus(ctx, connect.NewRequest(&v1alpha1.GetConfigStatusRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_APPLIED,
		statusResp.Msg.Assignment.GetStatus(),
		"Status should be APPLIED when agent reports matching hash")
}

// TestHashConsistency_MismatchedHashShowsPending verifies that when the agent
// reports a different hash, status shows PENDING (not applied).
func TestHashConsistency_MismatchedHashShowsPending(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-mismatch-test"
	configID := "config-mismatch-test"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	// Assign config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Agent reports a DIFFERENT hash (simulating old config still running)
	err = h.remoteStatusStore.Put(ctx, agentID, &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: []byte("different-hash"),
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	})
	require.NoError(t, err)

	// Get status - should show PENDING because hash doesn't match
	statusResp, err := h.server.GetConfigStatus(ctx, connect.NewRequest(&v1alpha1.GetConfigStatusRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_PENDING,
		statusResp.Msg.Assignment.GetStatus(),
		"Status should be PENDING when agent reports different hash")
}

// ============================================================================
// Test: Store Consistency Between assignedConfigStore and configAssignmentStore
// ============================================================================

// TestStoreConsistency_BothStoresUpdatedOnAssign verifies that both stores
// are updated atomically when assigning a config.
func TestStoreConsistency_BothStoresUpdatedOnAssign(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-store-test"
	configID := "config-store-test"
	configYAML := "exporters:\n  debug:\n"

	h.createTestAgent(ctx, t, agentID, nil)
	config := h.createTestConfig(ctx, t, configID, configYAML)

	// Assign config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Verify assignedConfigStore has the config
	storedConfig, err := h.assignedConfigStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, config.GetConfig(), storedConfig.GetConfig(),
		"assignedConfigStore should have the config bytes")

	// Verify configAssignmentStore has the metadata
	assignment, err := h.configAssignmentStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, agentID, assignment.GetAgentId())
	assert.Equal(t, configID, assignment.GetConfigId())
	assert.Equal(t, v1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL, assignment.GetSource())
	assert.NotNil(t, assignment.GetAssignedAt())
	assert.NotEmpty(t, assignment.GetConfigHash())
}

// TestStoreConsistency_BothStoresUpdatedOnUnassign verifies that both stores
// are cleaned up when unassigning a config.
func TestStoreConsistency_BothStoresUpdatedOnUnassign(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-unassign-test"
	configID := "config-unassign-test"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	// Assign then unassign
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	_, err = h.server.UnassignConfig(ctx, connect.NewRequest(&v1alpha1.UnassignConfigRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)

	// Verify assignedConfigStore is empty
	_, err = h.assignedConfigStore.Get(ctx, agentID)
	assert.Error(t, err, "assignedConfigStore should be empty after unassign")

	// Verify configAssignmentStore is empty
	_, err = h.configAssignmentStore.Get(ctx, agentID)
	assert.Error(t, err, "configAssignmentStore should be empty after unassign")
}

// TestStoreConsistency_ReassignOverwritesPrevious verifies that reassigning
// a different config correctly updates both stores.
func TestStoreConsistency_ReassignOverwritesPrevious(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-reassign-test"
	configID1 := "config-v1"
	configID2 := "config-v2"
	configYAML1 := "version: 1"
	configYAML2 := "version: 2"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID1, configYAML1)
	config2 := h.createTestConfig(ctx, t, configID2, configYAML2)

	// Assign first config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID1,
	}))
	require.NoError(t, err)

	// Get first assignment hash
	assignment1, err := h.configAssignmentStore.Get(ctx, agentID)
	require.NoError(t, err)
	hash1 := assignment1.GetConfigHash()

	// Assign second config
	_, err = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID2,
	}))
	require.NoError(t, err)

	// Verify stores have new config
	storedConfig, err := h.assignedConfigStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, config2.GetConfig(), storedConfig.GetConfig(),
		"assignedConfigStore should have the new config")

	assignment2, err := h.configAssignmentStore.Get(ctx, agentID)
	require.NoError(t, err)
	assert.Equal(t, configID2, assignment2.GetConfigId(),
		"configAssignmentStore should reference new config")
	assert.NotEqual(t, hash1, assignment2.GetConfigHash(),
		"Hash should change when config changes")
}

// ============================================================================
// Test: Notification Consistency
// ============================================================================

// TestNotification_FiredOnAssign verifies notifications are sent when assigning.
func TestNotification_FiredOnAssign(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-notify-assign"
	configID := "config-notify-assign"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	h.notifier.reset()

	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	notifications := h.notifier.getNotifications()
	assert.Contains(t, notifications, agentID,
		"Notification should be fired on config assignment")
}

// TestNotification_FiredOnUnassign verifies notifications are sent when unassigning.
func TestNotification_FiredOnUnassign(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-notify-unassign"
	configID := "config-notify-unassign"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	h.notifier.reset()

	_, err = h.server.UnassignConfig(ctx, connect.NewRequest(&v1alpha1.UnassignConfigRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)

	notifications := h.notifier.getNotifications()
	assert.Contains(t, notifications, agentID,
		"Notification should be fired on config unassignment")
}

// TestNotification_FiredForEachAgentInBatch verifies batch operations notify all agents.
func TestNotification_FiredForEachAgentInBatch(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agents := []string{"batch-agent-1", "batch-agent-2", "batch-agent-3"}
	configID := "config-batch"

	for _, agentID := range agents {
		h.createTestAgent(ctx, t, agentID, nil)
	}
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	h.notifier.reset()

	_, err := h.server.BatchAssignConfig(ctx, connect.NewRequest(&v1alpha1.BatchAssignConfigRequest{
		AgentIds: agents,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	notifications := h.notifier.getNotifications()
	for _, agentID := range agents {
		assert.Contains(t, notifications, agentID,
			"Each agent in batch should receive notification")
	}
}

// ============================================================================
// Test: InSync Status Consistency
// ============================================================================

// TestInSync_TrueWhenEffectiveMatchesAssigned verifies InSync is true when
// agent's effective config hash matches the assigned hash.
func TestInSync_TrueWhenEffectiveMatchesAssigned(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-insync-test"
	configID := "config-insync-test"
	configYAML := "receivers:\n  otlp:\n    protocols:\n      grpc:\n"

	h.createTestAgent(ctx, t, agentID, nil)
	config := h.createTestConfig(ctx, t, configID, configYAML)

	// Assign config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Simulate agent reporting the exact same config as effective
	agentConfigMap := util.ProtoConfigToAgentConfigMap(config)
	err = h.effectiveConfigStore.Put(ctx, agentID, &protobufs.EffectiveConfig{
		ConfigMap: agentConfigMap,
	})
	require.NoError(t, err)

	// Also set remote status as applied with matching hash
	expectedHash := util.HashAgentConfigMap(agentConfigMap)
	err = h.remoteStatusStore.Put(ctx, agentID, &protobufs.RemoteConfigStatus{
		LastRemoteConfigHash: expectedHash,
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
	})
	require.NoError(t, err)

	// Get status
	statusResp, err := h.server.GetConfigStatus(ctx, connect.NewRequest(&v1alpha1.GetConfigStatusRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)

	assert.True(t, statusResp.Msg.GetInSync(),
		"InSync should be true when effective config matches assigned")
}

// TestInSync_FalseWhenEffectiveDiffers verifies InSync is false when
// agent's effective config is different from assigned.
func TestInSync_FalseWhenEffectiveDiffers(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	agentID := "agent-outofsync-test"
	configID := "config-outofsync-test"

	h.createTestAgent(ctx, t, agentID, nil)
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	// Assign config
	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  agentID,
		ConfigId: configID,
	}))
	require.NoError(t, err)

	// Simulate agent reporting a DIFFERENT effective config
	err = h.effectiveConfigStore.Put(ctx, agentID, &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"config.yaml": {
					Body:        []byte("completely: different"),
					ContentType: "text/yaml",
				},
			},
		},
	})
	require.NoError(t, err)

	// Get status
	statusResp, err := h.server.GetConfigStatus(ctx, connect.NewRequest(&v1alpha1.GetConfigStatusRequest{
		AgentId: agentID,
	}))
	require.NoError(t, err)

	assert.False(t, statusResp.Msg.GetInSync(),
		"InSync should be false when effective config differs from assigned")
}

// ============================================================================
// Test: Label Matching Consistency
// ============================================================================

// TestLabelMatching_AllLabelsRequired verifies that ALL specified labels must
// match (AND semantics, not OR).
func TestLabelMatching_AllLabelsRequired(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "config-labels"
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	// Agent with both labels
	h.createTestAgent(ctx, t, "agent-both", map[string]string{
		"env":     "prod",
		"region":  "us-east",
		"service": "api",
	})

	// Agent with only one label
	h.createTestAgent(ctx, t, "agent-partial", map[string]string{
		"env": "prod",
	})

	// Agent with no matching labels
	h.createTestAgent(ctx, t, "agent-none", map[string]string{
		"env":    "staging",
		"region": "eu-west",
	})

	// Assign by labels requiring BOTH env=prod AND region=us-east
	resp, err := h.server.AssignConfigByLabels(ctx, connect.NewRequest(&v1alpha1.AssignConfigByLabelsRequest{
		ConfigId: configID,
		Labels: map[string]string{
			"env":    "prod",
			"region": "us-east",
		},
	}))
	require.NoError(t, err)

	// Only agent-both should match
	assert.Equal(t, int32(1), resp.Msg.GetSuccessful())
	assert.Contains(t, resp.Msg.GetMatchedAgentIds(), "agent-both")
	assert.NotContains(t, resp.Msg.GetMatchedAgentIds(), "agent-partial")
	assert.NotContains(t, resp.Msg.GetMatchedAgentIds(), "agent-none")
}

// TestLabelMatching_EmptyLabelsMatchNone verifies that empty labels don't
// accidentally match all agents.
func TestLabelMatching_EmptyLabelsReturnsError(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "config-empty-labels"
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")
	h.createTestAgent(ctx, t, "agent-any", map[string]string{"env": "prod"})

	// Empty labels should return error
	_, err := h.server.AssignConfigByLabels(ctx, connect.NewRequest(&v1alpha1.AssignConfigByLabelsRequest{
		ConfigId: configID,
		Labels:   map[string]string{},
	}))
	assert.Error(t, err, "Empty labels should return an error")
}

// ============================================================================
// Test: List Operations Consistency
// ============================================================================

// TestListConfigAssignments_FilterByConfigId verifies filtering works correctly.
func TestListConfigAssignments_FilterByConfigId(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	config1 := "shared-config-1"
	config2 := "shared-config-2"

	h.createTestConfig(ctx, t, config1, "version: 1")
	h.createTestConfig(ctx, t, config2, "version: 2")

	h.createTestAgent(ctx, t, "filter-agent-1", nil)
	h.createTestAgent(ctx, t, "filter-agent-2", nil)
	h.createTestAgent(ctx, t, "filter-agent-3", nil)

	// Assign config1 to agents 1 and 2
	_, _ = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId: "filter-agent-1", ConfigId: config1,
	}))
	_, _ = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId: "filter-agent-2", ConfigId: config1,
	}))
	// Assign config2 to agent 3
	_, _ = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId: "filter-agent-3", ConfigId: config2,
	}))

	// Filter by config1
	resp, err := h.server.ListConfigAssignments(ctx, connect.NewRequest(&v1alpha1.ListConfigAssignmentsRequest{
		ConfigId: &config1,
	}))
	require.NoError(t, err)

	assert.Len(t, resp.Msg.GetAssignments(), 2)
	for _, assignment := range resp.Msg.GetAssignments() {
		assert.Equal(t, config1, assignment.GetConfigId())
	}
}

// TestListConfigAssignments_NoFilterReturnsAll verifies unfiltered list returns all.
func TestListConfigAssignments_NoFilterReturnsAll(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	config1 := "all-config-1"
	config2 := "all-config-2"

	h.createTestConfig(ctx, t, config1, "version: 1")
	h.createTestConfig(ctx, t, config2, "version: 2")

	h.createTestAgent(ctx, t, "all-agent-1", nil)
	h.createTestAgent(ctx, t, "all-agent-2", nil)

	_, _ = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId: "all-agent-1", ConfigId: config1,
	}))
	_, _ = h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId: "all-agent-2", ConfigId: config2,
	}))

	// No filter
	resp, err := h.server.ListConfigAssignments(ctx, connect.NewRequest(&v1alpha1.ListConfigAssignmentsRequest{}))
	require.NoError(t, err)

	assert.Len(t, resp.Msg.GetAssignments(), 2)
}

// ============================================================================
// Test: Error Handling Consistency
// ============================================================================

// TestErrorHandling_AssignNonexistentConfig verifies proper error for missing config.
func TestErrorHandling_AssignNonexistentConfig(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	h.createTestAgent(ctx, t, "error-agent-1", nil)

	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  "error-agent-1",
		ConfigId: "nonexistent-config",
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestErrorHandling_AssignToNonexistentAgent verifies proper error for missing agent.
func TestErrorHandling_AssignToNonexistentAgent(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	h.createTestConfig(ctx, t, "error-config-1", "receivers:\n  otlp:\n")

	_, err := h.server.AssignConfig(ctx, connect.NewRequest(&v1alpha1.AssignConfigRequest{
		AgentId:  "nonexistent-agent",
		ConfigId: "error-config-1",
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestErrorHandling_GetConfigStatusForUnassigned verifies proper error when no assignment.
func TestErrorHandling_GetConfigStatusForUnassigned(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	h.createTestAgent(ctx, t, "unassigned-agent", nil)

	_, err := h.server.GetConfigStatus(ctx, connect.NewRequest(&v1alpha1.GetConfigStatusRequest{
		AgentId: "unassigned-agent",
	}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no config assigned")
}

// ============================================================================
// Test: Config CRUD Consistency
// ============================================================================

// TestConfigCRUD_GetReturnsWhatWasPut verifies basic CRUD consistency.
func TestConfigCRUD_GetReturnsWhatWasPut(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "crud-config"
	configYAML := "receivers:\n  otlp:\n    protocols:\n      grpc:\n        endpoint: 0.0.0.0:4317\n"

	// Put config
	_, err := h.server.PutConfig(ctx, connect.NewRequest(&v1alpha1.PutConfigRequest{
		Ref:    &v1alpha1.ConfigReference{Id: configID},
		Config: &v1alpha1.Config{Config: []byte(configYAML)},
	}))
	require.NoError(t, err)

	// Get config
	resp, err := h.server.GetConfig(ctx, connect.NewRequest(&v1alpha1.ConfigReference{
		Id: configID,
	}))
	require.NoError(t, err)

	assert.Equal(t, configYAML, string(resp.Msg.GetConfig()))
}

// TestConfigCRUD_DeletedConfigNotRetrievable verifies deletion works.
func TestConfigCRUD_DeletedConfigNotRetrievable(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "delete-config"

	// Put and delete
	_, err := h.server.PutConfig(ctx, connect.NewRequest(&v1alpha1.PutConfigRequest{
		Ref:    &v1alpha1.ConfigReference{Id: configID},
		Config: &v1alpha1.Config{Config: []byte("temp")},
	}))
	require.NoError(t, err)

	_, err = h.server.DeleteConfig(ctx, connect.NewRequest(&v1alpha1.ConfigReference{
		Id: configID,
	}))
	require.NoError(t, err)

	// Get should fail
	_, err = h.server.GetConfig(ctx, connect.NewRequest(&v1alpha1.ConfigReference{
		Id: configID,
	}))
	assert.Error(t, err)
}

// TestConfigCRUD_ListShowsAllConfigs verifies list consistency.
func TestConfigCRUD_ListShowsAllConfigs(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configs := []string{"list-config-1", "list-config-2", "list-config-3"}

	for _, id := range configs {
		_, err := h.server.PutConfig(ctx, connect.NewRequest(&v1alpha1.PutConfigRequest{
			Ref:    &v1alpha1.ConfigReference{Id: id},
			Config: &v1alpha1.Config{Config: []byte("content")},
		}))
		require.NoError(t, err)
	}

	resp, err := h.server.ListConfigs(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	ids := make([]string, len(resp.Msg.GetConfigs()))
	for i, ref := range resp.Msg.GetConfigs() {
		ids[i] = ref.GetId()
	}

	for _, expected := range configs {
		assert.Contains(t, ids, expected)
	}
}

// ============================================================================
// Test: Batch Operation Partial Failure Consistency
// ============================================================================

// TestBatchAssign_PartialFailureReportsCorrectCounts verifies that batch
// operations correctly report success/failure counts when some agents don't exist.
func TestBatchAssign_PartialFailureReportsCorrectCounts(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "partial-config"
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	// Create only some agents
	h.createTestAgent(ctx, t, "partial-agent-1", nil)
	h.createTestAgent(ctx, t, "partial-agent-3", nil)
	// partial-agent-2 intentionally NOT created

	resp, err := h.server.BatchAssignConfig(ctx, connect.NewRequest(&v1alpha1.BatchAssignConfigRequest{
		AgentIds: []string{"partial-agent-1", "partial-agent-2", "partial-agent-3"},
		ConfigId: configID,
	}))
	require.NoError(t, err)

	assert.Equal(t, int32(2), resp.Msg.GetSuccessful(), "2 agents should succeed")
	assert.Equal(t, int32(1), resp.Msg.GetFailed(), "1 agent should fail")
	assert.Contains(t, resp.Msg.GetFailedAgentIds(), "partial-agent-2")
}

// TestBatchAssign_NotificationsOnlyForSuccessful verifies that notifications
// are only sent for successfully assigned agents.
func TestBatchAssign_NotificationsOnlyForSuccessful(t *testing.T) {
	h := setupTestEnv(t)
	ctx := context.Background()

	configID := "notify-partial-config"
	h.createTestConfig(ctx, t, configID, "receivers:\n  otlp:\n")

	h.createTestAgent(ctx, t, "notify-success-1", nil)
	// notify-failure intentionally NOT created

	h.notifier.reset()

	_, err := h.server.BatchAssignConfig(ctx, connect.NewRequest(&v1alpha1.BatchAssignConfigRequest{
		AgentIds: []string{"notify-success-1", "notify-failure"},
		ConfigId: configID,
	}))
	require.NoError(t, err)

	notifications := h.notifier.getNotifications()
	assert.Contains(t, notifications, "notify-success-1",
		"Successful agent should be notified")
	assert.NotContains(t, notifications, "notify-failure",
		"Failed agent should NOT be notified")
}
