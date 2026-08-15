package otelconfig

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ConfigServer struct {
	services.Service

	logger      *slog.Logger
	filterStore types.KeyValue[*v1alpha1.ConfigFilter]
}

func NewConfigServer(
	logger *slog.Logger,
	filterStore types.KeyValue[*v1alpha1.ConfigFilter],
) (*ConfigServer, error) {
	srv := &ConfigServer{
		logger:      logger,
		filterStore: filterStore,
	}
	srv.Service = services.NewBasicService(srv.start, srv.running, srv.stopping)
	return srv, nil
}

var _ services.Service = (*ConfigServer)(nil)
var _ configv1alpha1.ConfigServiceHandler = (*ConfigServer)(nil)

func (c *ConfigServer) start(ctx context.Context) error {
	return nil
}

func (c *ConfigServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *ConfigServer) stopping(error) error {
	return nil
}

func (c *ConfigServer) validateRef(ref *v1alpha1.ConfigFilterReference) (string, error) {
	id := ref.GetId()
	if id == "" {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("config filter id is required"))
	}
	return id, nil
}

func (c *ConfigServer) PutConfigFilter(ctx context.Context, req *connect.Request[v1alpha1.PutConfigFilterRequest]) (*connect.Response[emptypb.Empty], error) {
	id, err := c.validateRef(req.Msg.GetRef())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetFilter() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("filter is required"))
	}

	if err := c.filterStore.Put(ctx, id, req.Msg.GetFilter()); err != nil {
		c.logger.With("id", id, "err", err).Error("failed to put config filter")
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *ConfigServer) GetConfigFilter(ctx context.Context, req *connect.Request[v1alpha1.ConfigFilterReference]) (*connect.Response[v1alpha1.ConfigFilter], error) {
	id, err := c.validateRef(req.Msg)
	if err != nil {
		return nil, err
	}

	filter, err := c.filterStore.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(filter), nil
}

func (c *ConfigServer) DeleteConfigFilter(ctx context.Context, req *connect.Request[v1alpha1.ConfigFilterReference]) (*connect.Response[emptypb.Empty], error) {
	id, err := c.validateRef(req.Msg)
	if err != nil {
		return nil, err
	}

	if err := c.filterStore.Delete(ctx, id); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *ConfigServer) ListConfigFilters(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1.ListConfigFiltersResponse], error) {
	keys, err := c.filterStore.ListKeys(ctx)
	if err != nil {
		return nil, err
	}

	refs := make([]*v1alpha1.ConfigFilterReference, 0, len(keys))
	for _, key := range keys {
		refs = append(refs, &v1alpha1.ConfigFilterReference{Id: key})
	}
	return connect.NewResponse(&v1alpha1.ListConfigFiltersResponse{
		Filters: refs,
	}), nil
}

func (c *ConfigServer) ConfigureHTTP(mux *mux.Router) {
	configv1alpha1.RegisterConfigServiceHandler(mux, c)
}
