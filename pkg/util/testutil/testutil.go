package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	servertypes "github.com/open-telemetry/opamp-go/server/types"
	services_int "github.com/otelfleet/otelfleet/pkg/services"

	"github.com/stretchr/testify/require"
)

func SetupOpampServer(t *testing.T, s server.OpAMPServer, settings server.Settings) *httptest.Server {
	t.Helper()
	handlerFunc, _, err := s.Attach(
		settings,
	)
	require.NoError(t, err)
	// TODO : maybe we need to construct the server with ConnContext
	// and copy over the local listener construction.
	srv := httptest.NewServer(http.HandlerFunc(handlerFunc))
	t.Cleanup(func() {
		srv.Close()
	})

	return srv
}

func SetupOpampClient(
	t *testing.T,
	oClient client.OpAMPClient,
	opampURL string,
	desc *protobufs.AgentDescription,
	startSet *types.StartSettings,
) client.OpAMPClient {
	t.Helper()
	require.NotNil(t, desc, "agent description must be set")
	require.NotEmpty(t, opampURL, "server URL must be set")
	require.NotNil(t, startSet, "start settings must be populated")
	startSet.OpAMPServerURL = opampURL
	oClient.SetAgentDescription(desc)
	require.NoError(t, oClient.Start(t.Context(), *startSet))
	t.Cleanup(func() { oClient.Stop(t.Context()) })
	return oClient
}

func SetupOpampServerImpl(t *testing.T, s services_int.OpAmpServerHandler) server.Settings {
	t.Helper()
	return server.Settings{
		Callbacks: servertypes.Callbacks{
			OnConnecting: func(request *http.Request) servertypes.ConnectionResponse {
				return servertypes.ConnectionResponse{
					Accept: true,
					ConnectionCallbacks: servertypes.ConnectionCallbacks{
						OnConnected:        s.OnConnected,
						OnMessage:          s.OnMessage,
						OnConnectionClose:  s.OnConnectionClose,
						OnReadMessageError: s.OnReadMessageError,
					},
				}
			},
		},
	}

}
