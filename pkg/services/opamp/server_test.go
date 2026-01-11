package opamp_test

import (
	"log/slog"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default().With("service", "opamp-server")

	srvImpl := opamp.NewServer(
		logger,
		nil,
		opamp.NewAgentTracker(),
		storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("agent-health")),
		storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("agent-effective-config")),
		storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("agent-remote-config-status")),
	)
	settings := testutil.SetupOpampServerImpl(t, srvImpl)
	opampSrv := server.New(logutil.NewOpAMPLogger(slog.Default().With("service", "server")))
	httpSrv := testutil.SetupOpampServer(t, opampSrv, settings)
	require.NotNil(t, httpSrv)
}
