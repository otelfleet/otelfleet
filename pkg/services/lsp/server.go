package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelcol-lsp/pkg/lsp"
	"github.com/otelfleet/otelcol-lsp/pkg/lsp/types"
	"github.com/otelfleet/otelcol-lsp/pkg/lspws"
	"github.com/otelfleet/otelcol-lsp/pkg/otelcfg/bindist"
	"github.com/otelfleet/otelcol-lsp/pkg/otelcfg/distro"
	"github.com/otelfleet/otelcol-lsp/pkg/otelcfg/lspbridge"
	"github.com/otelfleet/otelfleet/pkg/config"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
)

type Server struct {
	l *slog.Logger
	c *config.LSPConfig
	services.Service

	lspHandler atomic.Pointer[http.HandlerFunc]
}

var _ services.Service = (*Server)(nil)
var _ otelfleet_svc.HTTPExtension = (*Server)(nil)

func NewLSPServer(
	l *slog.Logger,
	c *config.LSPConfig,
) *Server {
	s := &Server{
		l:          l,
		c:          c,
		lspHandler: atomic.Pointer[http.HandlerFunc]{},
	}
	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) start(ctx context.Context) error {
	cache := bindist.NewFSCache(s.c.DistCache)

	s.l.With("cacheDir", s.c.DistCache).Info("indexing component catalog from cached binaries")
	catalog, err := lspbridge.NewComponentCatalog(ctx, cache)
	if err != nil {
		return fmt.Errorf("component catalog unavailable: %w", err)
	}

	opts := []lsp.Option{
		lsp.WithDistributionResolver(func(docURI string) types.Distribution {
			if d := distro.FromDocumentURI(docURI); !d.IsZero() {
				return d
			}
			return distro.New(s.c.DefaultDistributionName, s.c.DefaultDistributionVersion)
		}),
		lsp.WithValidator(lspbridge.StructuralValidator(catalog)),
		lsp.WithValidator(lspbridge.RuntimeValidator(cache)),
	}

	otelcolLSP := lsp.New(s.l, opts...)

	handler := lspws.NewHandler(s.l, websocket.Upgrader{
		// TODO:
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}, otelcolLSP)

	handlerFuncPTr := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})

	s.lspHandler.Store(&handlerFuncPTr)
	return nil
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) stop(error) error {
	return nil
}

func (s *Server) ConfigureHTTP(mux *mux.Router, _ []connect.HandlerOption) {
	mux.HandleFunc("/lsp", func(w http.ResponseWriter, r *http.Request) {
		handler := s.lspHandler.Load()
		if handler == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("lsp not initialized"))
			return
		}
		(*handler).ServeHTTP(w, r)
	})
}
