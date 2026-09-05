package server

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/grafana/dskit/middleware"
	"github.com/grafana/dskit/modules"
	"github.com/grafana/dskit/services"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	eventv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/auth/authenticator"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/authorization"
	deployment_svc "github.com/otelfleet/otelfleet/pkg/services/deployment"
	"github.com/otelfleet/otelfleet/pkg/services/event"
	"github.com/otelfleet/otelfleet/pkg/services/lsp"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/services/otlp"
	"github.com/otelfleet/otelfleet/pkg/services/resource"
	storagesvc "github.com/otelfleet/otelfleet/pkg/services/storage"
	"github.com/otelfleet/otelfleet/pkg/services/ui"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	su "github.com/otelfleet/otelfleet/pkg/util/serviceutil"
	"github.com/rs/cors"
	"github.com/samber/lo"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	eventsink "github.com/otelfleet/otelfleet/pkg/event"
)

func (o *OtelFleet) configureExtensions(name su.Module, svc services.Service) {
	httpExt, ok := svc.(otelfleet_svc.HTTPService)
	if ok {
		httpExt.ConfigureHTTP(o.httpInstrumentation.ForService(name))
	}
	grpcExt, ok := svc.(otelfleet_svc.GRPCService)
	if ok {
		grpcExt.ConfigureGRPC(o.grpcInstrumentation.Registrar())
	}
}

// Wraps module manager register module, and automatically configures extra
// server-side functionality based on available interface implementations
func (o *OtelFleet) RegisterModule(name su.Module, initFn func() (services.Service, error)) {
	o.mm.RegisterModule(name.Str(), func() (services.Service, error) {
		svc, err := initFn()
		if err != nil {
			return nil, err
		}
		o.configureExtensions(name, svc)
		return svc, nil
	})
}

// Same as RegisterModule, but invisible to users
func (o *OtelFleet) RegisterModuleInvisible(name su.Module, initFn func() (services.Service, error)) {
	o.mm.RegisterModule(name.Str(), func() (services.Service, error) {
		svc, err := initFn()
		if err != nil {
			return nil, err
		}
		o.configureExtensions(name, svc)
		return svc, nil
	}, modules.UserInvisibleModule)
}

