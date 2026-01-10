package testutil_test

import (
	"log/slog"
	"testing"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"

	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/testutil"
)

func TestOpAmp(t *testing.T) {
	opampServer := server.New(logutil.NewOpAMPLogger(slog.Default().With("servvice", "server")))
	settings := server.Settings{}
	srv := testutil.SetupOpampServer(t, opampServer, settings)

	opampClient := client.NewHTTP(logutil.NewOpAMPLogger(slog.Default().With("service", "client")))
	desc := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			util.KeyVal("foo", "bar"),
		},
	}
	set := &types.StartSettings{
		InstanceUid: util.NewInstanceUUID(),
		TLSConfig:   nil,
	}
	_ = testutil.SetupOpampClient(t, opampClient, srv.URL, desc, set)
}
