package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1/v1alpha1connect"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
)

// AgentServer provides the agent management API.
// It uses the agent repository to access agent data from multiple stores.
type AgentServer struct {
	logger     *slog.Logger
	repository agentdomain.Repository

	services.Service
}

var _ v1alpha1connect.AgentServiceHandler = (*AgentServer)(nil)

// NewAgentServer creates a new AgentServer with the specified repository.
func NewAgentServer(
	logger *slog.Logger,
	repository agentdomain.Repository,
) *AgentServer {
	a := &AgentServer{
		logger:     logger,
		repository: repository,
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
	agents, err := a.repository.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list agents: %w", err))
	}

	a.logger.With("numAgents", len(agents)).Debug("found agents")

	// Convert domain agents to API response
	descAndStatus := make([]*v1alpha1.AgentDescriptionAndStatus, 0, len(agents))
	for _, domainAgent := range agents {
		if req.Msg.GetWithStatus() {
			// Full view with status
			descAndStatus = append(descAndStatus, &v1alpha1.AgentDescriptionAndStatus{
				Agent:  toAPIAgentDescription(domainAgent),
				Status: agentdomain.ToAPIStatus(domainAgent),
			})
		} else {
			// Basic view without status
			descAndStatus = append(descAndStatus, &v1alpha1.AgentDescriptionAndStatus{
				Agent: toAPIAgentDescription(domainAgent),
			})
		}
	}

	return connect.NewResponse(&v1alpha1.ListAgentsResponse{
		Agents: descAndStatus,
	}), nil
}

func (a *AgentServer) GetAgent(ctx context.Context, req *connect.Request[v1alpha1.GetAgentRequest]) (*connect.Response[v1alpha1.GetAgentResponse], error) {
	agentID := req.Msg.GetAgentId()

	domainAgent, err := a.repository.Get(ctx, agentID)
	if err != nil {
		if errors.Is(err, agentdomain.ErrAgentNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetAgentResponse{
		Agent: toAPIAgentDescription(domainAgent),
	}), nil
}

func (a *AgentServer) Status(ctx context.Context, req *connect.Request[v1alpha1.GetAgentStatusRequest]) (*connect.Response[v1alpha1.GetAgentStatusResponse], error) {
	agentID := req.Msg.GetAgentId()

	domainAgent, err := a.repository.Get(ctx, agentID)
	if err != nil {
		if errors.Is(err, agentdomain.ErrAgentNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get agent: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetAgentStatusResponse{
		Status: agentdomain.ToAPIStatus(domainAgent),
	}), nil
}

// toAPIAgentDescription converts a domain Agent to the v1alpha1.AgentDescription proto type.
// This maintains backward compatibility with the existing API.
func toAPIAgentDescription(agent *agentdomain.Agent) *v1alpha1.AgentDescription {
	reg := agentdomain.ToAPIAgentRegistration(agent)
	return &v1alpha1.AgentDescription{
		Id:                       reg.GetId(),
		FriendlyName:             reg.GetFriendlyName(),
		IdentifyingAttributes:    reg.GetIdentifyingAttributes(),
		NonIdentifyingAttributes: reg.GetNonIdentifyingAttributes(),
		Capabilities:             reg.GetCapabilities(),
	}
}
