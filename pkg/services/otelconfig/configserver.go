package otelconfig

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/protobufs"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1/v1alpha1connect"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/configsync"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/samber/lo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ConfigChangeNotifier is an interface for notifying when a config changes for an agent.
// This is implemented by the OpAMP server to push configs to connected agents.
type ConfigChangeNotifier interface {
	NotifyConfigChange(agentID string)
}

// DeploymentController handles rolling deployments
type DeploymentController interface {
	StartDeployment(ctx context.Context, req *v1alpha1.RollingDeploymentRequest) (string, error)
	GetStatus(ctx context.Context, deploymentID string) (*v1alpha1.DeploymentStatus, error)
	PauseDeployment(ctx context.Context, deploymentID string) error
	ResumeDeployment(ctx context.Context, deploymentID string) error
	CancelDeployment(ctx context.Context, deploymentID string) error
	ListDeployments(ctx context.Context, stateFilter *v1alpha1.DeploymentState) ([]*v1alpha1.DeploymentStatus, error)
}

type ConfigServer struct {
	configStore           storage.KeyValue[*v1alpha1.Config]
	defaultConfigStore    storage.KeyValue[*v1alpha1.Config]
	assignedConfigStore   storage.KeyValue[*v1alpha1.Config]
	configAssignmentStore storage.KeyValue[*v1alpha1.ConfigAssignment]
	agentRepo             agentdomain.Repository
	effectiveConfigStore  storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore     storage.KeyValue[*protobufs.RemoteConfigStatus]
	logger                *slog.Logger

	notifier             ConfigChangeNotifier
	deploymentController DeploymentController

	services.Service
}

var _ v1alpha1connect.ConfigServiceHandler = (*ConfigServer)(nil)

func NewConfigServer(
	logger *slog.Logger,
	configStore storage.KeyValue[*v1alpha1.Config],
	defaultConfigStore storage.KeyValue[*v1alpha1.Config],
	assignedConfigStore storage.KeyValue[*v1alpha1.Config],
	configAssignmentStore storage.KeyValue[*v1alpha1.ConfigAssignment],
	agentRepo agentdomain.Repository,
	effectiveConfigStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
) *ConfigServer {
	cs := &ConfigServer{
		logger:                logger,
		configStore:           configStore,
		defaultConfigStore:    defaultConfigStore,
		assignedConfigStore:   assignedConfigStore,
		configAssignmentStore: configAssignmentStore,
		agentRepo:             agentRepo,
		effectiveConfigStore:  effectiveConfigStore,
		remoteStatusStore:     remoteStatusStore,
	}
	cs.Service = services.NewBasicService(nil, cs.running, nil)
	return cs
}

// SetNotifier sets the config change notifier (typically the OpAMP server)
func (c *ConfigServer) SetNotifier(notifier ConfigChangeNotifier) {
	c.notifier = notifier
}

// SetDeploymentController sets the deployment controller
func (c *ConfigServer) SetDeploymentController(controller DeploymentController) {
	c.deploymentController = controller
}

// notifyConfigChange notifies the OpAMP server that a config has changed for an agent
func (c *ConfigServer) notifyConfigChange(agentID string) {
	if c.notifier != nil {
		c.notifier.NotifyConfigChange(agentID)
	}
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
func (c *ConfigServer) PutConfig(ctx context.Context, connectReq *connect.Request[v1alpha1.PutConfigRequest]) (*connect.Response[emptypb.Empty], error) {
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

	configs, err := c.configStore.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	resp.Configs = lo.Map(configs, func(key string, _ int) *v1alpha1.ConfigReference {
		c.logger.With("key", key).Info("got config key")
		return &v1alpha1.ConfigReference{
			Id: key,
		}
	})
	return connect.NewResponse(resp), nil
}

const globalDefaultKey = "global"

func (c *ConfigServer) GetDefaultConfig(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1.Config], error) {
	val, err := c.defaultConfigStore.Get(ctx, globalDefaultKey)
	if err == nil {
		return connect.NewResponse(val), nil
	}
	st, ok := status.FromError(err)
	if ok && st.Code() == codes.NotFound {
		return connect.NewResponse(&v1alpha1.Config{
			Config: []byte(DefaultOtelConfig),
		}), nil
	}
	return nil, status.Error(codes.Internal, err.Error())
}

