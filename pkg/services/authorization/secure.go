//go:build !insecure

package authorization

import (
	"crypto"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
)

func NewBootstrapper(
	tokenStore object.KeyValue[*v1alpha1.BootstrapToken],
	privateKey crypto.Signer) Bootstrapper {
	return NewSecureBootstrapper(tokenStore, privateKey)
}
