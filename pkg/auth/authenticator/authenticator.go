package authenticator

import (
	"context"
	"net/http"
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

func (n *noopAuthenticator) AuthenticateRequest(context.Context, *http.Request) (Connection, error) {
	return Connection{Method: AuthMethodNone}, nil
}
