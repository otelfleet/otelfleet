// Package client provides bootstrap client functionality for agents to register
// with an OtelFleet server. It supports both secure (cryptographic) and insecure
// (development) bootstrap modes.
package client

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/ident"
)

// Bootstrapper defines the interface for agent bootstrap operations.
type Bootstrapper interface {
	// VerifyToken validates a bootstrap token before attempting to bootstrap.
	VerifyToken(ctx context.Context, token string) error

	// Bootstrap performs the bootstrap handshake with the server.
	// It registers the agent and optionally negotiates TLS credentials.
	Bootstrap(ctx context.Context, req *BootstrapRequest) (*BootstrapResult, error)
}

// BootstrapRequest contains the parameters for a bootstrap request.
type BootstrapRequest struct {
	// ClientID is the unique identifier for this agent.
	ClientID string

	// Name is the human-readable name for this agent.
	Name string

	// Token is the bootstrap token (for insecure mode, this is passed as Authorization header).
	Token string

	// ClientPubKey is the agent's ephemeral public key for ECDH (secure mode only).
	ClientPubKey []byte
}

// BootstrapResult contains the result of a successful bootstrap.
type BootstrapResult struct {
	// TLSConfig is the TLS configuration for secure communication (may be nil for insecure mode).
	TLSConfig *tls.Config

	// ServerPubKey is the server's ephemeral public key (secure mode only).
	ServerPubKey []byte
}

// Config holds the configuration for creating a bootstrap client.
type Config struct {
	// Logger for bootstrap operations.
	Logger *slog.Logger

	// ServerURL is the base URL of the OtelFleet server (e.g., "http://127.0.0.1:16587").
	ServerURL string

	// HTTPClient is the HTTP client to use. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// Client is a bootstrap client that can register agents with an OtelFleet server.
type Client struct {
	logger          *slog.Logger
	serverURL       string
	httpClient      *http.Client
	bootstrapClient v1alpha1connect.BootstrapServiceClient
	tokenClient     v1alpha1connect.TokenServiceClient
	bootstrapper    Bootstrapper
}

// New creates a new bootstrap client with the given configuration.
// The secure parameter determines whether to use cryptographic bootstrap (true)
// or simple/insecure bootstrap (false).
func New(cfg Config, secure bool) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	bootstrapClient := v1alpha1connect.NewBootstrapServiceClient(httpClient, cfg.ServerURL)
	tokenClient := v1alpha1connect.NewTokenServiceClient(httpClient, cfg.ServerURL)

	var bootstrapper Bootstrapper
	if secure {
		bootstrapper = newSecureBootstrapper(cfg.Logger, cfg.ServerURL, httpClient, bootstrapClient, tokenClient)
	} else {
		bootstrapper = newInsecureBootstrapper(cfg.Logger, bootstrapClient, tokenClient)
	}

	return &Client{
		logger:          cfg.Logger,
		serverURL:       cfg.ServerURL,
		httpClient:      httpClient,
		bootstrapClient: bootstrapClient,
		tokenClient:     tokenClient,
		bootstrapper:    bootstrapper,
	}
}

// NewInsecure creates a new bootstrap client configured for insecure/development mode.
func NewInsecure(cfg Config) *Client {
	return New(cfg, false)
}

// NewSecure creates a new bootstrap client configured for secure/production mode.
func NewSecure(cfg Config) *Client {
	return New(cfg, true)
}

// NewWithBootstrapper creates a new bootstrap client with a custom bootstrapper.
// This is useful for testing with mock bootstrappers.
func NewWithBootstrapper(cfg Config, bootstrapper Bootstrapper) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		logger:          cfg.Logger,
		serverURL:       cfg.ServerURL,
		httpClient:      httpClient,
		bootstrapClient: v1alpha1connect.NewBootstrapServiceClient(httpClient, cfg.ServerURL),
		tokenClient:     v1alpha1connect.NewTokenServiceClient(httpClient, cfg.ServerURL),
		bootstrapper:    bootstrapper,
	}
}

// VerifyToken validates a bootstrap token.
func (c *Client) VerifyToken(ctx context.Context, token string) error {
	return c.bootstrapper.VerifyToken(ctx, token)
}

// Bootstrap registers the agent with the server.
func (c *Client) Bootstrap(ctx context.Context, req *BootstrapRequest) (*BootstrapResult, error) {
	return c.bootstrapper.Bootstrap(ctx, req)
}

// BootstrapAgent is a convenience method that verifies the token and performs
// the bootstrap in one call using the provided identity.
func (c *Client) BootstrapAgent(ctx context.Context, identity ident.Identity, name, token string) (*BootstrapResult, error) {
	if err := c.VerifyToken(ctx, token); err != nil {
		return nil, err
	}

	return c.Bootstrap(ctx, &BootstrapRequest{
		ClientID: identity.UniqueIdentifier().UUID,
		Name:     name,
		Token:    token,
	})
}

// insecureBootstrapper implements Bootstrapper for development/testing without cryptography.
type insecureBootstrapper struct {
	logger  *slog.Logger
	bClient v1alpha1connect.BootstrapServiceClient
	tClient v1alpha1connect.TokenServiceClient
}

func newInsecureBootstrapper(
	logger *slog.Logger,
	bClient v1alpha1connect.BootstrapServiceClient,
	tClient v1alpha1connect.TokenServiceClient,
) *insecureBootstrapper {
	return &insecureBootstrapper{
		logger:  logger.With("bootstrapper", "insecure"),
		bClient: bClient,
		tClient: tClient,
	}
}

func (b *insecureBootstrapper) VerifyToken(ctx context.Context, token string) error {
	b.logger.With("token", token).Debug("verified token (insecure mode)")
	return nil
}

func (b *insecureBootstrapper) Bootstrap(ctx context.Context, req *BootstrapRequest) (*BootstrapResult, error) {
	// Set the token as Authorization header
	connectReq := connect.NewRequest(&v1alpha1.BootstrapAuthRequest{
		ClientId:     req.ClientID,
		Name:         req.Name,
		ClientPubKey: req.ClientPubKey,
	})
	connectReq.Header().Set("Authorization", req.Token)

	b.logger.With("client_id", req.ClientID, "name", req.Name).Debug("bootstrapping agent")

	_, err := b.bClient.Bootstrap(ctx, connectReq)
	if err != nil {
		return nil, err
	}

	return &BootstrapResult{
		TLSConfig: nil, // No TLS in insecure mode
	}, nil
}
