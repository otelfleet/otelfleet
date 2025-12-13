package agent

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/samber/lo"
)

type AgentServer struct {
	logger       *slog.Logger
	agentStore   storage.KeyValue[*v1alpha1.AgentDescription]
	agentTracker opamp.AgentTracker

	services.Service
}

var _ v1alpha1connect.AgentServiceHandler = (*AgentServer)(nil)

func NewAgentServer(
	logger *slog.Logger,
	agentStore storage.KeyValue[*v1alpha1.AgentDescription],
	agentTracker opamp.AgentTracker,
) *AgentServer {
	a := &AgentServer{
		logger:       logger,
		agentStore:   agentStore,
		agentTracker: agentTracker,
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
) (*connect.Response[v1alpha1.ListAgentsReponse], error) {
	agents, err := a.agentStore.List(ctx)
	if err != nil {
		return nil, err
	}
	a.logger.With("numAgents", len(agents)).Debug("found agents")
	descAndStatus := lo.Map(agents, func(a *v1alpha1.AgentDescription, _ int) *v1alpha1.AgentDescriptionAndStatus {
		return &v1alpha1.AgentDescriptionAndStatus{
			Agent: a,
		}
	})
	if req.Msg.GetWithStatus() {
		for idx, entry := range descAndStatus {
			newEntry := &v1alpha1.AgentDescriptionAndStatus{
				Agent: entry.Agent,
			}
			agentId := entry.Agent.GetId()
			st, ok := a.agentTracker.Get(agentId)

			if ok {
				newEntry.Status = st
			} else {
				// a.logger.With("agentId", agentId).Error("no tracked state, setting default")
				newEntry.Status = &v1alpha1.AgentStatus{
					State: v1alpha1.AgentState_AgentStateUnknown,
				}
			}
			descAndStatus[idx] = newEntry
		}
	}
	resp := &v1alpha1.ListAgentsReponse{
		Agents: descAndStatus,
	}
	return connect.NewResponse(resp), nil
}
func (a *AgentServer) GetAgent(context.Context, *connect.Request[v1alpha1.GetAgentRequest]) (*connect.Response[v1alpha1.GetAgentResponse], error) {
	panic("implement me")
}
func (a *AgentServer) Status(context.Context, *connect.Request[v1alpha1.GetAgentStatusRequest]) (*connect.Response[v1alpha1.AgentStatus], error) {
	panic("implement me")
}
