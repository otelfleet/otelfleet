package authenticator

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/otelfleet/otelfleet/pkg/auth/token"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/otelfleet/otelfleet/pkg/util/serviceutil"
	"github.com/otelfleet/otelfleet/pkg/util/traceutil"
	"go.opentelemetry.io/otel/attribute"
	otelcode "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
)

type tokenAuthenticator struct {
	verifier token.Verifier
}

func NewToken(verifier token.Verifier) Authenticator {
	return &tokenAuthenticator{verifier: verifier}
}

func (a *tokenAuthenticator) AuthenticateRequest(
	ctx context.Context,
	req *http.Request,
) (Connection, error) {
	ctx, span := traceutil.Continue(
		ctx, serviceutil.Auth.Name(), "authenticator.AuthenticateRequest",
		trace.WithAttributes(
			attribute.String("AuthMethod", string(AuthMethodMTLS)),
		),
	)
	defer span.End()
	if a.verifier == nil {
		span.SetStatus(otelcode.Error, "No token verified")
		return Connection{}, grpcutil.Error(
			codes.FailedPrecondition,
			fmt.Errorf("token verifier is not configured"),
		)
	}

	parts := strings.Fields(req.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		span.SetStatus(otelcode.Error, "Unauthenticated - No Authorization Bearer")
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("valid Bearer authorization is required"),
		)
	}

	principalID, err := a.verifier.VerifyToken(ctx, parts[1])
	if err != nil {
		span.SetStatus(otelcode.Error, "Unauthenticated - invalid token")
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("verify request token: %w", err),
		)
	}
	if principalID == "" {
		span.SetStatus(otelcode.Error, "Unauthenticated - no token identity")
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("request token has no principal ID"),
		)
	}
	return Connection{PrincipalID: principalID, Method: AuthMethodAPIToken}, nil
}
