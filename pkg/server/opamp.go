package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type slogWrapper struct {
	*slog.Logger
}

func (s *slogWrapper) Debugf(_ context.Context, format string, args ...any) {
	s.Logger.Debug(fmt.Sprintf(format, args...))
}

func (s *slogWrapper) Errorf(_ context.Context, format string, args ...any) {
	s.Logger.Error(fmt.Sprintf(format, args...))
}

type Server struct {
	logger   *slog.Logger
	opampSrv server.OpAMPServer
}

func NewServer(logger *slog.Logger) *Server {
	opampSvr := server.New(&slogWrapper{Logger: logger})
	return &Server{
		logger:   logger,
		opampSrv: opampSvr,
	}
}

func (s *Server) Start() error {
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

func (s *Server) Stop(ctx context.Context) error {
	if err := s.opampSrv.Stop(ctx); err != nil {
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
