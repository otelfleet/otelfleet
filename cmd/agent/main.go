package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/ident"
	_ "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"github.com/otelfleet/otelfleet/pkg/util/contextutil"
)

const (
	gatewayAddr = "http://127.0.0.1:8081"
	opAmpAddr   = "ws://127.0.0.1:4320/v1/opamp"
)

type Bootstrapper interface {
	VerifyToken(token string) error
	Bootstrap(ctx context.Context, req *v1alpha1.BootstrapAuthRequest) (*tls.Config, error)
}

func main() {
	logger := slog.Default()
	ctx := contextutil.SetupSignals(context.Background())

	bootstrapToken := os.Getenv("BOOTSTRAP_TOKEN")
	agentName := os.Getenv("AGENT_NAME")

	logger.Debug("acquiring bootstrap client...")
	httpClient := http.DefaultClient
	bootstrapClient := bootstrapv1alpha1.NewBootstrapServiceClient(
		httpClient,
		gatewayAddr,
	)
	logger.Debug("acquiring token client...")
	tokenClient := bootstrapv1alpha1.NewTokenServiceClient(
		httpClient,
		gatewayAddr,
	)

	bootstrapper := NewBootstrapper(
		logger.With("component", "bootstrapper"),
		bootstrapClient,
		tokenClient,
	)

	if err := bootstrapper.VerifyToken(bootstrapToken); err != nil {
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
	id, err := ident.IdFromMac(sha256.New(), agentName, map[string]string{})
	if err != nil {
		logger.With("err", err).Error("failed to get agent identity")
		os.Exit(1)
	}

	// FIXME: backoff retry
	tlsConf, err := bootstrapper.Bootstrap(
		ctx,
		&v1alpha1.BootstrapAuthRequest{
			ClientId: id.UniqueIdentifier().UUID,
			Name:     agentName,
		},
	)
	if err != nil {
		logger.With("err", err).Error("failed to bootstrap agent")
		os.Exit(1)
	}

	supervisor := supervisor.NewSupervisor(
		slog.Default().With("component", "supervisor"),
		tlsConf,
		opAmpAddr,
	)
	logger.Info("otelfleet agent starting...")
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
