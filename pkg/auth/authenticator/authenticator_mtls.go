package authenticator

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
)

type mtlsAuthenticator struct{}

func NewMTLS() Authenticator {
	return &mtlsAuthenticator{}
}

func (a *mtlsAuthenticator) AuthenticateRequest(
	ctx context.Context,
	req *http.Request,
) (Connection, error) {
	if err := ctx.Err(); err != nil {
		return Connection{}, err
	}
	if req.TLS == nil ||
		len(req.TLS.VerifiedChains) == 0 ||
		len(req.TLS.PeerCertificates) == 0 {
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("verified collector client certificate is required"),
		)
	}

	leaf := req.TLS.PeerCertificates[0]
	principalID, ok := CollectorIdentityFromCertificate(leaf)
	if !ok {
		return Connection{}, grpcutil.Error(
			codes.Unauthenticated,
			fmt.Errorf("verified client certificate has no valid collector identity"),
		)
	}
	return Connection{PrincipalID: principalID, Method: AuthMethodMTLS}, nil
}

func CollectorIdentityFromCertificate(certificate *x509.Certificate) (string, bool) {
	identities := []string{}
	for _, uri := range certificate.URIs {
		if uri.Scheme != "otelfleet" || uri.Host != "collectors" ||
			uri.RawQuery != "" || uri.Fragment != "" {
			continue
		}

		candidate := strings.TrimPrefix(uri.EscapedPath(), "/")
		if candidate == "" || strings.Contains(candidate, "/") {
			return "", false
		}
		identities = append(identities, candidate)
	}

	if len(identities) != 1 {
		return "", false
	}
	return identities[0], true
}