func (o *OtelFleet) setupModuleManager() error {
	mm := modules.NewManager(o.serverConf.Log)
	mm.RegisterModule(su.All.Str(), nil)
	mm.RegisterModule(su.Gateway.Str(), nil)
	o.mm = mm

	o.RegisterModuleInvisible(su.Storage, func() (services.Service, error) {
		clientOpts, err := otelfleet_svc.ConnectClientOptions(o.tracers.Provider(su.Storage))
		if err != nil {
			return nil, fmt.Errorf("configure storage client instrumentation: %w", err)
		}
		storeSvc, err := storagesvc.NewStorageService(
			o.logger.With("service", su.Storage.Str()),
			o.cfg.StorageConfig,
			clientOpts...,
		)
		if err != nil {
			return nil, err
		}
		o.store = storeSvc
		o.tokenStore = object.NewKeyValueAdapter[*bootstrapv1alpha1.BootstrapToken](o.store.Schema())
		o.deployMgr = deployment.NewManager(o.store.Schema())

		return storeSvc, nil
	})

	o.RegisterModule(su.Auth, func() (services.Service, error) {
		bootstrapSvc := authorization.NewBootstrapServer(
			nil, // TODO: privateKey for secure bootstrap
			o.tokenStore,
		)
		// TODO : auth should be part of the auth service, currently it's in memory / hacky
		o.autenticator = authenticator.NewNoop()
		return bootstrapSvc, nil
	})

	o.RegisterModule(su.OpAmp, func() (services.Service, error) {
		srv := opamp.NewServer(
			o.logger.With("service", su.OpAmp.Str()),
			o.store.Schema(),
			o.server.HTTPListenAddr().String(),
			o.cfg.OTLP,
			o.deployMgr,
			eventsink.NewEventSink(object.NewKeyValueAdapter[*eventv1alpha1.Event](o.store.Schema()), eventsink.GroupCollector),
			o.cfg.Certificates,
			o.autenticator,
		)
		o.opampServer = srv
		return srv, nil
	})

	o.RegisterModule(su.DeploymentManager, func() (services.Service, error) {
		srv := deployment_svc.NewDeploymentServer(o.deployMgr)
		return srv, nil
	})

	o.RegisterModule(su.UI, func() (services.Service, error) {
		uiSvc, err := ui.NewUIService(
			o.cfg.UI,
		)
		if err != nil {
			return nil, err
		}
		return uiSvc, nil
	})

	o.RegisterModule(su.OTLP, func() (services.Service, error) {
		otlpSvc := otlp.NewServer(
			o.cfg.OTLP,
			o.autenticator,
		)
		return otlpSvc, nil
	})

	o.RegisterModule(su.Resource, func() (services.Service, error) {
		resourceSvc := resource.NewServer(o.store.Schema())
		return resourceSvc, nil
	})

	o.RegisterModule(su.Events, func() (services.Service, error) {
		// FIXME: for now let's put the event querier on the same API
		// path as generic control plane resources API.
		eventSvc := event.NewServer(
			object.NewKeyValueAdapter[*eventv1alpha1.Event](o.store.Schema()),
		)
		return eventSvc, nil
	})

	o.RegisterModule(su.LSP, func() (services.Service, error) {
		lspService := lsp.NewLSPServer(
			o.logger.With("service", "lsp"),
			o.cfg.LSP,
		)
		return lspService, nil
	})

	o.RegisterModuleInvisible(su.ServerService, func() (services.Service, error) {
		servicesToWaitFor := func() []services.Service {
			svs := []services.Service(nil)
			for m, s := range o.serviceMap {
				// Server should not wait for itself.
				if m != su.ServerService.Str() {
					svs = append(svs, s)
				}
			}
			return svs
		}
		defaultHTTPMiddleware := []middleware.Interface{}
		o.server.HTTPServer.Handler = middleware.Merge(defaultHTTPMiddleware...).Wrap(o.server.HTTP)
		s := o.newServerService(servicesToWaitFor)
		corsHandler := cors.New(cors.Options{
			// TODO :
			AllowedOrigins:   []string{"http://localhost:5173"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		}).Handler(o.server.HTTPServer.Handler)
		o.server.HTTPServer.Handler = h2c.NewHandler(corsHandler, &http2.Server{})
		return s, nil
	})

	// Add dependencies
	deps := map[su.Module][]su.Module{
		su.All: {
			su.Gateway, su.UI,
		},
		su.Gateway: {
			su.Auth, su.OpAmp, su.DeploymentManager, su.OTLP, su.Resource, su.Events, su.LSP,
		},
		su.ServerService: {},

		su.Storage:           {su.ServerService},
		su.DeploymentManager: {su.ServerService, su.Storage, su.OpAmp},
		su.OpAmp:             {su.Auth, su.ServerService, su.Storage},
		su.Auth:              {su.ServerService, su.Storage},
		su.Events:            {su.ServerService, su.Storage},
		su.Resource:          {su.ServerService, su.Storage},
		su.UI:                {su.ServerService},
		su.OTLP:              {su.Auth, su.ServerService},
		su.LSP:               {su.ServerService},
	}

	for mod, targets := range deps {
		if err := mm.AddDependency(mod.Str(), lo.Map(targets, func(s su.Module, _ int) string {
			return s.Str()
		})...); err != nil {
			return err
		}
	}

	o.mm = mm
	o.deps = deps
	for _, curSvc := range o.cfg.Services {
		if !o.mm.IsModuleRegistered(curSvc) {
			return fmt.Errorf("unknown service target %q (registered targets: %v)", curSvc, o.mm.UserVisibleModuleNames())
		}
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
		fmt.Fprintf(os.Stdout, "Modules marked with * are included in target %s.\n", curSvc)
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
