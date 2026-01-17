package deployment

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/dskit/services"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxRetries     = 3
	retryBaseDelay = 100 * time.Millisecond
)

// retryWithBackoff retries the given function with exponential backoff.
// Returns the result and error from the last attempt.
func retryWithBackoff[T any](ctx context.Context, logger *slog.Logger, operation string, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Don't retry on NotFound - that's a legitimate "no data" response
		if grpcutil.IsErrorNotFound(lastErr) {
			return result, lastErr
		}

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Calculate backoff with exponential increase
		delay := retryBaseDelay * time.Duration(1<<attempt)

		logger.With("operation", operation, "attempt", attempt+1, "err", lastErr).
			Warn("storage operation failed, retrying")

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	return result, fmt.Errorf("%s failed after %d retries: %w", operation, maxRetries, lastErr)
}

// ConfigAssigner is an interface for assigning configs to agents
type ConfigAssigner interface {
	AssignConfigToAgent(ctx context.Context, agentID, configID string) error
}

// Controller manages rolling deployments of configs to agents
type Controller struct {
	logger *slog.Logger

	deploymentStore      storage.KeyValue[*configv1alpha1.DeploymentStatus]
	agentDeploymentStore storage.KeyValue[*configv1alpha1.AgentDeploymentStatus]
	configStore          storage.KeyValue[*configv1alpha1.Config]
	agentRepo            agentdomain.Repository

	configAssigner ConfigAssigner

	mu                sync.RWMutex
	activeDeployments map[string]context.CancelFunc

	services.Service
}

// Ensure Controller implements the DeploymentController interface
var _ otelconfig.DeploymentController = (*Controller)(nil)

// NewController creates a new deployment controller
func NewController(
	logger *slog.Logger,
	deploymentStore storage.KeyValue[*configv1alpha1.DeploymentStatus],
	agentDeploymentStore storage.KeyValue[*configv1alpha1.AgentDeploymentStatus],
	configStore storage.KeyValue[*configv1alpha1.Config],
	agentRepo agentdomain.Repository,
) *Controller {
	c := &Controller{
		logger:               logger,
		deploymentStore:      deploymentStore,
		agentDeploymentStore: agentDeploymentStore,
		configStore:          configStore,
		agentRepo:            agentRepo,
		activeDeployments:    make(map[string]context.CancelFunc),
	}
	c.Service = services.NewBasicService(nil, c.running, c.stopping)
	return c
}

// SetConfigAssigner sets the config assigner (typically the ConfigServer)
func (c *Controller) SetConfigAssigner(assigner ConfigAssigner) {
	c.configAssigner = assigner
}

func (c *Controller) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *Controller) stopping(_ error) error {
	// Cancel all active deployments
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cancel := range c.activeDeployments {
		cancel()
	}
	return nil
}

// StartDeployment starts a new rolling deployment
func (c *Controller) StartDeployment(ctx context.Context, req *configv1alpha1.RollingDeploymentRequest) (string, error) {
	if c.configAssigner == nil {
		return "", fmt.Errorf("config assigner not set")
	}

	// Validate config exists
	_, err := c.configStore.Get(ctx, req.GetConfigId())
	if err != nil {
		return "", fmt.Errorf("config not found: %s", req.GetConfigId())
	}

	// Resolve agent IDs (from list or labels)
	agentIDs := req.GetAgentIds()
	if len(agentIDs) == 0 && len(req.GetAgentLabels()) > 0 {
		agentIDs, err = c.resolveAgentsByLabels(ctx, req.GetAgentLabels())
		if err != nil {
			return "", err
		}
	}

	if len(agentIDs) == 0 {
		return "", fmt.Errorf("no agents to deploy to")
	}

	deploymentID := uuid.New().String()

	// Create deployment status
	status := &configv1alpha1.DeploymentStatus{
		DeploymentId:  deploymentID,
		ConfigId:      req.GetConfigId(),
		State:         configv1alpha1.DeploymentState_DEPLOYMENT_STATE_PENDING,
		TotalAgents:   int32(len(agentIDs)),
		PendingAgents: int32(len(agentIDs)),
		CurrentBatch:  0,
		StartedAt:     timestamppb.Now(),
	}

	// Store initial status
	if err := c.deploymentStore.Put(ctx, deploymentID, status); err != nil {
		return "", err
	}

	// Initialize per-agent status
	for _, agentID := range agentIDs {
		agentStatus := &configv1alpha1.AgentDeploymentStatus{
			AgentId: agentID,
			State:   configv1alpha1.AgentDeploymentState_AGENT_DEPLOYMENT_STATE_PENDING,
		}
		key := fmt.Sprintf("%s/%s", deploymentID, agentID)
		if err := c.agentDeploymentStore.Put(ctx, key, agentStatus); err != nil {
			c.logger.With("err", err, "agent_id", agentID).Error("failed to store agent deployment status")
		}
	}

	// Start deployment goroutine
	deployCtx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.activeDeployments[deploymentID] = cancel
	c.mu.Unlock()

	go c.runDeployment(deployCtx, deploymentID, agentIDs, req)

	c.logger.With("deployment_id", deploymentID, "config_id", req.GetConfigId(), "agent_count", len(agentIDs)).Info("started rolling deployment")

	return deploymentID, nil
}

