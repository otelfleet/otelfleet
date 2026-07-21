package ui

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gorilla/mux"
	"github.com/otelfleet/otelfleet/pkg/config"
)

func newTestService(t *testing.T, cfg *config.UIConfig, assets fstest.MapFS) *UIService {
	t.Helper()
	cfg.Sanitize()
	u := &UIService{logger: slog.Default(), cfg: cfg}
	h, err := u.newEmbeddedHandler(assets)
	if err != nil {
		t.Fatalf("newEmbeddedHandler: %v", err)
	}
	u.handler = h
	return u
}

const testIndex = `<html><head></head><body><div id="root"></div></body></html>`

func TestServesIndexVerbatim(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte(testIndex)}}
	u := newTestService(t, &config.UIConfig{}, assets)

	rr := httptest.NewRecorder()
	u.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rr.Body.String(); got != testIndex {
		t.Fatalf("expected index served verbatim, got: %s", got)
	}
}

func TestSPAFallbackServesIndex(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":    {Data: []byte(testIndex)},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	u := newTestService(t, &config.UIConfig{}, assets)

	// Unknown client-side route -> index.html
	rr := httptest.NewRecorder()
	u.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/agents/123", nil))
	if !strings.Contains(rr.Body.String(), `id="root"`) {
		t.Fatalf("expected SPA fallback to index.html, got: %s", rr.Body.String())
	}

	// Real asset -> served as-is
	rr = httptest.NewRecorder()
	u.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if got := rr.Body.String(); got != "console.log(1)" {
		t.Fatalf("expected asset content, got: %s", got)
	}
}

// When Upstream is set (standalone UI), non-UI paths are proxied to the control
// plane while UI paths keep serving the embedded assets.
func TestStandaloneProxiesAPIPathsToUpstream(t *testing.T) {
	var proxied string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from-control-plane"))
	}))
	defer upstream.Close()

	assets := fstest.MapFS{"index.html": {Data: []byte(testIndex)}}
	u := newTestService(t, &config.UIConfig{ApiURL: upstream.URL}, assets)
	proxy, err := newUpstreamProxy(u.cfg.ApiURL)
	if err != nil {
		t.Fatalf("newUpstreamProxy: %v", err)
	}
	u.proxy = proxy

	router := mux.NewRouter()
	u.mount(router)

	// API RPC path -> proxied upstream.
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/otelfleet.v1.AgentService/List", nil))
	if proxied != "/otelfleet.v1.AgentService/List" {
		t.Fatalf("expected RPC path proxied upstream, got proxied=%q", proxied)
	}
	if got := rr.Body.String(); got != "from-control-plane" {
		t.Fatalf("expected upstream response, got: %s", got)
	}

	// UI path -> embedded assets, not proxied.
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ui/", nil))
	if got := rr.Body.String(); got != testIndex {
		t.Fatalf("expected embedded index for /ui/, got: %s", got)
	}
}
