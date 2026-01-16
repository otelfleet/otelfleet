package supervisor

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/util"
)

type Supervisor struct {
	logger       *slog.Logger
	clientLogger types.Logger

	tlsConfig *tls.Config

	opampClient client.OpAMPClient
	opAmpAddr   string

	agentId   ident.Identity
	startTime time.Time

	// for direct in-process management
	procManager *ProcManager
	appliedHash string
}

func NewSupervisor(
	logger *slog.Logger,
	tlsConfig *tls.Config,
	opAmpAddr string,
	agentId ident.Identity,
) *Supervisor {
	s := &Supervisor{
		logger:       logger,
		tlsConfig:    tlsConfig,
		clientLogger: logutil.NewOpAMPLogger(logger),
		opAmpAddr:    opAmpAddr,
		agentId:      agentId,
		startTime:    time.Now(),
	}
	s.procManager = NewProcManager(
		logger.With("process", "otelcol"),
		//FIXME:
		"/home/alex/.asdf/shims/otelcol",
		"/var/lib/otelfleet/",
		s.reportHealth,
	)
	return s
}

func (s *Supervisor) Start() error {
	if err := s.startOpAMP(); err != nil {
		return err
	}
	return nil
}

func (s *Supervisor) startOpAMP() error {
	s.opampClient = client.NewWebSocket(s.clientLogger)
	settings := types.StartSettings{
		OpAMPServerURL: s.opAmpAddr,
		TLSConfig:      s.tlsConfig,
		InstanceUid:    types.InstanceUid([]byte(util.NewUUID())),
		Capabilities:   protobufs.AgentCapabilities(GetCapabilities()),
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				s.logger.Info("connected to OpAMP server")
				s.reportHealth(true, "connected", "")
			},
			OnConnectFailed: func(ctx context.Context, err error) {
				s.logger.With("err", err).Error("failed to connect to the server")
			},
			OnError: func(ctx context.Context, err *protobufs.ServerErrorResponse) {
				s.logger.With("err", err).Error("Server returned an error response")
			},
			GetEffectiveConfig: func(ctx context.Context) (*protobufs.EffectiveConfig, error) {
				return s.createEffectiveConfigMsg(), nil
			},
			OnMessage: s.onMessage,
		},
	}

	// Use enhanced agent description
	err := s.opampClient.SetAgentDescription(s.createAgentDescription())
	if err != nil {
		return err
	}

	// Set initial health status
	if err := s.opampClient.SetHealth(s.buildHealth(
		true,
		"initialized",
		"",
	)); err != nil {
		s.logger.With("err", err).Warn("failed to set initial health")
	}

	if err := s.opampClient.Start(context.TODO(), settings); err != nil {
		return err
	}

	return nil
}

func (s *Supervisor) onMessage(ctx context.Context, msg *types.MessageData) {
	l := s.logger
	if incomingCfg := msg.RemoteConfig; incomingCfg != nil {
		l = l.With("type", "remote-config")
		l.Info("updatading effective configuration")
		if err := s.procManager.Update(ctx, incomingCfg); err != nil {
			// TODO : only send failed apply when the write to disk fails in proc manager
			if err := s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				LastRemoteConfigHash: s.procManager.curHash,
				ErrorMessage:         err.Error(),
			}); err != nil {
				l.With("err", err).With("status", "failed").Error("failed to report remote config status to upstream server")
			}
			return
		}
		if err := s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			LastRemoteConfigHash: incomingCfg.GetConfigHash(),
		}); err != nil {
			l.With("err", err).With("status", "succeeded").Error("failed to report remote config status to upstream server")
		}
	}
	l.Debug("received message")
}

func (s *Supervisor) Shutdown() error {
	return s.opampClient.Stop(context.TODO())
}

func (s *Supervisor) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	contents, err := s.procManager.getConfigMap()
	if err != nil {
		s.logger.With("err", err).Error("failed to get effective config from proc manager")
		return defaultEffectiveConfig
	}
	return &protobufs.EffectiveConfig{
		ConfigMap: contents,
	}
}

func (s *Supervisor) createAgentDescription() *protobufs.AgentDescription {
	agentID := s.agentId.UniqueIdentifier().UUID
	s.logger.With("agentID", agentID).Info("sending agent description")
	// TODO : this will need to include the identifying resource attributes from the otel collector
	// the idea being that we can then lookup the metrics,logs,traces from this particular collector
	// instance from the gateway in their respective storage backends.
	return s.buildAgentDescription(agentID)
}

// buildHealth creates the current health status for the supervisor.
func (s *Supervisor) buildHealth(
	healthy bool,
	status,
	lastErrorMessage string,
) *protobufs.ComponentHealth {
	// TODO : this will need to check proc manager status
	// Also check if the deployment collector's packages include healthcheckextensionv2
	// and if it is loaded - so it can call that endpoint.
	return s.buildComponentHealth(healthy, status, lastErrorMessage, s.startTime)
}

// reportHealth sends the current health status to the server.
func (s *Supervisor) reportHealth(
	healthy bool,
	status string,
	lastErrorMessage string,
) {
	if err := s.opampClient.SetHealth(s.buildHealth(healthy, status, lastErrorMessage)); err != nil {
		s.logger.With("err", err).Warn("failed to report health")
	}
}

var defaultEffectiveConfig = &protobufs.EffectiveConfig{
	ConfigMap: &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"default": {
				Body:        []byte("none"),
				ContentType: "text/yaml",
			},
		},
	},
}
