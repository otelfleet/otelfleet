package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dslog "github.com/grafana/dskit/log"
	"github.com/grafana/dskit/middleware"
	"github.com/grafana/dskit/modules"
	"github.com/grafana/dskit/server"
	"github.com/grafana/dskit/services"
	"github.com/grafana/dskit/signals"
	"github.com/open-telemetry/opamp-go/protobufs"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/config"
	logutil "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/agent"
	"github.com/otelfleet/otelfleet/pkg/services/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	storagesvc "github.com/otelfleet/otelfleet/pkg/services/storage"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
	All           = "all"
	Storage       = "storage"
	Bootstrap     = "bootstrap"
	ServerService = "server"
	OpAmp         = "opamp"
	ConfigOTEL    = "config-otel"
	AgentManager  = "agent-manager"
)

type OtelFleet struct {
	logger *slog.Logger
	cfg    config.Config

	mm   *modules.Manager
	deps map[string][]string

	store           storage.KVBroker
	tokenStore      storage.KeyValue[*bootstrapv1alpha1.BootstrapToken]
	agentStore      storage.KeyValue[*agentsv1alpha1.AgentDescription]
	opampAgentStore storage.KeyValue[*protobufs.AgentToServer]

	agentHealthStore       storage.KeyValue[*protobufs.ComponentHealth]
	agentEffectiveConfig   storage.KeyValue[*protobufs.EffectiveConfig]
	agentRemoteConfigStore storage.KeyValue[*protobufs.RemoteConfigStatus]
	opampAgentDescription  storage.KeyValue[*protobufs.AgentDescription]

	// store for raw configs
	configStore storage.KeyValue[*configv1alpha1.Config]
	// store for default configs
	defaultConfigStore storage.KeyValue[*configv1alpha1.Config]
	// store for bootstrap configs
	// tokenID -> config
	bootstrapConfigStore storage.KeyValue[*configv1alpha1.Config]
	// store for associating configs to agents
	// otelfleet agentID -> config
	assignmentConfigStore storage.KeyValue[*configv1alpha1.Config]

	agentTracker opamp.AgentTracker

	serviceMap map[string]services.Service
	server     *server.Server
	serverConf server.Config
}

func New(cfg config.Config) (*OtelFleet, error) {
	l := slog.Default()
	f := &OtelFleet{
		logger:       l,
		cfg:          cfg,
		agentTracker: opamp.NewAgentTracker(),
	}

	conf := server.Config{
		HTTPListenAddress:             "127.0.0.1",
		HTTPListenPort:                8081,
		DoNotAddDefaultHTTPMiddleware: true,
		LogFormat:                     dslog.LogfmtFormat,
		LogLevel: dslog.Level{
			Option: level.AllowInfo(),
		},
	}

	conf.Log = initLogger(conf.LogFormat, conf.LogLevel)

	srv, err := server.New(conf)
	if err != nil {
		return nil, err
	}
	f.server = srv
	f.serverConf = conf

	if err := f.setupModuleManager(); err != nil {
		return nil, err
	}
	return f, nil
}

