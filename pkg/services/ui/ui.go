package ui

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/config"
	otelfleetsvc "github.com/otelfleet/otelfleet/pkg/services"
)

type UIService struct {
	services.Service

	logger  *slog.Logger
	cfg     *config.UIConfig
	handler http.Handler
	proxy   http.Handler
}

var _ otelfleetsvc.HTTPService = (*UIService)(nil)

func NewUIService(logger *slog.Logger, cfg *config.UIConfig) (*UIService, error) {
	u := &UIService{
		logger: logger,
		cfg:    cfg,
	}

	handler, err := u.buildHandler()
	if err != nil {
		return nil, err
	}
	u.handler = handler

	if cfg.ApiURL != "" {
		proxy, err := newUpstreamProxy(cfg.ApiURL)
		if err != nil {
			return nil, err
		}
		u.proxy = proxy
	}

	u.Service = services.NewBasicService(nil, u.running, nil)
	return u, nil
}

func (u *UIService) buildHandler() (http.Handler, error) {
	assets, err := Assets()
	if err != nil {
		return nil, err
	}
	return u.newEmbeddedHandler(assets)
}

func (u *UIService) newEmbeddedHandler(assets fs.FS) (http.Handler, error) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("reading embedded index.html: %w", err)
	}

	fileServer := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" || clean == "index.html" {
			serveIndex(w, index)
			return
		}
		if f, err := assets.Open(clean); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w, index)
	}), nil
}

func newUpstreamProxy(upstream string) (http.Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parsing ui upstream: %w", err)
	}
	return httputil.NewSingleHostReverseProxy(target), nil
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(index)
}

func (u *UIService) ConfigureHTTP(reg otelfleetsvc.HTTPRegistrar) {
	u.logger.With("prefix", u.cfg.PathPrefix, "proxy", u.proxy != nil).Info("mounting UI on shared listener")
	u.mountWithRegistrar(reg)
}

// mount remains as a small uninstrumented helper for focused router tests.
func (u *UIService) mount(router *mux.Router) {
	u.mountWithRegistrar(uiMuxRegistrar{router})
}

type uiRegistrar interface {
	Handle(pattern string, handler http.Handler) *mux.Route
	HandleFunc(pattern string, handler http.HandlerFunc) *mux.Route
	HandlePrefix(prefix string, handler http.Handler) *mux.Route
}

type uiMuxRegistrar struct{ *mux.Router }

func (r uiMuxRegistrar) HandleFunc(pattern string, handler http.HandlerFunc) *mux.Route {
	return r.Router.HandleFunc(pattern, handler)
}

func (r uiMuxRegistrar) HandlePrefix(prefix string, handler http.Handler) *mux.Route {
	return r.PathPrefix(prefix).Handler(handler)
}

func (u *UIService) mountWithRegistrar(reg uiRegistrar) {
	prefix := strings.TrimRight(u.cfg.PathPrefix, "/")
	if prefix == "" {
		reg.HandlePrefix("/", u.handler)
		return
	}
	reg.HandlePrefix(prefix+"/", http.StripPrefix(prefix, u.handler))
	reg.Handle(prefix, http.StripPrefix(prefix, u.handler))

	// Only register the catch-all when standalone; in all-in-one it would
	// shadow the API modules' routes on the shared listener.
	if u.proxy != nil {
		reg.HandlePrefix("/", u.proxy)
		return
	}
	reg.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, prefix+"/", http.StatusFound)
	})
}

func (u *UIService) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
