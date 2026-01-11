package agent

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/protobufs"
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

	healthStore       storage.KeyValue[*protobufs.ComponentHealth]
	configStore       storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus]

	services.Service
}

var _ v1alpha1connect.AgentServiceHandler = (*AgentServer)(nil)

func NewAgentServer(
	logger *slog.Logger,
	agentStore storage.KeyValue[*v1alpha1.AgentDescription],
	agentTracker opamp.AgentTracker,
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	configStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
) *AgentServer {
	a := &AgentServer{
		logger:            logger,
		agentStore:        agentStore,
		agentTracker:      agentTracker,
		healthStore:       healthStore,
		configStore:       configStore,
		remoteStatusStore: remoteStatusStore,
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
			agentID := entry.Agent.GetId()
			st, ok := a.agentTracker.GetStatus(agentID)

			if ok {
				newEntry.Status = st
			} else {
				newEntry.Status = a.status(ctx, agentID)
			}
			descAndStatus[idx] = newEntry
		}
	}
	resp := &v1alpha1.ListAgentsResponse{
		Agents: descAndStatus,
	}
	return connect.NewResponse(resp), nil
}
func (a *AgentServer) GetAgent(ctx context.Context, req *connect.Request[v1alpha1.GetAgentRequest]) (*connect.Response[v1alpha1.GetAgentResponse], error) {
	agentID := req.Msg.GetAgentId()
	agent, err := a.agentStore.Get(ctx, agentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&v1alpha1.GetAgentResponse{Agent: agent}), nil
}

func (a *AgentServer) Status(ctx context.Context, req *connect.Request[v1alpha1.GetAgentStatusRequest]) (*connect.Response[v1alpha1.GetAgentStatusResponse], error) {
	agentID := req.Msg.GetAgentId()

	resp := a.status(ctx, agentID)

	return connect.NewResponse(&v1alpha1.GetAgentStatusResponse{
		Status: resp,
	}), nil
}

func (a *AgentServer) status(ctx context.Context, agentID string) *v1alpha1.AgentStatus {
	resp := &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AGENT_STATE_UNKNOWN,
	}

	if st, ok := a.agentTracker.GetStatus(agentID); ok {
		resp.State = st.State
	}

	if health, err := a.healthStore.Get(ctx, agentID); err == nil {
		resp.Health = convertHealth(health)
	}

	if config, err := a.configStore.Get(ctx, agentID); err == nil {
		resp.EffectiveConfig = convertEffectiveConfig(config)
	}

	if status, err := a.remoteStatusStore.Get(ctx, agentID); err == nil {
		resp.RemoteConfigStatus = convertRemoteConfigStatus(status)
	}
	return resp
}

func convertHealth(h *protobufs.ComponentHealth) *v1alpha1.ComponentHealth {
	if h == nil {
		return nil
	}
	result := &v1alpha1.ComponentHealth{
		Healthy:            h.Healthy,
		StartTimeUnixNano:  h.StartTimeUnixNano,
		LastError:          h.LastError,
		Status:             h.Status,
		StatusTimeUnixNano: h.StatusTimeUnixNano,
	}
	if len(h.ComponentHealthMap) > 0 {
		result.ComponentHealthMap = make(map[string]*v1alpha1.ComponentHealth, len(h.ComponentHealthMap))
		for k, v := range h.ComponentHealthMap {
			result.ComponentHealthMap[k] = convertHealth(v)
		}
	}
	return result
}

func convertEffectiveConfig(c *protobufs.EffectiveConfig) *v1alpha1.EffectiveConfig {
	if c == nil || c.ConfigMap == nil {
		return nil
	}
	result := &v1alpha1.EffectiveConfig{
		ConfigMap: &v1alpha1.AgentConfigMap{},
	}
	if len(c.ConfigMap.ConfigMap) > 0 {
		result.ConfigMap.ConfigMap = make(map[string]*v1alpha1.AgentConfigFile, len(c.ConfigMap.ConfigMap))
		for k, v := range c.ConfigMap.ConfigMap {
			result.ConfigMap.ConfigMap[k] = &v1alpha1.AgentConfigFile{
				Body:        v.Body,
				ContentType: v.ContentType,
			}
		}
	}
	return result
}

func convertRemoteConfigStatus(s *protobufs.RemoteConfigStatus) *v1alpha1.RemoteConfigStatus {
	if s == nil {
		return nil
	}
	return &v1alpha1.RemoteConfigStatus{
		LastRemoteConfigHash: s.LastRemoteConfigHash,
		Status:               v1alpha1.RemoteConfigStatuses(s.Status),
		ErrorMessage:         s.ErrorMessage,
	}
}
