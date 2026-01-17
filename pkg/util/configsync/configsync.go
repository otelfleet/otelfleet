// Package configsync provides shared logic for computing config synchronization status.
package configsync

import (
	"bytes"
	"context"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
)

// ComputeConfigSyncStatus computes the config sync status for an agent.
// It compares the assigned config hash with the agent-reported hash and maps
// the OpAMP status to our unified ConfigSyncStatus.
//
// This function is shared between AgentService and ConfigServer to ensure
// consistent status computation.
func ComputeConfigSyncStatus(
	ctx context.Context,
	agentID string,
	assignedHash []byte,
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
) (v1alpha1.ConfigSyncStatus, string, error) {
	// If no assigned hash, we can't determine sync status
	if len(assignedHash) == 0 {
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_UNKNOWN, "no assigned config", nil
	}

	remoteStatus, err := remoteStatusStore.Get(ctx, agentID)
	if grpcutil.IsErrorNotFound(err) {
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC, "no status reported", nil
	} else if err != nil {
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC, "internal error", err
	}

	// Check if the hash matches what we assigned
	if !bytes.Equal(remoteStatus.GetLastRemoteConfigHash(), assignedHash) {
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC, "hash mismatch", nil
	}

	// Map OpAMP status to our status
	switch remoteStatus.GetStatus() {
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_IN_SYNC, "", nil
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_APPLYING, "", nil
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_ERROR, remoteStatus.GetErrorMessage(), nil
	default:
		return v1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_OUT_OF_SYNC, "unknown status", nil
	}
}
