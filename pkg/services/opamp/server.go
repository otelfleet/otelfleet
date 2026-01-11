package opamp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/grafana/dskit/services"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/logutil"
	services_int "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/services/otelconfig"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger   *slog.Logger
	opampSrv server.OpAMPServer

	agentStore storage.KeyValue[*protobufs.AgentToServer]
	services.Service

	addrToId map[string]string
	tracker  AgentTracker

	healthStore       storage.KeyValue[*protobufs.ComponentHealth]
	configStore       storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus]
}

var _ services_int.OpAmpServerHandler = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	agentStore storage.KeyValue[*protobufs.AgentToServer],
	tracker AgentTracker,
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	configStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:            l,
		opampSrv:          opampSvr,
		agentStore:        agentStore,
		addrToId:          map[string]string{},
		tracker:           tracker,
		healthStore:       healthStore,
		configStore:       configStore,
		remoteStatusStore: remoteStatusStore,
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	return s
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) start(ctx context.Context) error {
	addr := "127.0.0.1:4320"
	s.logger.With("addr", addr).Info("starting opamp server")
	settings := server.StartSettings{
		ListenEndpoint: addr,
		HTTPMiddleware: otelhttp.NewMiddleware("v1/opamp"),
		Settings: server.Settings{
			Callbacks: types.Callbacks{
				OnConnecting: func(request *http.Request) types.ConnectionResponse {
					return types.ConnectionResponse{
						Accept: true,
						ConnectionCallbacks: types.ConnectionCallbacks{
							OnConnected:        s.OnConnected,
							OnMessage:          s.OnMessage,
							OnConnectionClose:  s.OnConnectionClose,
							OnReadMessageError: s.OnReadMessageError,
						},
					}
				},
			},
		},
	}
	if err := s.opampSrv.Start(settings); err != nil {
		s.logger.With("err", err.Error()).Error("failed to start opamp server")
	}

	return nil
}

func (s *Server) stop(failureCase error) error {
	ctxca, ca := context.WithTimeout(context.TODO(), time.Second)
	defer ca()
	return s.opampSrv.Stop(ctxca)
}

func (s *Server) OnConnected(ctx context.Context, conn types.Connection) {
	s.logger.With("addr", conn.Connection().LocalAddr().String()).Info("agent connected")

	if err := s.sendConfig(ctx, conn); err != nil {
		s.logger.With("err", err).Error("failed to send remote config")
		panic("unhandled error")
	}
}

func (s *Server) sendConfig(ctx context.Context, conn types.Connection) error {
	s.logger.Log(ctx, logutil.LevelTrace, "sending config to agent")
	return conn.Send(ctx, &protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"config.yaml": {
						ContentType: "text/yaml",
						Body:        []byte(otelconfig.DefaultOtelConfig),
					},
				},
			},
			// TODO
			ConfigHash: []byte("a"),
		},
	})
}

func (s *Server) OnReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
	s.logger.
		With("remote-addr", conn.Connection().RemoteAddr().String()).
		With("msg", string(msgByte)).
		With("err", err).
		Error("failed to read / deserialize agent message")
}

func (s *Server) OnMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	s.logger.Info("received message from agent", "message", message)

	instanceUID := string(message.InstanceUid)
	agentAddr := conn.Connection().RemoteAddr().String()

	needsFullState := s.tracker.UpdateFromMessage(instanceUID, message)

	if message.Health != nil {
		if err := s.healthStore.Put(ctx, instanceUID, message.Health); err != nil {
			s.logger.Error("failed to persist health", "instanceUID", instanceUID, "err", err)
		}
	}

	if message.EffectiveConfig != nil {
		if err := s.configStore.Put(ctx, instanceUID, message.EffectiveConfig); err != nil {
			s.logger.Error("failed to persist effective config", "instanceUID", instanceUID, "err", err)
		}
	}

	if message.RemoteConfigStatus != nil {
		if err := s.remoteStatusStore.Put(ctx, instanceUID, message.RemoteConfigStatus); err != nil {
			s.logger.Error("failed to persist remote config status", "instanceUID", instanceUID, "err", err)
		}
	}

	if message.AgentDescription != nil {
		s.handleAgentDescription(agentAddr, message.AgentDescription)
	}

	resp := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}

	if needsFullState {
		resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
		s.logger.Info("requesting full state report due to sequence gap", "instanceUID", instanceUID)
	}

	return resp
}

func (s *Server) handleAgentDescription(agentAddr string, desc *protobufs.AgentDescription) {
	keyvalues := desc.IdentifyingAttributes
	for _, entry := range keyvalues {
		if entry.Key == supervisor.AttributeOtelfleetAgentId {
			agentID := entry.Value.GetStringValue()
			s.logger.With("agentID", agentID, "remote-addr", agentAddr).
				Info("associating agent remote-addr to persistent ID")
			s.addrToId[agentAddr] = agentID
			s.tracker.PutStatus(agentID, &v1alpha1.AgentStatus{
				State: v1alpha1.AgentState_AGENT_STATE_CONNECTED,
			})
			return
		}
	}
	s.logger.Warn("received agent description but performed no actions")
}

func (s *Server) OnConnectionClose(conn types.Connection) {
	remoteAddr := conn.Connection().RemoteAddr().String()
	logger := s.logger.With("remote_addr", remoteAddr)
	logger.Info("agent disconnected")
	agentID, ok := s.addrToId[remoteAddr]
	if !ok {
		logger.Error("agent not tracked in addr to persistent ID map")
		return
	}
	s.tracker.PutStatus(agentID, &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AGENT_STATE_DISCONNECTED,
	})
}