func (c *ConfigServer) SetDefaultConfig(context.Context, *connect.Request[v1alpha1.PutConfigRequest]) (*connect.Response[emptypb.Empty], error) {
	panic("implement me")
}

// ============================================================================
// Phase 1: Manual Config Assignment
// ============================================================================

// AssignConfig assigns a config to a single agent
func (c *ConfigServer) AssignConfig(ctx context.Context, req *connect.Request[v1alpha1.AssignConfigRequest]) (*connect.Response[v1alpha1.AssignConfigResponse], error) {
	agentID := req.Msg.GetAgentId()
	configID := req.Msg.GetConfigId()

	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent_id must be non-empty"))
	}
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("config_id must be non-empty"))
	}

	// Validate config exists
	config, err := c.configStore.Get(ctx, configID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("config not found: %s", configID))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Validate agent exists
	exists, err := c.agentRepo.Exists(ctx, agentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent not found: %s", agentID))
	}

	// Store the config in assignedConfigStore (keyed by agentID)
	if err := c.assignedConfigStore.Put(ctx, agentID, config); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Store assignment metadata
	assignment := &v1alpha1.ConfigAssignment{
		AgentId:    agentID,
		ConfigId:   configID,
		Source:     v1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL,
		AssignedAt: timestamppb.Now(),
		ConfigHash: util.HashAgentConfigMap(util.ProtoConfigToAgentConfigMap(config)),
	}
	if err := c.configAssignmentStore.Put(ctx, agentID, assignment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Notify OpAMP server to push config
	c.notifyConfigChange(agentID)

	c.logger.With("agent_id", agentID, "config_id", configID).Info("config assigned to agent")

	return connect.NewResponse(&v1alpha1.AssignConfigResponse{
		Success: true,
		Message: "Config assigned successfully",
	}), nil
}

// GetAgentConfig returns the config assignment for a specific agent
func (c *ConfigServer) GetAgentConfig(ctx context.Context, req *connect.Request[v1alpha1.GetAgentConfigRequest]) (*connect.Response[v1alpha1.GetAgentConfigResponse], error) {
	agentID := req.Msg.GetAgentId()
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent_id must be non-empty"))
	}

	assignment, err := c.configAssignmentStore.Get(ctx, agentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no config assigned to agent: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.GetAgentConfigResponse{
		ConfigId:   assignment.GetConfigId(),
		Source:     assignment.GetSource(),
		AssignedAt: assignment.GetAssignedAt(),
	}), nil
}

