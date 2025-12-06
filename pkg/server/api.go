package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/grafana/dskit/server"
	"github.com/otelfleet/otelfleet/pkg/logutil"
)

func printRoutes(r *mux.Router, l *slog.Logger) {
	l.Debug("walking routes")
	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		path, err := route.GetPathRegexp()
		if err != nil {
			fmt.Printf("failed to get path regexp %s\n", err)
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			methods = []string{http.MethodPost}
		}
		for _, method := range methods {
			logutil.WithMethod(l, method).Info(path)
		}
		return nil
	})
	if err != nil {
		l.With("err", err).Error("failed to walk routes")
	}
}

type API struct {
	Server *server.Server
}
