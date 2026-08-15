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
	servertypes "github.com/open-telemetry/opamp-go/server/types"
	agentsv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/config"
	agentdomain "github.com/otelfleet/otelfleet/pkg/domain/agent"
	"github.com/otelfleet/otelfleet/pkg/services/agent"
	"github.com/otelfleet/otelfleet/pkg/services/authorization"
	"github.com/otelfleet/otelfleet/pkg/services/opamp"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
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
	Broker types.KVBroker

	// KV Stores - all exposed for direct test manipulation
	TokenStore                 types.KeyValue[*bootstrapv1alpha1.BootstrapToken]
	AgentStore                 types.KeyValue[*agentsv1alpha1.AgentDescription]
	OpampAgentStore            types.KeyValue[*protobufs.AgentToServer]
	ConfigStore                types.KeyValue[*configv1alpha1.Config]
	DefaultConfigStore         types.KeyValue[*configv1alpha1.Config]
	BootstrapConfigStore       types.KeyValue[*configv1alpha1.Config]
	AssignedConfigStore        types.KeyValue[*configv1alpha1.Config]
	ConfigAssignmentStore      types.KeyValue[*configv1alpha1.ConfigAssignment]
	HealthStore                types.KeyValue[*protobufs.ComponentHealth]
	EffectiveConfigStore       types.KeyValue[*protobufs.EffectiveConfig]
	RemoteStatusStore          types.KeyValue[*protobufs.RemoteConfigStatus]
	OpampAgentDescriptionStore types.KeyValue[*protobufs.AgentDescription]
	DeploymentStore            types.KeyValue[*configv1alpha1.DeploymentStatus]
	AgentDeploymentStore       types.KeyValue[*configv1alpha1.AgentDeploymentStatus]
	// ConnectionStateStore replaces the in-memory AgentTracker
	ConnectionStateStore types.KeyValue[*agentsv1alpha1.AgentConnectionState]

	// Agent Repository - unified access to agent data
	AgentRepo agentdomain.Repository

	// Services
	BootstrapServer *authorization.BootstrapServer
	ConfigServer    *otelconfig.ConfigServer
	OpampServer     *opamp.Server
	AgentServer     *agent.AgentServer

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

func (e *TestEnv) initStores(logger *slog.Logger, broker types.KVBroker) {
	e.TokenStore = storage.NewProtoKVFromSchemaImpl[*bootstrapv1alpha1.BootstrapToken](schema.NewStorageSchemaProto(broker.KeyValue("tokens")))
	e.AgentStore = storage.NewProtoKVFromSchemaImpl[*agentsv1alpha1.AgentDescription](schema.NewStorageSchemaProto(broker.KeyValue("agents")))
	e.OpampAgentStore = storage.NewProtoKVFromSchemaImpl[*protobufs.AgentToServer](schema.NewStorageSchemaProto(broker.KeyValue("opamp-agents")))
	e.ConfigStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.Config](schema.NewStorageSchemaProto(broker.KeyValue("configs")))
	e.DefaultConfigStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.Config](schema.NewStorageSchemaProto(broker.KeyValue("default-configs")))
	e.BootstrapConfigStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.Config](schema.NewStorageSchemaProto(broker.KeyValue("bootstrap-configs")))
	e.AssignedConfigStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.Config](schema.NewStorageSchemaProto(broker.KeyValue("assigned-configs")))
	e.ConfigAssignmentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.ConfigAssignment](schema.NewStorageSchemaProto(broker.KeyValue("config-assignments")))
	e.HealthStore = storage.NewProtoKVFromSchemaImpl[*protobufs.ComponentHealth](schema.NewStorageSchemaProto(broker.KeyValue("agent-health")))
	e.EffectiveConfigStore = storage.NewProtoKVFromSchemaImpl[*protobufs.EffectiveConfig](schema.NewStorageSchemaProto(broker.KeyValue("effective-config")))
	e.RemoteStatusStore = storage.NewProtoKVFromSchemaImpl[*protobufs.RemoteConfigStatus](schema.NewStorageSchemaProto(broker.KeyValue("remote-config-status")))
	e.OpampAgentDescriptionStore = storage.NewProtoKVFromSchemaImpl[*protobufs.AgentDescription](schema.NewStorageSchemaProto(broker.KeyValue("opamp-agent-description")))
	e.DeploymentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.DeploymentStatus](schema.NewStorageSchemaProto(broker.KeyValue("deployments")))
	e.AgentDeploymentStore = storage.NewProtoKVFromSchemaImpl[*configv1alpha1.AgentDeploymentStatus](schema.NewStorageSchemaProto(broker.KeyValue("agent-deployments")))
	e.ConnectionStateStore = storage.NewProtoKVFromSchemaImpl[*agentsv1alpha1.AgentConnectionState](schema.NewStorageSchemaProto(broker.KeyValue("connection-state")))

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
	e.BootstrapServer = authorization.NewBootstrapServer(
		logger.With("service", "bootstrap"),
		privateKey,
		e.TokenStore,
		e.AgentRepo,
		e.ConfigStore,
		e.BootstrapConfigStore,
		e.AssignedConfigStore,
	)

	// OpampServer - uses repository for agent data access
	e.OpampServer = opamp.NewServer(
		logger.With("service", "opamp"),
		e.AgentRepo,
		e.AssignedConfigStore,
		//FIXME:
		"",
		&config.OTLPConfig{},
	)

	// AgentServer - uses repository for agent data access
	e.AgentServer = agent.NewAgentServer(
		logger.With("service", "agent"),
		e.AgentRepo,
	)

	// DeploymentController
	// e.DeploymentController = deployment.NewController(
	// 	logger.With("service", "deployment"),
	// 	e.DeploymentStore,
	// 	e.AgentDeploymentStore,
	// 	e.ConfigStore,
	// 	e.AgentRepo,
	// )
}

func (e *TestEnv) wireServices() {
	// // ConfigServer notifies OpampServer of config changes
	// e.ConfigServer.SetNotifier(e.OpampServer)

	// // ConfigServer uses DeploymentController for rolling deployments
	// e.ConfigServer.SetDeploymentController(e.DeploymentController)

	// // DeploymentController uses ConfigServer for assigning configs
	// e.DeploymentController.SetConfigAssigner(e.ConfigServer)
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

	// Create separate OpAMP WebSocket test server. The Server wires a fresh
	// per-connection ServerAgentHandler for each incoming connection via
	// OnConnecting, mirroring the production start path.
	opampSrv := server.New(nil)
	settings := server.Settings{
		Callbacks: servertypes.Callbacks{
			OnConnecting: e.OpampServer.OnConnecting,
		},
	}
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
