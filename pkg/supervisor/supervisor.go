package supervisor

import (
	"context"
	"crypto/tls"
	"log/slog"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/logger"
	"github.com/otelfleet/otelfleet/pkg/util"
)

type Supervisor struct {
	logger       *slog.Logger
	clientLogger types.Logger

	tlsConfig *tls.Config

	opampClient client.OpAMPClient
}

func NewSupervisor(
	log *slog.Logger,
	tlsConfig *tls.Config,
) *Supervisor {
	return &Supervisor{
		logger:       log,
		tlsConfig:    tlsConfig,
		clientLogger: logger.NewOpAMPLogger(log),
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
		OpAMPServerURL: "wss://127.0.0.1:4320/v1/opamp",
		TLSConfig:      s.tlsConfig,
		InstanceUid:    types.InstanceUid([]byte(uuid.New().String())),
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
	s.logger.Info("received message")
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
	return &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			util.KeyVal("agent.id", "example"),
		},
	}
}
