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

	healthStore           storage.KeyValue[*protobufs.ComponentHealth]
	configStore           storage.KeyValue[*protobufs.EffectiveConfig]
	remoteStatusStore     storage.KeyValue[*protobufs.RemoteConfigStatus]
	opampAgentDescription storage.KeyValue[*protobufs.AgentDescription]
}

var _ services_int.OpAmpServerHandler = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	agentStore storage.KeyValue[*protobufs.AgentToServer],
	tracker AgentTracker,
	healthStore storage.KeyValue[*protobufs.ComponentHealth],
	configStore storage.KeyValue[*protobufs.EffectiveConfig],
	remoteStatusStore storage.KeyValue[*protobufs.RemoteConfigStatus],
	opampAgentDescriptionStore storage.KeyValue[*protobufs.AgentDescription],
) *Server {
	opampSvr := server.New(logutil.NewOpAMPLogger(l))
	s := &Server{
		logger:                l,
		opampSrv:              opampSvr,
		agentStore:            agentStore,
		addrToId:              map[string]string{},
		tracker:               tracker,
		healthStore:           healthStore,
		configStore:           configStore,
		remoteStatusStore:     remoteStatusStore,
		opampAgentDescription: opampAgentDescriptionStore,
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
	instanceUID := string(message.InstanceUid)
	agentAddr := conn.Connection().RemoteAddr().String()

	// Resolve the persistent agentID: extract from description or use cached mapping
	// FIXME: AgentDescription may not always be set
	agentID := s.resolveAgentID(agentAddr, message.AgentDescription)
	logger := s.logger.With("agent-id", agentID, "instance-uid", instanceUID)
	logger.Debug("received message from agent")

	// Update tracker with agentID so capabilities can be looked up by persistent ID
	var needsFullState bool
	if agentID != "" {
		needsFullState = s.tracker.UpdateFromMessage(agentID, message)
	}

	if agentID == "" {
		logger.Warn("cannot persist agent data: no agent ID available")
	} else {
		if message.AgentDescription != nil {
			logger.Info("persisting agent description")
			if err := s.opampAgentDescription.Put(ctx, agentID, message.AgentDescription); err != nil {
				logger.With("err", err).Error("failed to persist opamp agent-description")
			}
		}

		if message.Health != nil {
			logger.Info("persisting agent health")
			if err := s.healthStore.Put(ctx, agentID, message.Health); err != nil {
				logger.With("err", err).Error("failed to persist health")
			}
		}

		if message.EffectiveConfig != nil {
			logger.Info("persisting effective config")
			if err := s.configStore.Put(ctx, agentID, message.EffectiveConfig); err != nil {
				logger.With("err", err).Error("failed to persist effective config")
			}
		}

		if message.RemoteConfigStatus != nil {
			logger.Info("persisting remote config status")
			if err := s.remoteStatusStore.Put(ctx, agentID, message.RemoteConfigStatus); err != nil {
				logger.With("err", err).Error("failed to persist remote config status")
			}
		}
	}

	resp := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}

	if needsFullState {
		resp.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState)
		logger.Info("requesting full state report due to sequence gap")
	}

	return resp
}

// resolveAgentID returns the persistent agent ID, either by extracting it from the
// agent description or by looking it up from the address mapping.
func (s *Server) resolveAgentID(agentAddr string, desc *protobufs.AgentDescription) string {
	// Try to extract from description first
	if desc != nil {
		if agentID := extractAgentID(desc); agentID != "" {
			s.addrToId[agentAddr] = agentID
			s.tracker.PutStatus(agentID, &v1alpha1.AgentStatus{
				State: v1alpha1.AgentState_AGENT_STATE_CONNECTED,
			})
			return agentID
		}
	}
	// Fall back to cached mapping
	return s.addrToId[agentAddr]
}

// extractAgentID extracts the persistent otelfleet agent ID from the agent description.
func extractAgentID(desc *protobufs.AgentDescription) string {
	for _, entry := range desc.IdentifyingAttributes {
		if entry.Key == supervisor.AttributeOtelfleetAgentId {
			return entry.Value.GetStringValue()
		}
	}
	return ""
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