func (o *OtelFleet) setupModuleManager() error {
	mm := modules.NewManager(o.serverConf.Log)
	mm.RegisterModule(All, nil)

	mm.RegisterModule(Storage, func() (services.Service, error) {
		storeSvc, err := storagesvc.NewStorageService(
			o.logger.With("service", Storage),
			o.cfg.StoragePath,
		)
		if err != nil {
			return nil, err
		}
		o.store = storeSvc
		o.opampAgentStore = storage.NewProtoKV[*protobufs.AgentToServer](
			o.logger.With("store", "opamp-agent"),
			o.store.KeyValue("opamp-agents"),
		)

		o.agentStore = storage.NewProtoKV[*agentsv1alpha1.AgentDescription](
			o.logger.With("store", "agents"),
			o.store.KeyValue("agents"),
		)

		o.tokenStore = storage.NewProtoKV[*bootstrapv1alpha1.BootstrapToken](
			o.logger.With("store", "tokens"),
			o.store.KeyValue("tokens"),
		)

		o.configStore = storage.NewProtoKV[*configv1alpha1.Config](
			o.logger.With("store", "configs"),
			o.store.KeyValue("configs"),
		)

		o.defaultConfigStore = storage.NewProtoKV[*configv1alpha1.Config](
			o.logger.With("store", "default-configs"),
			o.store.KeyValue("defaultconfigs"),
		)

		o.agentHealthStore = storage.NewProtoKV[*protobufs.ComponentHealth](
			o.logger.With("store", "agent-health"),
			o.store.KeyValue("agent-health"),
		)
		o.agentEffectiveConfig = storage.NewProtoKV[*protobufs.EffectiveConfig](
			o.logger.With("store", "agent-effective-config"),
			o.store.KeyValue("agent-effective-config"),
		)
		o.agentRemoteConfigStore = storage.NewProtoKV[*protobufs.RemoteConfigStatus](
			o.logger.With("store", "agent-remote-config-status"),
			o.store.KeyValue("agent-remote-config-status"),
		)

		o.opampAgentDescription = storage.NewProtoKV[*protobufs.AgentDescription](
			o.logger.With("store", "opamp-agent-description"),
			o.store.KeyValue("opamp-agent-description"),
		)
		o.bootstrapConfigStore = storage.NewProtoKV[*configv1alpha1.Config](
			o.logger.With("store", "bootstrap-configs"),
			o.store.KeyValue("bootstrapconfigs"),
		)
		o.assignmentConfigStore = storage.NewProtoKV[*configv1alpha1.Config](
			o.logger.With("store", "assignmentconfigs"),
			o.store.KeyValue("assignmentconfigs"),
		)
		return storeSvc, nil
	}, modules.UserInvisibleModule)

	mm.RegisterModule(Bootstrap, func() (services.Service, error) {
		bootstrapSvc := bootstrap.NewBootstrapServer(
			o.logger.With("service", Bootstrap),
			nil,
			o.tokenStore,
			o.opampAgentStore,
			o.agentStore,
			o.configStore,
			o.bootstrapConfigStore,
		)
		bootstrapSvc.ConfigureHTTP(o.server.HTTP)

		return bootstrapSvc, nil
	})

	mm.RegisterModule(ConfigOTEL, func() (services.Service, error) {
		cfgServer := otelconfig.NewConfigServer(
			o.logger.With("service", ConfigOTEL),
			o.configStore,
			o.defaultConfigStore,
		)
		cfgServer.ConfigureHTTP(o.server.HTTP)

		return cfgServer, nil
	})

	mm.RegisterModule(OpAmp, func() (services.Service, error) {
		srv := opamp.NewServer(
			o.logger.With("service", OpAmp),
			o.opampAgentStore,
			o.agentTracker,
			o.agentHealthStore,
			o.agentEffectiveConfig,
			o.agentRemoteConfigStore,
			o.opampAgentDescription,
		)
		return srv, nil
	})

	mm.RegisterModule(AgentManager, func() (services.Service, error) {
		srv := agent.NewAgentServer(
			o.logger.With("service", AgentManager),
			o.agentStore,
			o.agentTracker,
			o.agentHealthStore,
			o.agentEffectiveConfig,
			o.agentRemoteConfigStore,
			o.opampAgentDescription,
		)
		srv.ConfigureHTTP(o.server.HTTP)
		return srv, nil
	})

	mm.RegisterModule(ServerService, func() (services.Service, error) {
		servicesToWaitFor := func() []services.Service {
			svs := []services.Service(nil)
			for m, s := range o.serviceMap {
				// Server should not wait for itself.
				if m != ServerService {
					svs = append(svs, s)
				}
			}
			return svs
		}
		defaultHTTPMiddleware := []middleware.Interface{}
		o.server.HTTPServer.Handler = middleware.Merge(defaultHTTPMiddleware...).Wrap(o.server.HTTP)
		s := o.newServerService(servicesToWaitFor)
		corsHandler := cors.New(cors.Options{
			AllowedOrigins:   []string{"http://localhost:5173"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		}).Handler(o.server.HTTPServer.Handler)
		o.server.HTTPServer.Handler = h2c.NewHandler(corsHandler, &http2.Server{})

		// o.server.HTTPServer.Handler = util.RecoveryHTTPMiddleware.Wrap(f.Server.HTTPServer.Handler)
		return s, nil
	}, modules.UserInvisibleModule)

	// Add dependencies
	deps := map[string][]string{
		All: {
			ServerService,
		},
		ServerService: {Bootstrap, OpAmp, AgentManager},
		AgentManager:  {OpAmp},
		OpAmp:         {ConfigOTEL, Storage},
		Bootstrap:     {Storage},
		ConfigOTEL:    {Storage},
	}

	for mod, targets := range deps {
		if err := mm.AddDependency(mod, targets...); err != nil {
			return err
		}
	}

	o.mm = mm
	o.deps = deps
	allDeps := o.mm.DependenciesForModule(All)
	for _, m := range o.mm.UserVisibleModuleNames() {
		ix := sort.SearchStrings(allDeps, m)
		included := ix < len(allDeps) && allDeps[ix] == m

		if included {
			fmt.Fprintln(os.Stdout, m, "*")
		} else {
			fmt.Fprintln(os.Stdout, m)
		}
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Modules marked with * are included in target All.")
	return nil
}

func (o *OtelFleet) Run(ctx context.Context) error {
	// FIXME: config driven services
	svcMap, err := o.mm.InitModuleServices(All)
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

// newServerService constructs service from Server component.
// servicesToWaitFor is called when server is stopping, and should return all
// services that need to terminate before server actually stops.
// N.B.: this function is NOT Cortex specific, please let's keep it that way.
// Passed server should not react on signals. Early return from Run function is considered to be an error.
func (o *OtelFleet) newServerService(servicesToWaitFor func() []services.Service) services.Service {
	l := o.logger.With("service", "server")
	serverDone := make(chan error, 1)

	runFn := func(ctx context.Context) error {
		go func() {
			defer close(serverDone)
			rl := l
			if o.serverConf.GRPCListenAddress != "" {
				rl = rl.With("grpc-addr", fmt.Sprintf("%s:%d", o.serverConf.GRPCListenAddress, o.serverConf.GRPCListenPort))
			}
			if o.serverConf.HTTPListenAddress != "" {
				rl = rl.With("http-addr", fmt.Sprintf("%s:%d", o.serverConf.HTTPListenAddress, o.serverConf.HTTPListenPort))
			}
			rl.Info("running")
			serverDone <- o.server.Run()
		}()

		select {
		case <-ctx.Done():
			return nil
		case err := <-serverDone:
			if err != nil {
				return fmt.Errorf("server stopped unexpectedly: %w", err)
			}
			return nil
		}
	}

	stoppingFn := func(_ error) error {
		// wait until all modules are done, and then shutdown server.
		for _, s := range servicesToWaitFor() {
			_ = s.AwaitTerminated(context.Background())
		}

		// shutdown HTTP and gRPC servers (this also unblocks Run)
		o.server.Shutdown()

		// if not closed yet, wait until server stops.
		<-serverDone
		l.Info("server stopped")
		return nil
	}

	return services.NewBasicService(nil, runFn, stoppingFn)
}
