package authenticator

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/otelfleet/otelfleet/pkg/auth/token"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
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
	if err := ctx.Err(); err != nil {
		return Connection{}, err
	}
	if a.verifier == nil {
		return Connection{}, grpcutil.Error(
			codes.FailedPrecondition,
			fmt.Errorf("token verifier is not configured"),
		)
	}

	parts := strings.Fields(req.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("valid Bearer authorization is required"),
		)
	}

	principalID, err := a.verifier.VerifyToken(ctx, parts[1])
	if err != nil {
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("verify request token: %w", err),
		)
	}
	if principalID == "" {
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("request token has no principal ID"),
		)
	}
	return Connection{PrincipalID: principalID, Method: AuthMethodAPIToken}, nil
}
