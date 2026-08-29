package otlp

import (
	"context"
	"log/slog"
	"net/http"
	"path"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/config"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracespb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

const (
	contentTypePb   = "application/x-protobuf"
	contentTypeJSON = "application/json"
)

type Server struct {
	traceServer   *TracesServer
	metricsServer *MetricsServer
	logsServer    *LogsServer

	config *config.OTLPConfig
	services.Service
	authenticator authenticator.Authenticator
}

var _ otelfleet_svc.HTTPExtension = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	config *config.OTLPConfig,
	authenticator authenticator.Authenticator,
) *Server {
	s := &Server{
		traceServer: &TracesServer{
			l: l.With("type", "traces"),
		},
		metricsServer: &MetricsServer{
			l: l.With("type", "metrics"),
		},
		logsServer: &LogsServer{
			l: l.With("type", "logs"),
		},
		config:        config,
		authenticator: authenticator,
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) start(context.Context) error {
	return nil
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) stop(error) error {
	return nil
}

func (s *Server) ConfigureGRPC(srv *grpc.Server) {
	srv.RegisterService(&colmetricspb.MetricsService_ServiceDesc, s.metricsServer)
	srv.RegisterService(&coltracespb.TraceService_ServiceDesc, s.traceServer)
	srv.RegisterService(&collogspb.LogsService_ServiceDesc, s.logsServer)
}

func (s *Server) ConfigureHTTP(mux *mux.Router, _ []connect.HandlerOption) {
	register := func(route string, next http.Handler) {
		next = s.checkAuth(next)
		mux.Handle(route, next)
	}
	register(
		path.Join(s.config.BasePath, s.config.MetricsAPIPath),
		http.HandlerFunc(s.metricsServer.handleMetricsPost),
	)
	register(
		path.Join(s.config.BasePath, s.config.LogsAPIPath),
		http.HandlerFunc(s.logsServer.handleLogsPost),
	)
	register(
		path.Join(s.config.BasePath, s.config.TraceAPIPath),
		http.HandlerFunc(s.traceServer.handleTracePost),
	)
}
