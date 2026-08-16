package agent

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/deployment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AgentServer struct {
	logger *slog.Logger

	mgr deployment.Manager

	services.Service
}

var _ v1alpha1connect.AgentServiceHandler = (*AgentServer)(nil)

func NewAgentServer(
	logger *slog.Logger,
	mgr deployment.Manager,
) *AgentServer {
	a := &AgentServer{
		logger: logger,
		mgr:    mgr,
	}
	a.Service = services.NewBasicService(nil, a.running, nil)
	return a
}

func (a *AgentServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (a *AgentServer) ConfigureHTTP(mux *mux.Router) {
	a.logger.Info("configuring routes")
	v1alpha1connect.RegisterAgentServiceHandler(mux, a)
}

func (a *AgentServer) ListAgents(
	ctx context.Context, req *connect.Request[v1alpha1.ListAgentsRequest],
) (*connect.Response[v1alpha1.ListAgentsResponse], error) {
	agents, err := a.mgr.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list agents: %w", err))
	}

	a.logger.With("numAgents", len(agents)).Debug("found agents")

	// Convert domain agents to API response
	descAndStatus := make([]*v1alpha1.AgentDescriptionAndStatus, 0, len(agents))
	for _, domainAgent := range agents {
		desc, err := domainAgent.GetDescription(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent description: %w", err))
		}
		entry := &v1alpha1.AgentDescriptionAndStatus{Agent: desc}
		if req.Msg.GetWithStatus() {
			st, err := domainAgent.Status(ctx)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent status: %w", err))
			}
			entry.Status = st
		}
		descAndStatus = append(descAndStatus, entry)
	}

	return connect.NewResponse(&v1alpha1.ListAgentsResponse{
		Agents: descAndStatus,
	}), nil
}

func (a *AgentServer) GetAgent(ctx context.Context, req *connect.Request[v1alpha1.GetAgentRequest]) (*connect.Response[v1alpha1.GetAgentResponse], error) {
	agentID := req.Msg.GetAgentId()

	domainAgent, err := a.mgr.Get(ctx, agentID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent: %w", err))
	}

	desc, err := domainAgent.GetDescription(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent description: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetAgentResponse{
		Agent: desc,
	}), nil
}

func (a *AgentServer) Status(ctx context.Context, req *connect.Request[v1alpha1.GetAgentStatusRequest]) (*connect.Response[v1alpha1.GetAgentStatusResponse], error) {
	agentID := req.Msg.GetAgentId()

	domainAgent, err := a.mgr.Get(ctx, agentID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent: %w", err))
	}

	agentStatus, err := domainAgent.Status(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent status: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetAgentStatusResponse{
		Status: agentStatus,
	}), nil
}

func (a *AgentServer) DeleteAgent(ctx context.Context, req *connect.Request[v1alpha1.DeleteAgentRequest]) (*connect.Response[emptypb.Empty], error) {
	agentID := req.Msg.GetAgentId()
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent_id must not be empty"))
	}

	a.logger.With("agent_id", agentID).Info("deleting agent")

	if err := a.mgr.Delete(ctx, agentID); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		a.logger.With("agent_id", agentID, "err", err).Error("failed to delete agent")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete agent: %w", err))
	}

	a.logger.With("agent_id", agentID).Info("agent deleted successfully")
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (a *AgentServer) AgentHistory(ctx context.Context, req *connect.Request[v1alpha1.GetAgentHistoryRequest]) (*connect.Response[v1alpha1.GetAgentHistoryResponse], error) {
	agentID := req.Msg.GetAgentId()
	offset, limit := req.Msg.GetOffset(), req.Msg.GetLimit()

	a.logger.With("agent_id", agentID).Debug("requesting agent history")
	inst, err := a.mgr.Get(ctx, agentID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent: %w", err))
	}

	configs, err := inst.History(ctx, offset, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent history: %w", err))
	}
	return connect.NewResponse(&v1alpha1.GetAgentHistoryResponse{
		EffectiveConfig: configs,
	}), nil
}
