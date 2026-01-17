package testutil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/gorilla/mux"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/services/agent"
	"github.com/otelfleet/otelfleet/pkg/services/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/services/deployment"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/require"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// TestEnv provides a complete OtelFleet server environment for integration testing.
// All KV stores and services are exposed for direct test access.
type TestEnv struct {
	// Storage
	db     *pebble.DB
	Broker storage.KVBroker

	// KV Stores - all exposed for direct test manipulation
	TokenStore                 storage.KeyValue[*bootstrapv1alpha1.BootstrapToken]
	AgentStore                 storage.KeyValue[*agentsv1alpha1.AgentDescription]
	OpampAgentStore            storage.KeyValue[*protobufs.AgentToServer]
	ConfigStore                storage.KeyValue[*configv1alpha1.Config]
	DefaultConfigStore         storage.KeyValue[*configv1alpha1.Config]
	BootstrapConfigStore       storage.KeyValue[*configv1alpha1.Config]
	AssignedConfigStore        storage.KeyValue[*configv1alpha1.Config]
	ConfigAssignmentStore      storage.KeyValue[*configv1alpha1.ConfigAssignment]
	HealthStore                storage.KeyValue[*protobufs.ComponentHealth]
	EffectiveConfigStore       storage.KeyValue[*protobufs.EffectiveConfig]
	RemoteStatusStore          storage.KeyValue[*protobufs.RemoteConfigStatus]
	OpampAgentDescriptionStore storage.KeyValue[*protobufs.AgentDescription]
	DeploymentStore            storage.KeyValue[*configv1alpha1.DeploymentStatus]
	AgentDeploymentStore       storage.KeyValue[*configv1alpha1.AgentDeploymentStatus]
	// ConnectionStateStore replaces the in-memory AgentTracker
	ConnectionStateStore storage.KeyValue[*agentsv1alpha1.AgentConnectionState]

	// Agent Repository - unified access to agent data
	AgentRepo agentdomain.Repository

	// Services
	BootstrapServer      *bootstrap.BootstrapServer
	ConfigServer         *otelconfig.ConfigServer
	OpampServer          *opamp.Server
	AgentServer          *agent.AgentServer
	DeploymentController *deployment.Controller

	// HTTP
	HTTPServer    *httptest.Server
	OpampWSServer *httptest.Server
	BaseURL       string
	OpampURL      string

	// Private key for bootstrap signing
	PrivateKey crypto.Signer

	// Logger
	Logger *slog.Logger

	// Test context
	t *testing.T

	// Track test agents
	mu     sync.Mutex
	agents map[string]*TestAgent
}

// NewTestEnv creates a new test environment with all services initialized.
// The environment uses in-memory storage and httptest servers.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create in-memory Pebble database
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	require.NoError(t, err)

	broker := otelpebble.NewKVBroker(db)
	logger := slog.Default()

	// Generate a test RSA key for bootstrap signing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	env := &TestEnv{
		db:         db,
		Broker:     broker,
		Logger:     logger,
		PrivateKey: privateKey,
		t:          t,
		agents:     make(map[string]*TestAgent),
	}

	// Initialize all KV stores
	env.initStores(logger, broker)

	// Initialize services
	env.initServices(logger, privateKey)

	// Wire up service dependencies
	env.wireServices()

	// Setup HTTP servers
	env.setupHTTPServers(t)

	// Register cleanup
	t.Cleanup(func() {
		env.Close()
	})

	return env
}

func (e *TestEnv) initStores(logger *slog.Logger, broker storage.KVBroker) {
	e.TokenStore = storage.NewProtoKV[*bootstrapv1alpha1.BootstrapToken](logger, broker.KeyValue("tokens"))
	e.AgentStore = storage.NewProtoKV[*agentsv1alpha1.AgentDescription](logger, broker.KeyValue("agents"))
	e.OpampAgentStore = storage.NewProtoKV[*protobufs.AgentToServer](logger, broker.KeyValue("opamp-agents"))
	e.ConfigStore = storage.NewProtoKV[*configv1alpha1.Config](logger, broker.KeyValue("configs"))
	e.DefaultConfigStore = storage.NewProtoKV[*configv1alpha1.Config](logger, broker.KeyValue("default-configs"))
	e.BootstrapConfigStore = storage.NewProtoKV[*configv1alpha1.Config](logger, broker.KeyValue("bootstrap-configs"))
	e.AssignedConfigStore = storage.NewProtoKV[*configv1alpha1.Config](logger, broker.KeyValue("assigned-configs"))
	e.ConfigAssignmentStore = storage.NewProtoKV[*configv1alpha1.ConfigAssignment](logger, broker.KeyValue("config-assignments"))
	e.HealthStore = storage.NewProtoKV[*protobufs.ComponentHealth](logger, broker.KeyValue("agent-health"))
	e.EffectiveConfigStore = storage.NewProtoKV[*protobufs.EffectiveConfig](logger, broker.KeyValue("effective-config"))
	e.RemoteStatusStore = storage.NewProtoKV[*protobufs.RemoteConfigStatus](logger, broker.KeyValue("remote-config-status"))
	e.OpampAgentDescriptionStore = storage.NewProtoKV[*protobufs.AgentDescription](logger, broker.KeyValue("opamp-agent-description"))
	e.DeploymentStore = storage.NewProtoKV[*configv1alpha1.DeploymentStatus](logger, broker.KeyValue("deployments"))
	e.AgentDeploymentStore = storage.NewProtoKV[*configv1alpha1.AgentDeploymentStatus](logger, broker.KeyValue("agent-deployments"))
	e.ConnectionStateStore = storage.NewProtoKV[*agentsv1alpha1.AgentConnectionState](logger, broker.KeyValue("connection-state"))

	// Create the agent repository with all stores
	e.AgentRepo = agentdomain.NewRepository(
		logger.With("component", "agent-repository"),
		e.AgentStore,
		e.OpampAgentDescriptionStore,
		e.ConnectionStateStore,
		e.HealthStore,
		e.EffectiveConfigStore,
		e.RemoteStatusStore,
		e.ConfigAssignmentStore,
	)
}

