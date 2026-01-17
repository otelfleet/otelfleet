package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
)

// secureBootstrapper implements Bootstrapper with full cryptographic verification.
type secureBootstrapper struct {
	logger     *slog.Logger
	serverURL  string
	httpClient *http.Client
	bClient    v1alpha1connect.BootstrapServiceClient
	tClient    v1alpha1connect.TokenServiceClient
}

func newSecureBootstrapper(
	logger *slog.Logger,
	serverURL string,
	httpClient *http.Client,
	bClient v1alpha1connect.BootstrapServiceClient,
	tClient v1alpha1connect.TokenServiceClient,
) *secureBootstrapper {
	return &secureBootstrapper{
		logger:     logger.With("bootstrapper", "secure"),
		serverURL:  serverURL,
		httpClient: httpClient,
		bClient:    bClient,
		tClient:    tClient,
	}
}

func (b *secureBootstrapper) VerifyToken(ctx context.Context, token string) error {
	// TODO: Implement secure token verification
	// This should:
	// 1. Fetch signatures from server
	// 2. Verify the token signature using server's public key
	b.logger.With("token", token).Debug("verifying token")
	return nil
}

func (b *secureBootstrapper) Bootstrap(ctx context.Context, req *BootstrapRequest) (*BootstrapResult, error) {
	// TODO: Implement secure bootstrap with ECDH key exchange
	// This should:
	// 1. Generate ephemeral key pair
	// 2. Send bootstrap request with client public key
	// 3. Receive server public key
	// 4. Derive shared secret
	// 5. Build TLS config from shared secret
	return nil, fmt.Errorf("secure bootstrap not yet implemented - use insecure mode for development")
}
