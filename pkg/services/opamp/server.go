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
}

var _ services_int.OpAmpServerHandler = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	agentStore storage.KeyValue[*protobufs.AgentToServer],
	tracker AgentTracker,
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:     l,
		opampSrv:   opampSvr,
		agentStore: agentStore,
		addrToId:   map[string]string{},
		tracker:    tracker,
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
	// FIXME: handle failure case?
	ctxca, ca := context.WithTimeout(context.TODO(), time.Second)
	defer ca()
	if err := s.opampSrv.Stop(ctxca); err != nil {
		return err
	}
	return nil
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
	if err := conn.Send(ctx, &protobufs.ServerToAgent{
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
	}); err != nil {
		return err
	}
	return nil
}

// onMessageReadFailurefunc is called when an error occurs while reading or deserializing a message.
func (s *Server) OnReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
	s.logger.
		With("remote-addr", conn.Connection().RemoteAddr().String()).
		With("msg", string(msgByte)).
		With("err", err).
		Error("failed to read / deserialize agent message")
}

func (s *Server) OnMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	s.logger.Info("received message from agent", "message", message)
	if message.AgentDescription != nil {
		agentAddr := conn.Connection().RemoteAddr().String()
		s.handleAgentDescription(agentAddr, message.AgentDescription)
	}
	return &protobufs.ServerToAgent{}
}

func (s *Server) handleAgentDescription(agentAddr string, desc *protobufs.AgentDescription) {
	keyvalues := desc.IdentifyingAttributes
	for _, entry := range keyvalues {
		if entry.Key == supervisor.AttributeOtelfleetAgentId {
			agentID := entry.Value.GetStringValue()
			s.logger.With("agentID", agentID, "remote-addr", agentAddr).
				Info("associating agent remote-addr to persistent ID")
			s.addrToId[agentAddr] = agentID
			s.tracker.Put(agentID, &v1alpha1.AgentStatus{
				State: v1alpha1.AgentState_AgentStateConnected,
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
	s.tracker.Put(agentID, &v1alpha1.AgentStatus{
		State: v1alpha1.AgentState_AgentStateDisconnected,
	})
}