// UnassignConfig removes the config assignment from an agent
func (c *ConfigServer) UnassignConfig(ctx context.Context, req *connect.Request[v1alpha1.UnassignConfigRequest]) (*connect.Response[v1alpha1.UnassignConfigResponse], error) {
	agentID := req.Msg.GetAgentId()
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent_id must be non-empty"))
	}

	// Delete from assignedConfigStore
	if err := c.assignedConfigStore.Delete(ctx, agentID); err != nil {
		if !grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Delete from configAssignmentStore
	if err := c.configAssignmentStore.Delete(ctx, agentID); err != nil {
		if !grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Notify OpAMP server - agent will get default config
	c.notifyConfigChange(agentID)

	c.logger.With("agent_id", agentID).Info("config unassigned from agent")

	return connect.NewResponse(&v1alpha1.UnassignConfigResponse{
		Success: true,
	}), nil
}

// ============================================================================
// Phase 2: Config Assignment Queries and Status
// ============================================================================

// ListConfigAssignments lists all config assignments, optionally filtered by config ID
func (c *ConfigServer) ListConfigAssignments(ctx context.Context, req *connect.Request[v1alpha1.ListConfigAssignmentsRequest]) (*connect.Response[v1alpha1.ListConfigAssignmentsResponse], error) {
	assignments, err := c.configAssignmentStore.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var result []*v1alpha1.ConfigAssignmentInfo
	for _, assignment := range assignments {
		// Filter by configId if specified
		if req.Msg.ConfigId != nil && assignment.GetConfigId() != *req.Msg.ConfigId {
			continue
		}

		// Enrich with status from remoteStatusStore
		appStatus, errorMsg, err := c.getRemoteConfigStatus(ctx, assignment.GetAgentId(), assignment.GetConfigHash())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get config status for agent %s: %w", assignment.GetAgentId(), err))
		}

		result = append(result, &v1alpha1.ConfigAssignmentInfo{
			AgentId:      assignment.GetAgentId(),
			ConfigId:     assignment.GetConfigId(),
			Source:       assignment.GetSource(),
			AssignedAt:   assignment.GetAssignedAt(),
			Status:       appStatus,
			ErrorMessage: errorMsg,
		})
	}

	return connect.NewResponse(&v1alpha1.ListConfigAssignmentsResponse{
		Assignments: result,
	}), nil
}

// getRemoteConfigStatus returns the application status for an agent's config.
// Uses the shared configsync helper for consistent status computation.
func (c *ConfigServer) getRemoteConfigStatus(ctx context.Context, agentID string, assignedHash []byte) (v1alpha1.ConfigApplicationStatus, string, error) {
	syncStatus, reason, err := configsync.ComputeConfigSyncStatus(ctx, agentID, assignedHash, c.remoteStatusStore)
	if err != nil {
		return v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_UNSPECIFIED, reason, err
	}

	// Map ConfigSyncStatus to ConfigApplicationStatus for backward compatibility
	switch syncStatus {
	case agentsv1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_IN_SYNC:
		return v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_APPLIED, "", nil
	case agentsv1alpha1.ConfigSyncStatus_CONFIG_SYNC_STATUS_ERROR:
		return v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_FAILED, reason, nil
	default:
		return v1alpha1.ConfigApplicationStatus_CONFIG_APPLICATION_STATUS_PENDING, reason, nil
	}
}

// GetConfigStatus returns detailed status for a specific agent's config
func (c *ConfigServer) GetConfigStatus(ctx context.Context, req *connect.Request[v1alpha1.GetConfigStatusRequest]) (*connect.Response[v1alpha1.GetConfigStatusResponse], error) {
	agentID := req.Msg.GetAgentId()
	if agentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent_id must be non-empty"))
	}

	assignment, err := c.configAssignmentStore.Get(ctx, agentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no config assigned to agent: %s", agentID))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get effective config from agent (what it reports)
	// Use the same hash function as when storing the assignment for consistent comparison
	var effectiveHash []byte
	effectiveConfig, err := c.effectiveConfigStore.Get(ctx, agentID)
	if err != nil && !grpcutil.IsErrorNotFound(err) {
		// Storage error (not just "no config reported yet")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get effective config: %w", err))
	}
	if err == nil && effectiveConfig.GetConfigMap() != nil {
		effectiveHash = util.HashAgentConfigMap(effectiveConfig.GetConfigMap())
	}

	inSync := bytes.Equal(assignment.GetConfigHash(), effectiveHash)

	appStatus, errorMsg, err := c.getRemoteConfigStatus(ctx, agentID, assignment.GetConfigHash())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get config status: %w", err))
	}

	return connect.NewResponse(&v1alpha1.GetConfigStatusResponse{
		Assignment: &v1alpha1.ConfigAssignmentInfo{
			AgentId:      assignment.GetAgentId(),
			ConfigId:     assignment.GetConfigId(),
			Source:       assignment.GetSource(),
			AssignedAt:   assignment.GetAssignedAt(),
			Status:       appStatus,
			ErrorMessage: errorMsg,
		},
		EffectiveConfigHash: effectiveHash,
		AssignedConfigHash:  assignment.GetConfigHash(),
		InSync:              inSync,
	}), nil
}

