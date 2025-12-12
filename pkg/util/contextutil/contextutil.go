package contextutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var ErrShutdown = errors.New("otelfleet shutdown requested")

func SetupSignals(ctx context.Context) context.Context {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	ctxCa, ca := context.WithCancelCause(ctx)
	go func() {
		defer ca(fmt.Errorf("signal received : %w", ErrShutdown))
		select {
		case <-sig:
			slog.Info("interrupt received")
		case <-ctxCa.Done():
		}
	}()
	return ctxCa
}
