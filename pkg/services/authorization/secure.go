//go:build !insecure

package authorization

import (
	"crypto"
	"log/slog"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
)

func NewBootstrapper(
	logger *slog.Logger,
	tokenStore object.KeyValue[*v1alpha1.BootstrapToken],
	privateKey crypto.Signer) Bootstrapper {
	return NewSecureBootstrapper(logger, tokenStore, privateKey)
}