func (e *TestEnv) initServices(logger *slog.Logger, privateKey crypto.Signer) {
	// BootstrapServer
	e.BootstrapServer = bootstrap.NewBootstrapServer(
		logger.With("service", "bootstrap"),
		privateKey,
		e.TokenStore,
		e.AgentRepo,
		e.ConfigStore,
		e.BootstrapConfigStore,
		e.AssignedConfigStore,
	)

	// ConfigServer
	e.ConfigServer = otelconfig.NewConfigServer(
		logger.With("service", "config"),
		e.ConfigStore,
		e.DefaultConfigStore,
		e.AssignedConfigStore,
		e.ConfigAssignmentStore,
		e.AgentRepo,
		e.EffectiveConfigStore,
		e.RemoteStatusStore,
	)

	// OpampServer - uses repository for agent data access
	e.OpampServer = opamp.NewServer(
		logger.With("service", "opamp"),
		e.AgentRepo,
		e.AssignedConfigStore,
	)

	// AgentServer - uses repository for agent data access
	e.AgentServer = agent.NewAgentServer(
		logger.With("service", "agent"),
		e.AgentRepo,
	)

	// DeploymentController
	e.DeploymentController = deployment.NewController(
		logger.With("service", "deployment"),
		e.DeploymentStore,
		e.AgentDeploymentStore,
		e.ConfigStore,
		e.AgentRepo,
	)
}

func (e *TestEnv) wireServices() {
	// ConfigServer notifies OpampServer of config changes
	e.ConfigServer.SetNotifier(e.OpampServer)

	// ConfigServer uses DeploymentController for rolling deployments
	e.ConfigServer.SetDeploymentController(e.DeploymentController)

	// DeploymentController uses ConfigServer for assigning configs
	e.DeploymentController.SetConfigAssigner(e.ConfigServer)
}

func (e *TestEnv) setupHTTPServers(t *testing.T) {
	// Create HTTP router and register services
	router := mux.NewRouter()
	e.BootstrapServer.ConfigureHTTP(router)
	e.ConfigServer.ConfigureHTTP(router)
	e.AgentServer.ConfigureHTTP(router)

	// Create HTTP test server
	e.HTTPServer = httptest.NewServer(router)
	e.BaseURL = e.HTTPServer.URL

	// Create separate OpAMP WebSocket test server
	opampSrv := server.New(nil)
	settings := SetupOpampServerImpl(t, e.OpampServer)
	handlerFunc, _, err := opampSrv.Attach(settings)
	require.NoError(t, err)

	e.OpampWSServer = httptest.NewServer(http.HandlerFunc(handlerFunc))
	e.OpampURL = "ws" + e.OpampWSServer.URL[4:] // Convert http:// to ws://
}

// Close cleans up all test environment resources.
func (e *TestEnv) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Stop all agents
	for _, agent := range e.agents {
		_ = agent.Stop()
	}

	// Close HTTP servers
	if e.HTTPServer != nil {
		e.HTTPServer.Close()
	}
	if e.OpampWSServer != nil {
		e.OpampWSServer.Close()
	}

	// Close database
	if e.db != nil {
		_ = e.db.Close()
	}
}

// GetAgent returns a previously created test agent by ID.
func (e *TestEnv) GetAgent(agentID string) (*TestAgent, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	agent, ok := e.agents[agentID]
	return agent, ok
}

// ListAgentIDs returns the IDs of all created test agents.
func (e *TestEnv) ListAgentIDs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	ids := make([]string, 0, len(e.agents))
	for id := range e.agents {
		ids = append(ids, id)
	}
	return ids
}
