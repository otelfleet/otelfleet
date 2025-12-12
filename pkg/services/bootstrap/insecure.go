//go:build insecure

package bootstrap

import (
	"crypto"
	"log/slog"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
)

func NewBootstrapper(
	logger *slog.Logger,
	_ storage.KeyValue[*v1alpha1.BootstrapToken],
	_ crypto.Signer) Bootstrapper {
	return NewNoopBootstrapper(logger)
}
