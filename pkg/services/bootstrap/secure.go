//go:build !insecure

package bootstrap

import (
	"crypto"
	"log/slog"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
)

func NewBootstrapper(
	logger *slog.Logger,
	tokenStore types.KeyValue[*v1alpha1.BootstrapToken],
	privateKey crypto.Signer) Bootstrapper {
	return NewSecureBootstrapper(logger, tokenStore, privateKey)
}
