package sync

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	routev1alpha1 "github.com/otelfleet/otelfleet/pkg/api/route/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	"github.com/otelfleet/otelfleet/pkg/router"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	stypes "github.com/otelfleet/otelfleet/pkg/storage/types"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO : replace with watch
const DefaultConfigFilterSyncInterval = 1 * time.Second

type AgentLabels struct {
	Identifying    map[string]string
	NonIdentifying map[string]string
	Otelfleet      map[string]string
}

type ConfigFilterSyncOptions struct {
	Logger   *slog.Logger
	Storage  schema.SchemaProto
	Interval time.Duration
}

type ConfigFilterSync struct {
	ConfigFilterSyncOptions

	mgr             deployment.Manager
	assignedConfigs stypes.KeyValue[*configv1alpha1.AssignedConfig]

	routerKV stypes.KeyValue[*routev1alpha1.Router]

	curRouter atomic.Pointer[router.Matcher]
}

func NewConfigFilterSync(opts ConfigFilterSyncOptions) *ConfigFilterSync {
	s := &ConfigFilterSync{
		ConfigFilterSyncOptions: opts,
		routerKV:                storage.NewProtoKVFromSchemaImpl[*routev1alpha1.Router](opts.Storage),
	}

	s.curRouter.Store(nil)
	return s
}

func (s *ConfigFilterSync) Start(ctx context.Context) error {
	// TODO : we should definitely do a backoff.Retry here for starting.
	syncErr := s.sync(ctx)
	if grpcutil.IsError(codes.NotFound, syncErr) {

		// TODO : should we bootstrap a router when its not found?
		// TODO : do we need to enforce a default global config?
		return nil
	}
	return syncErr
}

func (s *ConfigFilterSync) Stopping() {
}

func (s *ConfigFilterSync) Running(ctx context.Context) error {
	// TODO : this should be a a watch
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := s.sync(ctx); err != nil {
				s.Logger.With("err", err).Error("failed to sync config filters")
			}
		}
	}
}

func (s *ConfigFilterSync) sync(ctx context.Context) error {
	pbRouter, err := s.routerKV.Get(ctx, "global")
	if err != nil {
		return err
	}

	matcher, err := router.NewMatcher(pbRouter)
	if err != nil {
		return err
	}
	s.curRouter.Store(&matcher)

	return nil

}

func (s *ConfigFilterSync) Match(ctx context.Context, labels router.CollectorLabels) (configRef string, err error) {
	matcher := s.curRouter.Load()
	if matcher == nil {
		return "", status.Error(codes.NotFound, "no router loaded")
	}
	m := *matcher
	return m.Match(ctx, labels), nil
}
