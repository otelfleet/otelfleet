package opamp_test

import (
	"log/slog"
	"testing"

	"github.com/open-telemetry/opamp-go/server"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	srvImpl := opamp.NewServer(
		slog.Default().With("service", "opamp-server"),
		nil,
		opamp.NewAgentTracker(),
	)
	settings := testutil.SetupOpampServerImpl(t, srvImpl)
	opampSrv := server.New(logutil.NewOpAMPLogger(slog.Default().With("service", "server")))
	httpSrv := testutil.SetupOpampServer(t, opampSrv, settings)
	require.NotNil(t, httpSrv)

	
}
