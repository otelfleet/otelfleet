package authenticator_test

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestMTLSAuthenticator(t *testing.T) {
	testCases := []struct {
		name        string
		certificate *x509.Certificate
		want        authenticator.Connection
		wantCode    codes.Code
	}{
		{
			name:        "verified collector certificate",
			certificate: &x509.Certificate{URIs: []*url.URL{util.NewURIFromID("collector-1")}},
			want:        authenticator.Connection{PrincipalID: "collector-1", Method: authenticator.AuthMethodMTLS},
		},
		{
			name: "certificate with multiple collector identities",
			certificate: &x509.Certificate{URIs: []*url.URL{
				util.NewURIFromID("collector-1"),
				util.NewURIFromID("collector-2"),
			}},
			wantCode: codes.Unauthenticated,
		},
		{name: "certificate without collector identity", certificate: &x509.Certificate{}, wantCode: codes.Unauthenticated},
		{name: "no verified certificate", wantCode: codes.Unauthenticated},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authenticator := authenticator.NewMTLS()
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost/v1/opamp", nil)
			require.NoError(t, err)
			if tc.certificate != nil {
				request.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{tc.certificate},
					VerifiedChains:   [][]*x509.Certificate{{tc.certificate}},
				}
			}

			got, err := authenticator.AuthenticateRequest(t.Context(), request)
			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.True(t, grpcutil.IsError(tc.wantCode, err), "expected %s, got %v", tc.wantCode, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
