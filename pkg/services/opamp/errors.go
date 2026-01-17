package opamp

import "github.com/open-telemetry/opamp-go/protobufs"

// NewUnavailableError creates an error response for transient failures (e.g., storage errors).
// The agent should retry later.
func NewUnavailableError(msg string) *protobufs.ServerErrorResponse {
	return &protobufs.ServerErrorResponse{
		Type:         protobufs.ServerErrorResponseType_ServerErrorResponseType_Unavailable,
		ErrorMessage: msg,
	}
}

// NewBadRequestError creates an error response for malformed or invalid messages.
// The agent should not retry.
func NewBadRequestError(msg string) *protobufs.ServerErrorResponse {
	return &protobufs.ServerErrorResponse{
		Type:         protobufs.ServerErrorResponseType_ServerErrorResponseType_BadRequest,
		ErrorMessage: msg,
	}
}

// ErrorResponse creates a ServerToAgent with only the error field set.
// Per the OpAMP spec, when ErrorResponse is set, all other fields must be unset.
func ErrorResponse(instanceUID []byte, err *protobufs.ServerErrorResponse) *protobufs.ServerToAgent {
	return &protobufs.ServerToAgent{
		InstanceUid:   instanceUID,
		ErrorResponse: err,
	}
}
