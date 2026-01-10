package services

import (
	"context"
	"net/http"

	clienttypes "github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
)

type OpAmpServerHandler interface {
	// The following callbacks will never be called concurrently for the same
	// connection. They may be called concurrently for different connections.

	// OnConnected is called when an incoming OpAMP connection is successfully
	// established after OnConnecting() returns.
	OnConnected(ctx context.Context, conn servertypes.Connection)

	// OnMessage is called when a message is received from the connection. Can happen
	// only after OnConnected().
	// When the returned ServerToAgent message is nil, WebSocket will not send a
	// message to the Agent, and the HTTP request will respond to an empty message.
	// If the return is not nil it will be sent as a response to the Agent.
	// For plain HTTP requests once OnMessage returns and the response is sent
	// to the Agent the OnConnectionClose message will be called immediately.
	OnMessage(ctx context.Context, conn servertypes.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent

	// OnConnectionClose is called when the OpAMP connection is closed.
	OnConnectionClose(conn servertypes.Connection)

	// OnReadMessageError is called when an error occurs while reading or deserializing a message.
	OnReadMessageError(conn servertypes.Connection, mt int, msgByte []byte, err error)
}

type OpAmpClientHandler interface {
	// OnConnect is called when the connection is successfully established to the Server.
	// May be called after Start() is called and every time a connection is established to the Server.
	// For WebSocket clients this is called after the handshake is completed without any error.
	// For HTTP clients this is called for any request if the response status is OK.
	OnConnect(ctx context.Context)

	// OnConnectFailed is called when the connection to the Server cannot be established.
	// May be called after Start() is called and tries to connect to the Server.
	// May also be called if the connection is lost and reconnection attempt fails.
	OnConnectFailed(ctx context.Context, err error)

	// OnError is called when the Server reports an error in response to some previously
	// sent request. Useful for logging purposes. The Agent should not attempt to process
	// the error by reconnecting or retrying previous operations. The client handles the
	// ErrorResponse_UNAVAILABLE case internally by performing retries as necessary.
	OnError(ctx context.Context, err *protobufs.ServerErrorResponse)

	// OnMessage is called when the Agent receives a message that needs processing.
	// See MessageData definition for the data that may be available for processing.
	// During OnMessage execution the OpAMPClient functions that change the status
	// of the client may be called, e.g. if RemoteConfig is processed then
	// SetRemoteConfigStatus should be called to reflect the processing result.
	// These functions may also be called after OnMessage returns. This is advisable
	// if processing can take a long time. In that case returning quickly is preferable
	// to avoid blocking the OpAMPClient.
	OnMessage(ctx context.Context, msg *clienttypes.MessageData)

	// OnOpampConnectionSettings is called when the Agent receives an OpAMP
	// connection settings offer from the Server. Typically, the settings can specify
	// authorization headers or TLS certificate, potentially also a different
	// OpAMP destination to work with.
	//
	// The Agent should process the offer by reconnecting the client using the new
	// settings or return an error if the Agent does not want to accept the settings
	// (e.g. if the TSL certificate in the settings cannot be verified).
	//
	// Only one OnOpampConnectionSettings call can be active at any time.
	// See OnRemoteConfig for the behavior.
	OnOpampConnectionSettings(
		ctx context.Context,
		settings *protobufs.OpAMPConnectionSettings,
	) error

	// For all methods that accept a context parameter the caller may cancel the
	// context if processing takes too long. In that case the method should return
	// as soon as possible with an error.

	// SaveRemoteConfigStatus is called after OnRemoteConfig returns. The status
	// will be set either as APPLIED or FAILED depending on whether OnRemoteConfig
	// returned a success or error.
	// The Agent must remember this RemoteConfigStatus and supply in the future
	// calls to Start() in StartSettings.RemoteConfigStatus.
	SaveRemoteConfigStatus(ctx context.Context, status *protobufs.RemoteConfigStatus)

	// GetEffectiveConfig returns the current effective config. Only one
	// GetEffectiveConfig call can be active at any time. Until GetEffectiveConfig
	// returns it will not be called again.
	GetEffectiveConfig(ctx context.Context) (*protobufs.EffectiveConfig, error)

	// OnCommand is called when the Server requests that the connected Agent perform a command.
	OnCommand(ctx context.Context, command *protobufs.ServerToAgentCommand) error

	// CheckRedirect is called before following a redirect, allowing the client
	// the opportunity to observe the redirect chain, and optionally terminate
	// following redirects early.
	//
	// CheckRedirect is intended to be similar, although not exactly equivalent,
	// to net/http.Client's CheckRedirect feature. Unlike in net/http, the via
	// parameter is a slice of HTTP responses, instead of requests. This gives
	// an opportunity to users to know what the exact response headers and
	// status were. The request itself can be obtained from the response.
	//
	// The responses in the via parameter are passed with their bodies closed.
	CheckRedirect(req *http.Request, viaReq []*http.Request, via []*http.Response) error

	// DownloadHTTPClient is called to create an HTTP client that is used to download files by the package syncer.
	// If the callback is not set, a default HTTP client will be created with the default transport settings.
	// The callback must return a non-nil HTTP client or an error.
	DownloadHTTPClient(ctx context.Context, file *protobufs.DownloadableFile) (*http.Client, error)
}