func (c *Controller) resolveAgentsByLabels(ctx context.Context, labels map[string]string) ([]string, error) {
	agents, err := c.agentRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	var matchedAgentIDs []string
	for _, agent := range agents {
		if agent.MatchesLabels(labels) {
			matchedAgentIDs = append(matchedAgentIDs, agent.ID)
		}
	}
	return matchedAgentIDs, nil
}

func (c *Controller) runDeployment(ctx context.Context, deploymentID string, agentIDs []string, req *configv1alpha1.RollingDeploymentRequest) {
	defer func() {
		c.mu.Lock()
		delete(c.activeDeployments, deploymentID)
		c.mu.Unlock()
	}()

	batchSize := int(req.GetBatchSize())
	if batchSize <= 0 {
		batchSize = 1
	}

	batchDelay := time.Duration(req.GetBatchDelaySeconds()) * time.Second
	failureCount := 0
	maxFailures := int(req.GetMaxFailures())

	// Update status to in_progress
	c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_IN_PROGRESS)

	// Process in batches
	for i := 0; i < len(agentIDs); i += batchSize {
		select {
		case <-ctx.Done():
			c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_CANCELLED)
			return
		default:
		}

		// Check if paused (with retry for transient storage errors)
		status, err := retryWithBackoff(ctx, c.logger, "check deployment paused state", func() (*configv1alpha1.DeploymentStatus, error) {
			return c.deploymentStore.Get(ctx, deploymentID)
		})
		if err != nil {
			c.logger.With("err", err, "deployment_id", deploymentID).Error("failed to check deployment state, failing deployment")
			c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_FAILED)
			return
		}
		if status.GetState() == configv1alpha1.DeploymentState_DEPLOYMENT_STATE_PAUSED {
			// Wait for resume or cancel
			pauseCheckFailures := 0
			const maxPauseCheckFailures = 5
			for status.GetState() == configv1alpha1.DeploymentState_DEPLOYMENT_STATE_PAUSED {
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
					status, err = retryWithBackoff(ctx, c.logger, "check deployment paused state", func() (*configv1alpha1.DeploymentStatus, error) {
						return c.deploymentStore.Get(ctx, deploymentID)
					})
					if err != nil {
						pauseCheckFailures++
						c.logger.With("err", err, "deployment_id", deploymentID, "failures", pauseCheckFailures).Error("failed to check deployment state while paused")
						if pauseCheckFailures >= maxPauseCheckFailures {
							c.logger.With("deployment_id", deploymentID).Error("too many storage failures while paused, failing deployment")
							c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_FAILED)
							return
						}
					} else {
						pauseCheckFailures = 0 // Reset on success
					}
				}
			}
		}

		// Get batch
		end := i + batchSize
		if end > len(agentIDs) {
			end = len(agentIDs)
		}
		batch := agentIDs[i:end]

		// Update current batch
		c.updateCurrentBatch(ctx, deploymentID, int32(i/batchSize+1))

		// Apply config to batch
		for _, agentID := range batch {
			c.updateAgentState(ctx, deploymentID, agentID, configv1alpha1.AgentDeploymentState_AGENT_DEPLOYMENT_STATE_APPLYING, "")

			err := c.configAssigner.AssignConfigToAgent(ctx, agentID, req.GetConfigId())
			if err != nil {
				c.updateAgentState(ctx, deploymentID, agentID, configv1alpha1.AgentDeploymentState_AGENT_DEPLOYMENT_STATE_FAILED, err.Error())
				failureCount++
				c.incrementFailureCount(ctx, deploymentID)

				if maxFailures > 0 && failureCount >= maxFailures {
					c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_FAILED)
					return
				}
			} else {
				c.updateAgentState(ctx, deploymentID, agentID, configv1alpha1.AgentDeploymentState_AGENT_DEPLOYMENT_STATE_APPLIED, "")
				c.incrementCompletedCount(ctx, deploymentID)
			}
		}

		// Batch delay
		if batchDelay > 0 && i+batchSize < len(agentIDs) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(batchDelay):
			}
		}
	}

	// Mark as completed
	c.updateDeploymentState(ctx, deploymentID, configv1alpha1.DeploymentState_DEPLOYMENT_STATE_COMPLETED)
	c.logger.With("deployment_id", deploymentID).Info("rolling deployment completed")
}