// ============================================================================
// Phase 3: Batch Assignment
// ============================================================================

// assignConfigToAgent is a helper that assigns a config to an agent (used by batch operations)
func (c *ConfigServer) assignConfigToAgent(ctx context.Context, agentID, configID string, config *v1alpha1.Config) error {
	// Validate agent exists
	exists, err := c.agentRepo.Exists(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to check agent existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	// Store the config in assignedConfigStore
	if err := c.assignedConfigStore.Put(ctx, agentID, config); err != nil {
		return err
	}

	// Store assignment metadata
	assignment := &v1alpha1.ConfigAssignment{
		AgentId:    agentID,
		ConfigId:   configID,
		Source:     v1alpha1.ConfigSource_CONFIG_SOURCE_MANUAL,
		AssignedAt: timestamppb.Now(),
		ConfigHash: util.HashAgentConfigMap(util.ProtoConfigToAgentConfigMap(config)),
	}
	return c.configAssignmentStore.Put(ctx, agentID, assignment)
}

// AssignConfigToAgent assigns a config to an agent by config ID (used by deployment controller)
// This implements the deployment.ConfigAssigner interface
func (c *ConfigServer) AssignConfigToAgent(ctx context.Context, agentID, configID string) error {
	// Get the config
	config, err := c.configStore.Get(ctx, configID)
	if err != nil {
		return fmt.Errorf("config not found: %s", configID)
	}

	// Assign the config
	if err := c.assignConfigToAgent(ctx, agentID, configID, config); err != nil {
		return err
	}

	// Notify OpAMP server to push config
	c.notifyConfigChange(agentID)

	return nil
}

// BatchAssignConfig assigns a config to multiple agents
func (c *ConfigServer) BatchAssignConfig(ctx context.Context, req *connect.Request[v1alpha1.BatchAssignConfigRequest]) (*connect.Response[v1alpha1.BatchAssignConfigResponse], error) {
	configID := req.Msg.GetConfigId()
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("config_id must be non-empty"))
	}

	// Validate config exists first
	config, err := c.configStore.Get(ctx, configID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("config not found: %s", configID))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var successful, failed int32
	var failedAgentIDs, errorMessages []string

	for _, agentID := range req.Msg.GetAgentIds() {
		err := c.assignConfigToAgent(ctx, agentID, configID, config)
		if err != nil {
			failed++
			failedAgentIDs = append(failedAgentIDs, agentID)
			errorMessages = append(errorMessages, err.Error())
		} else {
			successful++
			c.notifyConfigChange(agentID)
		}
	}

	c.logger.With("config_id", configID, "successful", successful, "failed", failed).Info("batch config assignment completed")

	return connect.NewResponse(&v1alpha1.BatchAssignConfigResponse{
		Successful:     successful,
		Failed:         failed,
		FailedAgentIds: failedAgentIDs,
		ErrorMessages:  errorMessages,
	}), nil
}


