package deployment

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1/v1alpha1connect"
	routev1alpha1 "github.com/otelfleet/otelfleet/pkg/api/route/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	"github.com/otelfleet/otelfleet/pkg/router"
	otelfleetsvc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/util"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DeploymentServer struct {
	logger *slog.Logger

	mgr deployment.Manager

	services.Service
}

var _ v1alpha1connect.CollectorServiceHandler = (*DeploymentServer)(nil)
var _ otelfleetsvc.HTTPExtension = (*DeploymentServer)(nil)

func NewDeploymentServer(
	logger *slog.Logger,
	mgr deployment.Manager,
) *DeploymentServer {
	a := &DeploymentServer{
		logger: logger,
		mgr:    mgr,
	}
	a.Service = services.NewBasicService(nil, a.running, nil)
	return a
}

func (a *DeploymentServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (a *DeploymentServer) ConfigureHTTP(mux *mux.Router, opts []connect.HandlerOption) {
	a.logger.Info("configuring routes")
	v1alpha1connect.RegisterCollectorServiceHandler(mux, a, opts...)
}

func (a *DeploymentServer) ListCollectors(
	ctx context.Context, req *connect.Request[v1alpha1.ListCollectorsRequest],
) (*connect.Response[v1alpha1.ListCollectorsResponse], error) {
	collectors, err := a.mgr.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list collectors: %w", err))
	}

	a.logger.With("numCollectors", len(collectors)).Debug("found collectors")

	descAndStatus := make([]*v1alpha1.CollectorDescriptionAndStatus, 0, len(collectors))
	for _, domainCollector := range collectors {
		desc, err := domainCollector.GetDescription(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector description: %w", err))
		}
		entry := &v1alpha1.CollectorDescriptionAndStatus{Collector: desc}
		if req.Msg.GetWithStatus() {
			st, err := domainCollector.Status(ctx)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector status: %w", err))
			}
			entry.Status = st
		}
		descAndStatus = append(descAndStatus, entry)
	}

	return connect.NewResponse(&v1alpha1.ListCollectorsResponse{
		Collectors: descAndStatus,
	}), nil
}

func (a *DeploymentServer) GetCollector(ctx context.Context, req *connect.Request[v1alpha1.GetCollectorRequest]) (*connect.Response[v1alpha1.GetCollectorResponse], error) {
	collectorID := req.Msg.GetCollectorId()

	domainCollector, err := a.mgr.Get(ctx, collectorID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("collector not found: %s", collectorID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector: %w", err))
	}

	desc, err := domainCollector.GetDescription(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector description: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetCollectorResponse{
		Collector: desc,
	}), nil
}

func (a *DeploymentServer) Status(ctx context.Context, req *connect.Request[v1alpha1.GetCollectorStatusRequest]) (*connect.Response[v1alpha1.GetCollectorStatusResponse], error) {
	collectorID := req.Msg.GetCollectorId()

	domainCollector, err := a.mgr.Get(ctx, collectorID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("collector not found: %s", collectorID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector: %w", err))
	}

	collectorStatus, err := domainCollector.Status(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector status: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetCollectorStatusResponse{
		Status: collectorStatus,
	}), nil
}

func (a *DeploymentServer) DeleteCollector(ctx context.Context, req *connect.Request[v1alpha1.DeleteCollectorRequest]) (*connect.Response[emptypb.Empty], error) {
	collectorID := req.Msg.GetCollectorId()
	if collectorID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("collector_id must not be empty"))
	}

	a.logger.With("collector_id", collectorID).Info("deleting collector")

	if err := a.mgr.Delete(ctx, collectorID); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("collector not found: %s", collectorID))
		}
		a.logger.With("collector_id", collectorID, "err", err).Error("failed to delete collector")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete collector: %w", err))
	}

	a.logger.With("collector_id", collectorID).Info("collector deleted successfully")
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (a *DeploymentServer) CollectorHistory(ctx context.Context, req *connect.Request[v1alpha1.GetCollectorHistoryRequest]) (*connect.Response[v1alpha1.GetCollectorHistoryResponse], error) {
	collectorID := req.Msg.GetCollectorId()
	offset, limit := req.Msg.GetOffset(), req.Msg.GetLimit()

	a.logger.With("collector_id", collectorID).Debug("requesting collector history")
	inst, err := a.mgr.Get(ctx, collectorID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("collector not found: %s", collectorID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector: %w", err))
	}

	configs, err := inst.History(ctx, offset, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get collector history: %w", err))
	}
	return connect.NewResponse(&v1alpha1.GetCollectorHistoryResponse{
		EffectiveConfig: configs,
	}), nil
}

func (a *DeploymentServer) ValidateRouter(ctx context.Context, req *connect.Request[v1alpha1.ValidateRouterRequest]) (*connect.Response[v1alpha1.ValidateRouterResponse], error) {
	_, err := router.NewMatcher(req.Msg.GetRouter())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid router : %w", err))
	}
	return connect.NewResponse(&v1alpha1.ValidateRouterResponse{}), nil
}

func (a *DeploymentServer) PreviewRouter(ctx context.Context, req *connect.Request[v1alpha1.PreviewRouterRequest]) (*connect.Response[v1alpha1.PreviewRouterResponse], error) {
	incoming, err := a.incomingAssignment(ctx, req.Msg.GetRouter())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1alpha1.PreviewRouterResponse{
		Old: &v1alpha1.RouterConfigAssignment{
			// TODO :
			CollectorsToConfigRef: nil,
		},
		New: &v1alpha1.RouterConfigAssignment{
			CollectorsToConfigRef: incoming,
		},
	}), nil
}

func (a *DeploymentServer) incomingAssignment(ctx context.Context, req *routev1alpha1.Router) (map[string]string, error) {
	matcher, err := router.NewMatcher(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("expected a valid router : %w", err))
	}
	insts, err := a.mgr.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ret := map[string]string{}

	for _, inst := range insts {
		desc, err := inst.GetDescription(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		l := util.CollectorDescriptionToLabels(desc)
		configRef := matcher.Match(ctx, l)
		ret[desc.GetId()] = configRef
	}
	return ret, nil
}
