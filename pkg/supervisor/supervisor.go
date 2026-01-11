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
}

func NewSupervisor(
	logger *slog.Logger,
	tlsConfig *tls.Config,
	opAmpAddr string,
	agentId ident.Identity,
) *Supervisor {
	return &Supervisor{
		logger:       logger,
		tlsConfig:    tlsConfig,
		clientLogger: logutil.NewOpAMPLogger(logger),
		opAmpAddr:    opAmpAddr,
		agentId:      agentId,
		startTime:    time.Now(),
		procManager: NewProcManager(
			logger.With("process", "otelcol"),
			//FIXME:
			"/home/alex/.asdf/shims/otelcol",
			"/var/lib/otelfleet/",
		),
	}
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
				// Report initial health on connect
				s.reportHealth()
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
	if err := s.opampClient.SetHealth(s.buildHealth()); err != nil {
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
		if err := s.procManager.Update(ctx, incomingCfg); err != nil {
			if err := s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				LastRemoteConfigHash: s.procManager.curHash,
				ErrorMessage:         err.Error(),
			}); err != nil {
				s.logger.With("err", err).With("status", "failed").Error("failed to report remote config status to upstream server")

				// panic("unhandled error")
			}
			return
		}
		if err := s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			LastRemoteConfigHash: incomingCfg.GetConfigHash(),
		}); err != nil {
			s.logger.With("err", err).With("status", "succeeded").Error("failed to report remote config status to upstream server")
			// panic("unhandled error")
		}
	}
	l.Debug("received message")
}

func (s *Supervisor) sendStatus(ctx context.Context) {
	// TODO
	// msg := &protobufs.AgentToServer{
	// 	InstanceUid:        nil,
	// 	AgentDescription:   nil,
	// 	Capabilities:       0,
	// 	Health:             &protobufs.ComponentHealth{},
	// 	EffectiveConfig:    &protobufs.EffectiveConfig{},
	// 	RemoteConfigStatus: &protobufs.RemoteConfigStatus{},
	// 	PackageStatuses:    &protobufs.PackageStatuses{},
	// }
}

// onServerStatusAck needs to handle success / failure of processing of agent status
// upstream.
func (s *Supervisor) onServerStatusAck(ctx context.Context) {

}

func (s *Supervisor) Shutdown() error {
	return s.opampClient.Stop(context.TODO())
}

func (s *Supervisor) createEffectiveConfigMsg() *protobufs.EffectiveConfig {
	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"default": {
					Body: []byte(
						"key : val",
					),
					ContentType: "text/yaml",
				},
			},
		},
	}
}

func (s *Supervisor) createAgentDescription() *protobufs.AgentDescription {
	agentID := s.agentId.UniqueIdentifier().UUID
	s.logger.With("agentID", agentID).Info("sending agent description")
	// TODO : this will need to include the identifying resource attributes from the otel collector
	// the idea being that we can then lookup the metrics,logs,traces from this particular collector
	// instance from the gateway in their respective storage backends.
	return BuildAgentDescription(agentID)
}

// buildHealth creates the current health status for the supervisor.
func (s *Supervisor) buildHealth() *protobufs.ComponentHealth {
	// TODO : this will need to check proc manager status
	// Also check if the deployment collector's packages include healthcheckextensionv2
	// and if it is loaded - so it can call that endpoint.
	return BuildComponentHealth(true, "running", s.startTime)
}

// reportHealth sends the current health status to the server.
func (s *Supervisor) reportHealth() {
	if err := s.opampClient.SetHealth(s.buildHealth()); err != nil {
		s.logger.With("err", err).Warn("failed to report health")
	}
}
