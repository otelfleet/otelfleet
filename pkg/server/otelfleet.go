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
	"sort"
	"strconv"
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
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	logutil "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/services/agent"
	"github.com/otelfleet/otelfleet/pkg/services/authorization"
	"github.com/otelfleet/otelfleet/pkg/services/lsp"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/services/otlp"
	"github.com/otelfleet/otelfleet/pkg/services/resource"
	storagesvc "github.com/otelfleet/otelfleet/pkg/services/storage"
	"github.com/otelfleet/otelfleet/pkg/services/ui"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
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
	Auth          = "authorization"
	ServerService = "server"
	OpAmp         = "opamp"
	ConfigOTEL    = "config-otel"
	AgentManager  = "agent-manager"
	// DeploymentModule = "deployment"
	LSP = "lsp"
	// UI serves the web UI. Attached to the all-in-one target only.
	UI = "ui"
	// Embedded OTLP service
	OTLP = "otlp"
	// Resource server is the API over the first class resources in the data layer.
	Resource = "resource"
	// Gatewat acts as the control plane service. This is the public
	// entry point for all other services, whether other services
	// run in-process or not
	Gateway = "gateway"
)

type OtelFleet struct {
	logger *slog.Logger
	cfg    config.Config

	mm   *modules.Manager
	deps map[string][]string

	store           *storagesvc.StorageService
	tokenStore      types.KeyValue[*bootstrapv1alpha1.BootstrapToken]
	agentStore      types.KeyValue[*agentsv1alpha1.AgentDescription]
	opampAgentStore types.KeyValue[*protobufs.AgentToServer]

	agentHealthStore       types.KeyValue[*protobufs.ComponentHealth]
	agentEffectiveConfig   types.KeyValue[*protobufs.EffectiveConfig]
	agentRemoteConfigStore types.KeyValue[*protobufs.RemoteConfigStatus]
	opampAgentDescription  types.KeyValue[*protobufs.AgentDescription]

	// store for raw configs
	configStore types.KeyValue[*configv1alpha1.Config]
	// store for config filters
	configFilterStore types.KeyValue[*configv1alpha1.ConfigFilter]
	// store for default configs
	defaultConfigStore types.KeyValue[*configv1alpha1.Config]
	// store for bootstrap configs
	// tokenID -> config
	bootstrapConfigStore types.KeyValue[*configv1alpha1.Config]
	// store for associating configs to agents
	// otelfleet agentID -> config
	assignmentConfigStore types.KeyValue[*configv1alpha1.Config]
	// store for config assignment metadata
	// otelfleet agentID -> ConfigAssignment
	configAssignmentStore types.KeyValue[*configv1alpha1.ConfigAssignment]

	// store for deployment status
	deploymentStore types.KeyValue[*configv1alpha1.DeploymentStatus]
	// store for per-agent deployment status
	agentDeploymentStore types.KeyValue[*configv1alpha1.AgentDeploymentStatus]
	// store for persisted connection state (replaces in-memory agentTracker)
	connectionStateStore types.KeyValue[*agentsv1alpha1.AgentConnectionState]

	// Agent repository - unified access to agent data
	agentRepo agentdomain.Repository

	opampServer  *opamp.Server
	configServer *otelconfig.ConfigServer

	serviceMap map[string]services.Service
	server     *server.Server
	serverConf server.Config
}

