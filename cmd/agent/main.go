package main

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"os"

	bootstrapclient "github.com/otelfleet/otelfleet/pkg/bootstrap/client"
	"github.com/otelfleet/otelfleet/pkg/ident"
	_ "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util/contextutil"
)

const (
	gatewayAddr = "http://127.0.0.1:16587"
	opAmpAddr   = "ws://127.0.0.1:4320/v1/opamp"
)

func main() {
	logger := slog.Default()
	ctx := contextutil.SetupSignals(context.Background())

	bootstrapToken := os.Getenv("BOOTSTRAP_TOKEN")
	agentName := os.Getenv("AGENT_NAME")

	// Create bootstrap client using shared package
	// isSecureMode() is defined in insecure.go or secure.go based on build tags
	client := bootstrapclient.New(
		bootstrapclient.Config{
			Logger:    logger.With("component", "bootstrapper").With("agent-name", agentName).With("token", bootstrapToken),
			ServerURL: gatewayAddr,
		},
		isSecureMode(),
	)

	if err := client.VerifyToken(ctx, bootstrapToken); err != nil {
		logger.With("err", err).Error("failed to verify bootstrap token")
		os.Exit(1)
	}

	// FIXME: this only accounts for baremetal environments.
	// I need to extend this for container / kubernetes
	// but I think I'd rather keep imports on those things in a separate repo
	// since they're more of a known compile time thing, and can come with dependency hell and
	// binary bloat.
	// Perhaps the API to construct agents can live here, but agent builds and capabilities
	// are registered in an out-of-scope repo?
	agentID, err := ident.IdFromMac(sha256.New(), agentName)
	if err != nil {
		logger.With("err", err).Error("failed to get agent identity")
		os.Exit(1)
	}

	// FIXME: backoff retry
	result, err := client.BootstrapAgent(ctx, agentID, agentName, bootstrapToken)
	if err != nil {
		logger.With("err", err).Error("failed to bootstrap agent")
		os.Exit(1)
	}

	supervisor := supervisor.NewSupervisorWithProcManager(
		slog.Default().With("component", "supervisor"),
		result.TLSConfig,
		opAmpAddr,
		agentID,
	)
	logger.With("agentID", agentID.UniqueIdentifier().UUID).Info("otelfleet agent starting...")
	if err := supervisor.Start(); err != nil {
		logger.With("err", err.Error()).Error("failed to start supervisor")
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("shutting down otelfleet agent...")
	if err := supervisor.Shutdown(); err != nil {
		logger.With("err", err.Error()).Error("failed to shutdown supervisor")
		os.Exit(1)
	}
}
