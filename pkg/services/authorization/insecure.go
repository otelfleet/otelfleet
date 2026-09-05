//go:build insecure

package authorization

import (
	"crypto"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
)

func NewBootstrapper(
	_ object.KeyValue[*v1alpha1.BootstrapToken],
	_ crypto.Signer) Bootstrapper {
	return NewNoopBootstrapper()
}
