//go:build insecure

package main

import (
	"context"
	"crypto/tls"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
)

func NewBootstrapper(
	logger *slog.Logger,
	bClient bootstrapv1alpha1.BootstrapServiceClient,
	tClient bootstrapv1alpha1.TokenServiceClient,
) Bootstrapper {
	return &noopBootstrapper{
		logger:  logger,
		bClient: bClient,
		tClient: tClient,
	}
}

type noopBootstrapper struct {
	logger  *slog.Logger
	bClient bootstrapv1alpha1.BootstrapServiceClient
	tClient bootstrapv1alpha1.TokenServiceClient
	token   string
}

func (n *noopBootstrapper) VerifyToken(token string) error {
	n.token = token
	n.logger.With("token", token).Debug("verified")
	return nil
}

func (n *noopBootstrapper) Bootstrap(ctx context.Context, req *v1alpha1.BootstrapAuthRequest) (*tls.Config, error) {
	ctx, withToken := connect.NewClientContext(ctx)
	withToken.RequestHeader().Set("Authorization", n.token)
	n.logger.With("token", n.token).Debug("requesting access")
	_, err := n.bClient.Bootstrap(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return nil, nil
}
