package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"github.com/gorilla/mux"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type HTTPRegistrar interface {
	Router() *mux.Router
	ConnectOptions() []connect.HandlerOption
	Handle(pattern string, handler http.Handler) *mux.Route
	HandleFunc(pattern string, handler http.HandlerFunc) *mux.Route
	HandlePrefix(prefix string, handler http.Handler) *mux.Route
}

type GRPCRegistrar interface {
	RegisterService(desc *grpc.ServiceDesc, impl any)
}

type HTTPInstrumentation struct {
	router         *mux.Router
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	connectOptions []connect.HandlerOption
	connectTracing connect.Interceptor
}

func NewHTTPInstrumentation(router *mux.Router, logger *slog.Logger, tp trace.TracerProvider, opts ...connect.HandlerOption) (*HTTPInstrumentation, error) {
	otelInterceptor, err := otelconnect.NewInterceptor(
		otelconnect.WithTracerProvider(tp),
		otelconnect.WithoutMetrics(),
		otelconnect.WithoutServerPeerAttributes(),
	)
	if err != nil {
		return nil, err
	}
	return &HTTPInstrumentation{
		router: router, logger: logger, tracerProvider: tp,
		connectOptions: append([]connect.HandlerOption(nil), opts...),
		connectTracing: otelInterceptor,
	}, nil
}

func (i *HTTPInstrumentation) ForService(name string) HTTPRegistrar {
	logger := i.logger.With("service", name)
	// Tracing and logging are outermost so rejected requests are observable and
	// the request logger can include the active span IDs.
	opts := []connect.HandlerOption{connect.WithInterceptors(
		i.connectTracing,
		connectLoggingInterceptor{logger: logger},
	)}
	opts = append(opts, i.connectOptions...)
	return &httpRegistrar{router: i.router, logger: logger, tracerProvider: i.tracerProvider, connectOptions: opts}
}

type httpRegistrar struct {
	router         *mux.Router
	logger         *slog.Logger
	tracerProvider trace.TracerProvider
	connectOptions []connect.HandlerOption
}

func (r *httpRegistrar) Router() *mux.Router { return r.router }
func (r *httpRegistrar) ConnectOptions() []connect.HandlerOption {
	return append([]connect.HandlerOption(nil), r.connectOptions...)
}
func (r *httpRegistrar) Handle(pattern string, handler http.Handler) *mux.Route {
	return r.router.Handle(pattern, r.wrapHTTP(handler))
}
func (r *httpRegistrar) HandleFunc(pattern string, handler http.HandlerFunc) *mux.Route {
	return r.Handle(pattern, handler)
}
func (r *httpRegistrar) HandlePrefix(prefix string, handler http.Handler) *mux.Route {
	return r.router.PathPrefix(prefix).Handler(r.wrapHTTP(handler))
}
func (r *httpRegistrar) wrapHTTP(next http.Handler) http.Handler {
	logged := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		started := time.Now()
		logger := requestLogger(r.logger, req.Context()).With("method", req.Method, "path", req.URL.Path)
		req = req.WithContext(logutil.WithContext(req.Context(), logger))
		next.ServeHTTP(w, req)
		logger.Debug("request completed", "duration", time.Since(started))
	})
	return otelhttp.NewHandler(logged, "http.server",
		otelhttp.WithTracerProvider(r.tracerProvider),
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			if route := mux.CurrentRoute(req); route != nil {
				if template, err := route.GetPathTemplate(); err == nil {
					return req.Method + " " + template
				}
			}
			return req.Method + " " + req.URL.Path
		}),
	)
}

type connectLoggingInterceptor struct{ logger *slog.Logger }

func (i connectLoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		started := time.Now()
		logger := requestLogger(i.logger, ctx).With("procedure", req.Spec().Procedure)
		resp, err := next(logutil.WithContext(ctx, logger), req)
		logger.Debug("request completed", "duration", time.Since(started), "code", connect.CodeOf(err))
		return resp, err
	}
}
func (i connectLoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (i connectLoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		started := time.Now()
		logger := requestLogger(i.logger, ctx).With("procedure", conn.Spec().Procedure)
		err := next(logutil.WithContext(ctx, logger), conn)
		logger.Debug("request completed", "duration", time.Since(started), "code", connect.CodeOf(err))
		return err
	}
}
func requestLogger(logger *slog.Logger, ctx context.Context) *slog.Logger {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		return logger.With("trace_id", spanContext.TraceID().String(), "span_id", spanContext.SpanID().String())
	}
	return logger
}

type HTTPService interface {
	ConfigureHTTP(HTTPRegistrar)
}

type GRPCInstrumentation struct{ server *grpc.Server }

func NewGRPCInstrumentation(server *grpc.Server) *GRPCInstrumentation {
	return &GRPCInstrumentation{server: server}
}
func (i *GRPCInstrumentation) Registrar() GRPCRegistrar { return i }
func (i *GRPCInstrumentation) RegisterService(desc *grpc.ServiceDesc, impl any) {
	i.server.RegisterService(desc, impl)
}

type GRPCService interface {
	ConfigureGRPC(GRPCRegistrar)
}
