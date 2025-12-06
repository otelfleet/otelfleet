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
	"github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	logger   *slog.Logger
	opampSrv server.OpAMPServer

	agentStore storage.KeyValue[*protobufs.AgentToServer]
	services.Service
}

func NewServer(
	l *slog.Logger,
	agentStore storage.KeyValue[*protobufs.AgentToServer],
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:     l,
		opampSrv:   opampSvr,
		agentStore: agentStore,
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
							OnMessage:         s.onMessage,
							OnConnectionClose: s.onDisconnect,
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
	ctxca, ca := context.WithTimeout(context.TODO(), time.Minute)
	defer ca()
	if err := s.opampSrv.Stop(ctxca); err != nil {
		return err
	}
	return nil
}

func (s *Server) onMessage(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	s.logger.Info("received message from agent", "message", message)
	return &protobufs.ServerToAgent{}
}

func (s *Server) onDisconnect(conn types.Connection) {
	remoteAddr := conn.Connection().RemoteAddr().String()
	localAddr := conn.Connection().LocalAddr().String()
	logger := s.logger.With("remote_addr", remoteAddr, "local_addr", localAddr)
	logger.Info("agent disconnected")
}
