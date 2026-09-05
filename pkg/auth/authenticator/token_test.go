package authenticator_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/auth/token"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

type testTokenVerifier struct {
	principalID string
	err         error
	gotToken    string
}

func (v *testTokenVerifier) VerifyToken(_ context.Context, token string) (string, error) {
	v.gotToken = token
	return v.principalID, v.err
}

func TestTokenAuthenticator(t *testing.T) {
	testCases := []struct {
		name          string
		authorization string
		verifier      *testTokenVerifier
		want          authenticator.Connection
		wantCode      codes.Code
		wantToken     string
	}{
		{
			name:          "valid bearer token",
			authorization: "Bearer token-value",
			verifier:      &testTokenVerifier{principalID: "bootstrap-token"},
			want:          authenticator.Connection{PrincipalID: "bootstrap-token", Method: authenticator.AuthMethodAPIToken},
			wantToken:     "token-value",
		},
		{
			name:          "bearer scheme is case insensitive",
			authorization: "bearer token-value",
			verifier:      &testTokenVerifier{principalID: "bootstrap-token"},
			want:          authenticator.Connection{PrincipalID: "bootstrap-token", Method: authenticator.AuthMethodAPIToken},
			wantToken:     "token-value",
		},
		{name: "missing authorization", verifier: &testTokenVerifier{}, wantCode: codes.Unauthenticated},
		{name: "wrong authorization scheme", authorization: "Basic value", verifier: &testTokenVerifier{}, wantCode: codes.Unauthenticated},
		{name: "invalid token", authorization: "Bearer bad", verifier: &testTokenVerifier{err: errors.New("invalid token")}, wantCode: codes.Unauthenticated, wantToken: "bad"},
		{name: "token without principal", authorization: "Bearer value", verifier: &testTokenVerifier{}, wantCode: codes.Unauthenticated, wantToken: "value"},
		{name: "missing verifier", authorization: "Bearer value", wantCode: codes.FailedPrecondition},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var verifier token.Verifier
			if tc.verifier != nil {
				verifier = tc.verifier
			}
			authenticator := authenticator.NewToken(verifier)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost/v1/opamp", nil)
			require.NoError(t, err)
			request.Header.Set("Authorization", tc.authorization)

			got, err := authenticator.AuthenticateRequest(t.Context(), request)
			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.True(t, grpcutil.IsError(tc.wantCode, err), "expected %s, got %v", tc.wantCode, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
			if tc.verifier != nil {
				assert.Equal(t, tc.wantToken, tc.verifier.gotToken)
			}
		})
	}
}