func New(cfg config.Config) (*OtelFleet, error) {
	l := slog.Default()
	f := &OtelFleet{
		logger: l,
		cfg:    cfg,
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
	mm.RegisterModule(Gateway, nil)

	mm.RegisterModule(Storage, func() (services.Service, error) {
		storeSvc, err := storagesvc.NewStorageService(
			o.logger.With("service", Storage),
			o.cfg.StorageConfig,
		)
		if err != nil {
			return nil, err
		}
		o.store = storeSvc
		storeSvc.ConfigureHTTP(o.server.HTTP)
		o.opampAgentStore = storage.NewProtoKVFromSchemaImpl[*protobufs.AgentToServer](o.store.Schema())

		o.agentStore = storage.NewProtoKVFromSchemaImpl[*agentsv1alpha1.AgentDescription](o.store.Schema())

		o.tokenStore = storage.NewProtoKVFromSchemaImpl[*bootstrapv1alpha1.BootstrapToken](o.store.Schema())

		// dedupe: all *configv1alpha1.Config stores share one underlying store
		// TODO : figure out best to decouple these
		configStore := storage.NewProtoKVFromSchemaImpl[*configv1alpha1.Config](o.store.Schema())
		o.configStore = configStore
		o.defaultConfigStore = configStore
		o.bootstrapConfigStore = configStore
		o.assignmentConfigStore = configStore

		o.agentHealthStore = storage.NewProtoKVFromSchemaImpl[*protobufs.ComponentHealth](o.store.Schema())
		o.agentEffectiveConfig = storage.NewProtoKVFromSchemaImpl[*protobufs.EffectiveConfig](o.store.Schema())
		o.agentRemoteConfigStore = storage.NewProtoKVFromSchemaImpl[*protobufs.RemoteConfigStatus](o.store.Schema())

		o.opampAgentDescription = storage.NewProtoKVFromSchemaImpl[*protobufs.AgentDescription](o.store.Schema())
		o.configAssignmentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.ConfigAssignment](o.store.Schema())
		o.deploymentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.DeploymentStatus](o.store.Schema())
		o.agentDeploymentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.AgentDeploymentStatus](o.store.Schema())
		o.connectionStateStore = storage.NewProtoKVFromSchemaImpl[*agentsv1alpha1.AgentConnectionState](o.store.Schema())

		// Create the agent repository with all the underlying stores
		o.agentRepo = agentdomain.NewRepository(
			o.logger.With("component", "agent-repository"),
			o.agentStore,
			o.opampAgentDescription,
			o.connectionStateStore,
			o.agentHealthStore,
			o.agentEffectiveConfig,
			o.agentRemoteConfigStore,
			o.configAssignmentStore,
		)

		return storeSvc, nil
	}, modules.UserInvisibleModule)

	mm.RegisterModule(Auth, func() (services.Service, error) {
		bootstrapSvc := authorization.NewBootstrapServer(
			o.logger.With("service", Auth),
			nil, // TODO: privateKey for secure bootstrap
			o.tokenStore,
			o.agentRepo,
			o.configStore,
			o.bootstrapConfigStore,
			o.assignmentConfigStore,
		)
		bootstrapSvc.ConfigureHTTP(o.server.HTTP)

		return bootstrapSvc, nil
	})

	mm.RegisterModule(ConfigOTEL, func() (services.Service, error) {
		cfgServer, err := otelconfig.NewConfigServer(
			o.logger.With("service", ConfigOTEL),
			o.configFilterStore,
		)
		if err != nil {
			return nil, err
		}
		cfgServer.ConfigureHTTP(o.server.HTTP)
		o.configServer = cfgServer

		return cfgServer, nil
	})

	mm.RegisterModule(OpAmp, func() (services.Service, error) {
		srv := opamp.NewServer(
			o.logger.With("service", OpAmp),
			o.agentRepo,
			o.assignmentConfigStore,
			o.server.HTTPListenAddr().String(),
			o.cfg.OTLP,
		)
		o.opampServer = srv
		// Wire up the config change notifier so ConfigServer can push configs to agents
		// if o.configServer != nil {
		// 	o.configServer.SetNotifier(srv)
		// }
		return srv, nil
	})

	mm.RegisterModule(AgentManager, func() (services.Service, error) {
		srv := agent.NewAgentServer(
			o.logger.With("service", AgentManager),
			o.agentRepo,
		)
		srv.ConfigureHTTP(o.server.HTTP)
		return srv, nil
	})

	// mm.RegisterModule(DeploymentModule, func() (services.Service, error) {
	// ctrl := deployment.NewController(
	// 	o.logger.With("service", DeploymentModule),
	// 	o.deploymentStore,
	// 	o.agentDeploymentStore,
	// 	o.configStore,
	// 	o.agentRepo,
	// )
	// o.deploymentController = ctrl
	// // Wire up the config assigner so the deployment controller can assign configs
	// if o.configServer != nil {
	// 	ctrl.SetConfigAssigner(o.configServer)
	// 	o.configServer.SetDeploymentController(ctrl)
	// }
	// return ctrl, nil
	// })

	mm.RegisterModule(UI, func() (services.Service, error) {
		uiSvc, err := ui.NewUIService(
			o.logger.With("service", UI),
			o.cfg.UI,
		)
		if err != nil {
			return nil, err
		}
		uiSvc.ConfigureHTTP(o.server.HTTP)
		return uiSvc, nil
	})

	mm.RegisterModule(OTLP, func() (services.Service, error) {
		otlpSvc := otlp.NewServer(o.logger.With("service", "otlp"), o.cfg.OTLP)
		otlpSvc.ConfigureGRPC(o.server.GRPC)
		otlpSvc.ConfigureHTTP(o.server.HTTP)
		return otlpSvc, nil
	})

	mm.RegisterModule(Resource, func() (services.Service, error) {
		resourceSvc := resource.NewServer(o.logger.With("service", "resource-server"), o.store.Schema())
		resourceSvc.ConfigureHTTP(o.server.HTTP)
		return resourceSvc, nil
	})

	mm.RegisterModule(LSP, func() (services.Service, error) {
		lspService := lsp.NewLSPServer(
			o.logger.With("service", "lsp"),
			o.cfg.LSP,
		)
		lspService.ConfigureHTTP(o.server.HTTP)
		return lspService, nil
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
			Gateway, UI,
		},
		Gateway: {
			Auth, OpAmp, AgentManager, OTLP, Resource, LSP,
		},
		ServerService: {},

		Storage:      {ServerService},
		AgentManager: {ServerService, OpAmp},
		OpAmp:        {ServerService, ConfigOTEL, Storage},
		Auth:         {ServerService, Storage},
		ConfigOTEL:   {ServerService, Storage},
		Resource:     {ServerService, Storage},
		UI:           {ServerService},
		OTLP:         {ServerService},
		LSP:          {ServerService},
		// DeploymentModule: {ServerService, ConfigOTEL, Storage},
	}

	for mod, targets := range deps {
		if err := mm.AddDependency(mod, targets...); err != nil {
			return err
		}
	}

	o.mm = mm
	o.deps = deps
	for _, curSvc := range o.cfg.Services {
		curDeps := o.mm.DependenciesForModule(curSvc)
		for _, m := range o.mm.UserVisibleModuleNames() {
			ix := sort.SearchStrings(curDeps, m)
			included := ix < len(curDeps) && curDeps[ix] == m

			if included {
				fmt.Fprintln(os.Stdout, m, "*")
			} else {
				fmt.Fprintln(os.Stdout, m)
			}
		}

		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, fmt.Sprintf("Modules marked with * are included in target %s.", curSvc))
	}
	return nil
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