// AssignConfigByLabels assigns a config to agents matching the specified labels
func (c *ConfigServer) AssignConfigByLabels(ctx context.Context, req *connect.Request[v1alpha1.AssignConfigByLabelsRequest]) (*connect.Response[v1alpha1.AssignConfigByLabelsResponse], error) {
	configID := req.Msg.GetConfigId()
	labels := req.Msg.GetLabels()

	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("config_id must be non-empty"))
	}
	if len(labels) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("labels must be non-empty"))
	}

	// Find agents matching labels using repository
	agents, err := c.agentRepo.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var matchedAgentIDs []string
	for _, agent := range agents {
		if agent.MatchesLabels(labels) {
			matchedAgentIDs = append(matchedAgentIDs, agent.ID)
		}
	}

	if len(matchedAgentIDs) == 0 {
		return connect.NewResponse(&v1alpha1.AssignConfigByLabelsResponse{
			MatchedAgentIds: []string{},
			Successful:      0,
			Failed:          0,
		}), nil
	}

	// Delegate to batch assign
	batchReq := connect.NewRequest(&v1alpha1.BatchAssignConfigRequest{
		AgentIds: matchedAgentIDs,
		ConfigId: configID,
	})

	batchResp, err := c.BatchAssignConfig(ctx, batchReq)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&v1alpha1.AssignConfigByLabelsResponse{
		MatchedAgentIds: matchedAgentIDs,
		Successful:      batchResp.Msg.GetSuccessful(),
		Failed:          batchResp.Msg.GetFailed(),
	}), nil
}

// ============================================================================
// Phase 4: Rolling Deployment
// ============================================================================

// StartRollingDeployment starts a new rolling deployment
func (c *ConfigServer) StartRollingDeployment(ctx context.Context, req *connect.Request[v1alpha1.RollingDeploymentRequest]) (*connect.Response[v1alpha1.RollingDeploymentResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	deploymentID, err := c.deploymentController.StartDeployment(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.RollingDeploymentResponse{
		DeploymentId: deploymentID,
	}), nil
}

// GetDeploymentStatus returns the status of a deployment
func (c *ConfigServer) GetDeploymentStatus(ctx context.Context, req *connect.Request[v1alpha1.GetDeploymentStatusRequest]) (*connect.Response[v1alpha1.GetDeploymentStatusResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	status, err := c.deploymentController.GetStatus(ctx, req.Msg.GetDeploymentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.GetDeploymentStatusResponse{
		Status: status,
	}), nil
}

// PauseDeployment pauses a running deployment
func (c *ConfigServer) PauseDeployment(ctx context.Context, req *connect.Request[v1alpha1.PauseDeploymentRequest]) (*connect.Response[v1alpha1.DeploymentActionResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	if err := c.deploymentController.PauseDeployment(ctx, req.Msg.GetDeploymentId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.DeploymentActionResponse{
		Success: true,
		Message: "Deployment paused",
	}), nil
}

// ResumeDeployment resumes a paused deployment
func (c *ConfigServer) ResumeDeployment(ctx context.Context, req *connect.Request[v1alpha1.ResumeDeploymentRequest]) (*connect.Response[v1alpha1.DeploymentActionResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	if err := c.deploymentController.ResumeDeployment(ctx, req.Msg.GetDeploymentId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.DeploymentActionResponse{
		Success: true,
		Message: "Deployment resumed",
	}), nil
}

// CancelDeployment cancels a deployment
func (c *ConfigServer) CancelDeployment(ctx context.Context, req *connect.Request[v1alpha1.CancelDeploymentRequest]) (*connect.Response[v1alpha1.DeploymentActionResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	if err := c.deploymentController.CancelDeployment(ctx, req.Msg.GetDeploymentId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.DeploymentActionResponse{
		Success: true,
		Message: "Deployment cancelled",
	}), nil
}

// ListDeployments lists all deployments, optionally filtered by state
func (c *ConfigServer) ListDeployments(ctx context.Context, req *connect.Request[v1alpha1.ListDeploymentsRequest]) (*connect.Response[v1alpha1.ListDeploymentsResponse], error) {
	if c.deploymentController == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("deployment controller not configured"))
	}

	var stateFilter *v1alpha1.DeploymentState
	if req.Msg.StateFilter != nil {
		stateFilter = req.Msg.StateFilter
	}

	deployments, err := c.deploymentController.ListDeployments(ctx, stateFilter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.ListDeploymentsResponse{
		Deployments: deployments,
	}), nil
}
