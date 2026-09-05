package authenticator

import (
	"context"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/util/serviceutil"
	"github.com/otelfleet/otelfleet/pkg/util/traceutil"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type AuthMethod string

const (
	AuthMethodNone     AuthMethod = "none"
	AuthMethodAPIToken AuthMethod = "api-token"
	AuthMethodMTLS     AuthMethod = "mtls"
)

type Connection struct {
	PrincipalID string
	Method      AuthMethod
}

type Authenticator interface {
	AuthenticateRequest(ctx context.Context, req *http.Request) (Connection, error)
}

type noopAuthenticator struct{}

func NewNoop() Authenticator {
	return &noopAuthenticator{}
}

func (n *noopAuthenticator) AuthenticateRequest(ctx context.Context, r *http.Request) (Connection, error) {
	ctx, span := traceutil.Continue(ctx, serviceutil.Auth.Name(), "authenticator.AuthenticateRequest", trace.WithAttributes(
		attribute.String("AuthMethod", string(AuthMethodNone)),
	))
	defer span.End()
	return Connection{Method: AuthMethodNone}, nil
}
