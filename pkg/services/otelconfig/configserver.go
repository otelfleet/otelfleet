package otelconfig

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ConfigServer struct {
	configStore storage.KeyValue[*v1alpha1.Config]
	logger      *slog.Logger

	services.Service
}

var _ v1alpha1connect.ConfigServiceHandler = (*ConfigServer)(nil)

func NewConfigServer(
	logger *slog.Logger,
	configStore storage.KeyValue[*v1alpha1.Config],
) *ConfigServer {
	cs := &ConfigServer{
		logger:      logger,
		configStore: configStore,
	}
	cs.Service = services.NewBasicService(nil, cs.running, nil)
	return cs
}

func (c *ConfigServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *ConfigServer) ConfigureHTTP(mux *mux.Router) {
	c.logger.Info("configuring routes")
	v1alpha1connect.RegisterConfigServiceHandler(mux, c)
}

func (c *ConfigServer) ValidConfig(context.Context, *connect.Request[v1alpha1.ValidateConfigRequest]) (*connect.Response[emptypb.Empty], error) {
	return connect.NewResponse(&emptypb.Empty{}), nil
}
func (c *ConfigServer) UpdateConfig(ctx context.Context, connectReq *connect.Request[v1alpha1.UpdateConfigRequest]) (*connect.Response[emptypb.Empty], error) {
	req := connectReq.Msg

	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config must be non-empty")
	}
	if req.GetRef().GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "config key must be non-empty")
	}
	err := c.configStore.Put(ctx, req.GetRef().GetId(), req.GetConfig())
	return connect.NewResponse(&emptypb.Empty{}), err
}

func (c *ConfigServer) GetConfig(ctx context.Context, connectReq *connect.Request[v1alpha1.ConfigReference]) (*connect.Response[v1alpha1.Config], error) {
	req := connectReq.Msg

	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "config key must be non-empty")
	}
	config, err := c.configStore.Get(ctx, req.GetId())
	return connect.NewResponse(config), err
}

func (c *ConfigServer) DeleteConfig(ctx context.Context, connectReq *connect.Request[v1alpha1.ConfigReference]) (*connect.Response[emptypb.Empty], error) {
	req := connectReq.Msg
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "config key must be non-empty")
	}

	return connect.NewResponse(&emptypb.Empty{}), c.configStore.Delete(ctx, req.GetId())
}

// ListConfigs by matchers
func (c *ConfigServer) ListConfigs(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1.ListConfigReponse], error) {
	resp := &v1alpha1.ListConfigReponse{}

	configs, err := c.configStore.List(ctx)
	if err != nil {
		return nil, err
	}
	resp.Configs = configs
	return connect.NewResponse(resp), nil
}

func (c *ConfigServer) SetDefaultConfig(context.Context, *connect.Request[v1alpha1.UpdateConfigRequest]) (*connect.Response[emptypb.Empty], error) {
	panic("implement me")
}
