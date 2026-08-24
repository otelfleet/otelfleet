package pending

// TODO : maybe differentiate this handler to be one that handles bootstrap specifically.
// Once bootstrapped this handler transitions to the generic stateless handler.

import (
	"context"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server/types"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
	services_int "github.com/otelfleet/otelfleet/pkg/services"
)

type PendingCollectorHandler struct{}

var _ services_int.OpAmpServerHandler = (*PendingCollectorHandler)(nil)

// The following callbacks will never be called concurrently for the same
// connection. They may be called concurrently for different connections.

// OnConnected is called when an incoming OpAMP connection is successfully
// established after OnConnecting() returns.
func (h *PendingCollectorHandler) OnConnected(ctx context.Context, conn servertypes.Connection) {
	panic("implement me")
}

// OnMessage is called when a message is received from the connection. Can happen
// only after OnConnected().
// When the returned ServerToAgent message is nil, WebSocket will not send a
// message to the Agent, and the HTTP request will respond to an empty message.
// If the return is not nil it will be sent as a response to the Agent.
// For plain HTTP requests once OnMessage returns and the response is sent
// to the Agent the OnConnectionClose message will be called immediately.
func (h *PendingCollectorHandler) OnMessage(ctx context.Context, conn servertypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	panic("implement me")
}

// OnConnectionClose is called when the OpAMP connection is closed.
func (h *PendingCollectorHandler) OnConnectionClose(conn servertypes.Connection) {
	panic("implement me")
}

// OnReadMessageError is called when an error occurs while reading or deserializing a message.
func (h *PendingCollectorHandler) OnReadMessageError(conn servertypes.Connection, mt int, msgByte []byte, err error) {
	panic("implement me")
}

// OnMessageResponseError is called when an error occurs while sending the response message from the OnMessage loop.
func (h *PendingCollectorHandler) OnMessageResponseError(conn types.Connection, message *protobufs.ServerToAgent, err error) {
	panic("implement me")
}
