package otlp

import (
	"context"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
)

type collectorIdentityContextKey struct{}

func AuthenticatedConnectionFromContext(ctx context.Context) (authenticator.Connection, bool) {
	connection, ok := ctx.Value(collectorIdentityContextKey{}).(authenticator.Connection)
	return connection, ok
}

func CollectorIdentityFromContext(ctx context.Context) (string, bool) {
	connection, ok := AuthenticatedConnectionFromContext(ctx)
	return connection.PrincipalID, ok
}

func (s *Server) checkAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.authenticator.AuthenticateRequest(r.Context(), r)
		if err != nil {
			http.Error(w, "client requires collector identity", http.StatusUnauthorized)
			return
		}
		// TODO : context helper in authenticator.
		ctx := context.WithValue(r.Context(), collectorIdentityContextKey{}, conn)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
