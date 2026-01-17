package opamp

import (
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
)

func TestNewUnavailableError(t *testing.T) {
	err := NewUnavailableError("storage failure")

	assert.Equal(t, protobufs.ServerErrorResponseType_ServerErrorResponseType_Unavailable, err.Type)
	assert.Equal(t, "storage failure", err.ErrorMessage)
}

func TestNewBadRequestError(t *testing.T) {
	err := NewBadRequestError("agent not registered")

	assert.Equal(t, protobufs.ServerErrorResponseType_ServerErrorResponseType_BadRequest, err.Type)
	assert.Equal(t, "agent not registered", err.ErrorMessage)
}

func TestErrorResponse(t *testing.T) {
	instanceUID := []byte("test-instance-uid")
	serverErr := NewUnavailableError("test error")

	resp := ErrorResponse(instanceUID, serverErr)

	assert.Equal(t, instanceUID, resp.InstanceUid)
	assert.Equal(t, serverErr, resp.ErrorResponse)
	// Per OpAMP spec, when ErrorResponse is set, other fields should be unset
	assert.Nil(t, resp.RemoteConfig)
	assert.Nil(t, resp.ConnectionSettings)
	assert.Nil(t, resp.PackagesAvailable)
	assert.Zero(t, resp.Flags)
	assert.Zero(t, resp.Capabilities)
}
