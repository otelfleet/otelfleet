package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"os"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/validate"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dslog "github.com/grafana/dskit/log"
	"github.com/grafana/dskit/modules"
	"github.com/grafana/dskit/server"
	"github.com/grafana/dskit/services"
	"github.com/grafana/dskit/signals"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/config"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	logutil "github.com/otelfleet/otelfleet/pkg/logutil"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	storagesvc "github.com/otelfleet/otelfleet/pkg/services/storage"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/util/connectutil"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initLogger(logFormat string, logLevel dslog.Level) *logger {
	w := logutil.NewAsyncWriter(os.Stderr, // Flush after:
		256<<10, 20, // 256KiB buffer is full (keep 20 buffers).
		1<<10, // 1K writes or 100ms.
		100*time.Millisecond,
	)

	// Use UTC timestamps and skip 5 stack frames.
	l := dslog.NewGoKitWithWriter(logFormat, w)
	l = log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.Caller(5))

	// Must put the level filter last for efficiency.
	l = level.NewFilter(l, logLevel.Option)

	return &logger{w: w, Logger: l}
}

type logger struct {
	w io.WriteCloser
	log.Logger
}

// The various modules that make up OtelFleet
const (
	All               = "all"
	Storage           = "storage"
	Auth              = "authorization"
	ServerService     = "server"
	OpAmp             = "opamp"
	ConfigOTEL        = "config-otel"
	DeploymentManager = "deployment-manager"
	// DeploymentModule = "deployment"
	LSP = "lsp"
	// UI serves the web UI. Attached to the all-in-one target only.
	UI = "ui"
	// Embedded OTLP service
	OTLP = "otlp"
	// Resource server is the API over the first class resources in the data layer.
	Resource = "resource"
	// Events server is the API responsible for listing/watching events.
	Events = "events"
	// Gatewat acts as the control plane service. This is the public
	// entry point for all other services, whether other services
	// run in-process or not
	Gateway = "gateway"
)

type OtelFleet struct {
	logger *slog.Logger
	tp     *sdktrace.TracerProvider
	cfg    config.Config

	mm   *modules.Manager
	deps map[string][]string

	store      *storagesvc.StorageService
	tokenStore object.KeyValue[*bootstrapv1alpha1.BootstrapToken]
	deployMgr  deployment.Manager

	opampServer *opamp.Server

	serviceMap map[string]services.Service
	server     *server.Server
	serverConf server.Config

	autenticator authenticator.Authenticator

	connectOpts         []connect.HandlerOption
	httpInstrumentation *otelfleet_svc.HTTPInstrumentation
	grpcInstrumentation *otelfleet_svc.GRPCInstrumentation
}

func New(cfg config.Config, tp *sdktrace.TracerProvider) (*OtelFleet, error) {
	l := slog.Default()
	f := &OtelFleet{
		logger: l,
		tp:     tp,
		cfg:    cfg,
		connectOpts: []connect.HandlerOption{
			connect.WithInterceptors(validate.NewInterceptor(), connectutil.StatusInterceptor()),
		},
	}

	httpListenHost, httpListenPort, err := net.SplitHostPort(cfg.HttpListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse http_listen_addr : %w", err)
	}
	httpListenPortNum, err := strconv.Atoi(httpListenPort)
	if err != nil {
		return nil, fmt.Errorf("http_listen_addr port is not a number : %w", err)
	}

	grpcListenHost, grpcListenPort, err := net.SplitHostPort(cfg.GRPCListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse grpc_listen_addr : %w", err)
	}

	grpcListenPortNum, err := strconv.Atoi(grpcListenPort)
	if err != nil {
		return nil, fmt.Errorf("grpc_listen_addr port is not a number : %w", err)
	}

	conf := server.Config{
		HTTPListenNetwork:             cfg.HttpListenNetwork,
		HTTPListenAddress:             httpListenHost,
		HTTPListenPort:                httpListenPortNum,
		GRPCListenAddress:             grpcListenHost,
		GRPCListenPort:                grpcListenPortNum,
		DoNotAddDefaultHTTPMiddleware: true,
		LogFormat:                     dslog.LogfmtFormat,
		LogLevel: dslog.Level{
			Option: level.AllowInfo(),
		},
	}
	if cfg.Certificates != nil {
		conf.HTTPTLSConfig = server.TLSConfig{
			TLSCertPath: cfg.Certificates.Server.CertFile,
			TLSKeyPath:  cfg.Certificates.Server.KeyFile,
			ClientAuth:  "VerifyClientCertIfGiven",
			ClientCAs:   cfg.Certificates.Collectors.CaCertFile,
		}
		conf.GRPCTLSConfig = server.TLSConfig{
			TLSCertPath: cfg.Certificates.Server.CertFile,
			TLSKeyPath:  cfg.Certificates.Server.KeyFile,
		}
	}

	conf.Log = initLogger(conf.LogFormat, conf.LogLevel)

	srv, err := server.New(conf)
	if err != nil {
		return nil, err
	}
	f.server = srv
	f.serverConf = conf
	f.httpInstrumentation, err = otelfleet_svc.NewHTTPInstrumentation(srv.HTTP, l, tp, f.connectOpts...)
	if err != nil {
		return nil, fmt.Errorf("configure HTTP instrumentation: %w", err)
	}
	f.grpcInstrumentation = otelfleet_svc.NewGRPCInstrumentation(srv.GRPC)

	if err := f.setupModuleManager(); err != nil {
		return nil, err
	}
	return f, nil
}

func (o *OtelFleet) Run(ctx context.Context) error {
	svcMap, err := o.mm.InitModuleServices(o.cfg.Services...)
	if err != nil {
		return err
	}
	o.serviceMap = svcMap

	mgr, err := services.NewManager(slices.Collect(maps.Values(svcMap))...)
	if err != nil {
		o.logger.With("err", err).Error("failed to start service manager")
		return err
	}

	servicesFailed := func(service services.Service) {
		mgr.StopAsync()

		for m, s := range svcMap {
			if s == service {
				if service.FailureCase() == modules.ErrStopProcess {
					o.logger.With(
						"module", m,
					).With(
						"error", service.FailureCase(),
					).Info("received stop signal via return error")
				} else {
					o.logger.With(
						"module", m,
					).With(
						"error", service.FailureCase(),
					).Error("module failed")
				}
				return
			}
		}
		o.logger.With("module", "unknown").With("error", service.FailureCase()).Error("module failed")
	}

	mgr.AddListener(services.NewManagerListener(
		func() {},
		func() {},
		servicesFailed,
	))

	handler := signals.NewHandler(o.serverConf.Log)
	go func() {
		handler.Loop()
		mgr.StopAsync()
	}()
	printRoutes(o.server.HTTP, o.logger)
	var stopErr error
	if err := mgr.StartAsync(ctx); err == nil {
		stopErr = mgr.AwaitStopped(ctx)
	}

	if stopErr != nil {
		return stopErr
	}

	if failed := mgr.ServicesByState()[services.Failed]; len(failed) > 0 {
		for _, f := range failed {
			if f.FailureCase() != modules.ErrStopProcess {
				// Details were reported via failure listener before
				return fmt.Errorf("services failed")
			}
		}
	}
	return nil
}