func (c *Controller) updateDeploymentState(ctx context.Context, deploymentID string, state configv1alpha1.DeploymentState) {
	status, err := retryWithBackoff(ctx, c.logger, "get deployment status", func() (*configv1alpha1.DeploymentStatus, error) {
		return c.deploymentStore.Get(ctx, deploymentID)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Error("failed to get deployment status after retries")
		return
	}
	status.State = state
	if state == configv1alpha1.DeploymentState_DEPLOYMENT_STATE_COMPLETED ||
		state == configv1alpha1.DeploymentState_DEPLOYMENT_STATE_FAILED ||
		state == configv1alpha1.DeploymentState_DEPLOYMENT_STATE_CANCELLED {
		status.CompletedAt = timestamppb.Now()
	}
	_, err = retryWithBackoff(ctx, c.logger, "update deployment state", func() (struct{}, error) {
		return struct{}{}, c.deploymentStore.Put(ctx, deploymentID, status)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Error("failed to update deployment state after retries")
	}
}

func (c *Controller) updateCurrentBatch(ctx context.Context, deploymentID string, batch int32) {
	status, err := retryWithBackoff(ctx, c.logger, "get deployment for batch update", func() (*configv1alpha1.DeploymentStatus, error) {
		return c.deploymentStore.Get(ctx, deploymentID)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to get deployment for batch update")
		return
	}
	status.CurrentBatch = batch
	_, err = retryWithBackoff(ctx, c.logger, "update current batch", func() (struct{}, error) {
		return struct{}{}, c.deploymentStore.Put(ctx, deploymentID, status)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to update current batch")
	}
}

func (c *Controller) updateAgentState(ctx context.Context, deploymentID, agentID string, state configv1alpha1.AgentDeploymentState, errorMsg string) {
	key := fmt.Sprintf("%s/%s", deploymentID, agentID)
	agentStatus, err := retryWithBackoff(ctx, c.logger, "get agent deployment status", func() (*configv1alpha1.AgentDeploymentStatus, error) {
		return c.agentDeploymentStore.Get(ctx, key)
	})
	if err != nil {
		// If we can't get existing status (including NotFound), create a new one
		agentStatus = &configv1alpha1.AgentDeploymentStatus{
			AgentId: agentID,
		}
	}
	agentStatus.State = state
	agentStatus.ErrorMessage = errorMsg
	if state == configv1alpha1.AgentDeploymentState_AGENT_DEPLOYMENT_STATE_APPLIED {
		agentStatus.AppliedAt = timestamppb.Now()
	}
	_, err = retryWithBackoff(ctx, c.logger, "update agent deployment status", func() (struct{}, error) {
		return struct{}{}, c.agentDeploymentStore.Put(ctx, key, agentStatus)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID, "agent_id", agentID).Warn("failed to update agent state")
	}
}

func (c *Controller) incrementCompletedCount(ctx context.Context, deploymentID string) {
	status, err := retryWithBackoff(ctx, c.logger, "get deployment for completed count", func() (*configv1alpha1.DeploymentStatus, error) {
		return c.deploymentStore.Get(ctx, deploymentID)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to get deployment for completed count")
		return
	}
	status.CompletedAgents++
	status.PendingAgents--
	_, err = retryWithBackoff(ctx, c.logger, "increment completed count", func() (struct{}, error) {
		return struct{}{}, c.deploymentStore.Put(ctx, deploymentID, status)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to increment completed count")
	}
}

func (c *Controller) incrementFailureCount(ctx context.Context, deploymentID string) {
	status, err := retryWithBackoff(ctx, c.logger, "get deployment for failure count", func() (*configv1alpha1.DeploymentStatus, error) {
		return c.deploymentStore.Get(ctx, deploymentID)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to get deployment for failure count")
		return
	}
	status.FailedAgents++
	status.PendingAgents--
	_, err = retryWithBackoff(ctx, c.logger, "increment failure count", func() (struct{}, error) {
		return struct{}{}, c.deploymentStore.Put(ctx, deploymentID, status)
	})
	if err != nil {
		c.logger.With("err", err, "deployment_id", deploymentID).Warn("failed to increment failure count")
	}
}

// GetStatus returns the status of a deployment
func (c *Controller) GetStatus(ctx context.Context, deploymentID string) (*configv1alpha1.DeploymentStatus, error) {
	status, err := c.deploymentStore.Get(ctx, deploymentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, fmt.Errorf("deployment not found: %s", deploymentID)
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Fetch agent statuses
	keys, err := c.agentDeploymentStore.ListKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent deployment keys: %w", err)
	}
	var agentStatuses []*configv1alpha1.AgentDeploymentStatus
	prefix := deploymentID + "/"
	for _, key := range keys {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			agentStatus, err := c.agentDeploymentStore.Get(ctx, key)
			if err != nil && !grpcutil.IsErrorNotFound(err) {
				return nil, fmt.Errorf("failed to get agent deployment status for %s: %w", key, err)
			}
			if err == nil {
				agentStatuses = append(agentStatuses, agentStatus)
			}
		}
	}
	status.AgentStatuses = agentStatuses

	return status, nil
}

// PauseDeployment pauses a running deployment
func (c *Controller) PauseDeployment(ctx context.Context, deploymentID string) error {
	status, err := c.deploymentStore.Get(ctx, deploymentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return fmt.Errorf("deployment not found: %s", deploymentID)
		}
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if status.GetState() != configv1alpha1.DeploymentState_DEPLOYMENT_STATE_IN_PROGRESS {
		return fmt.Errorf("deployment is not in progress")
	}

	status.State = configv1alpha1.DeploymentState_DEPLOYMENT_STATE_PAUSED
	return c.deploymentStore.Put(ctx, deploymentID, status)
}

// ResumeDeployment resumes a paused deployment
func (c *Controller) ResumeDeployment(ctx context.Context, deploymentID string) error {
	status, err := c.deploymentStore.Get(ctx, deploymentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return fmt.Errorf("deployment not found: %s", deploymentID)
		}
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if status.GetState() != configv1alpha1.DeploymentState_DEPLOYMENT_STATE_PAUSED {
		return fmt.Errorf("deployment is not paused")
	}

	status.State = configv1alpha1.DeploymentState_DEPLOYMENT_STATE_IN_PROGRESS
	return c.deploymentStore.Put(ctx, deploymentID, status)
}

// CancelDeployment cancels a deployment
func (c *Controller) CancelDeployment(ctx context.Context, deploymentID string) error {
	c.mu.Lock()
	cancel, exists := c.activeDeployments[deploymentID]
	c.mu.Unlock()

	if exists {
		cancel()
	}

	status, err := c.deploymentStore.Get(ctx, deploymentID)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return fmt.Errorf("deployment not found: %s", deploymentID)
		}
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	status.State = configv1alpha1.DeploymentState_DEPLOYMENT_STATE_CANCELLED
	status.CompletedAt = timestamppb.Now()
	return c.deploymentStore.Put(ctx, deploymentID, status)
}

// ListDeployments lists all deployments, optionally filtered by state
func (c *Controller) ListDeployments(ctx context.Context, stateFilter *configv1alpha1.DeploymentState) ([]*configv1alpha1.DeploymentStatus, error) {
	deployments, err := c.deploymentStore.List(ctx)
	if err != nil {
		return nil, err
	}

	if stateFilter == nil {
		return deployments, nil
	}

	var filtered []*configv1alpha1.DeploymentStatus
	for _, d := range deployments {
		if d.GetState() == *stateFilter {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}
