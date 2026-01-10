package supervisor

import (
	"context"
	"crypto/tls"
	"log/slog"

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

	agentId ident.Identity

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
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig,
		Callbacks: types.Callbacks{
			OnConnect: func(ctx context.Context) {
				s.logger.Info("connected to OpAMP server")
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

	err := s.opampClient.SetAgentDescription(s.createAgentDescription())
	if err != nil {
		return err
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

				panic("unhandled error")
			}
		} else {
			if err := s.opampClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
				LastRemoteConfigHash: incomingCfg.GetConfigHash(),
			}); err != nil {
				s.logger.With("err", err).With("status", "succeeded").Error("failed to report remote config status to upstream server")
				panic("unhandled error")
			}
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
	s.logger.With("agentID", s.agentId.UniqueIdentifier().UUID).Info("sending agent description")
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			// TODO : semconv for other identifying attributes
			util.KeyVal(AttributeOtelfleetAgentId, s.agentId.UniqueIdentifier().UUID),
		},
	}
}
