package main

import (
	"log/slog"
	"os"
	"os/signal"

	_ "github.com/otelfleet/otelfleet/pkg/logger"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
)

func main() {
	logger := slog.Default()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	supervisor := supervisor.NewSupervisor(
		slog.Default().With("component", "supervisor"),
	)
	logger.Info("otelfleet agent starting...")
	if err := supervisor.Start(); err != nil {
		logger.With("err", err.Error()).Error("failed to start supervisor")
		os.Exit(1)
	}
	<-interrupt
	logger.Info("shutting down otelfleet agent...")
	if err := supervisor.Shutdown(); err != nil {
		logger.With("err", err.Error()).Error("failed to shutdown supervisor")
		os.Exit(1)
	}
}
