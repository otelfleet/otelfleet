package resource

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1/v1alpha1connect"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/util/protoutil"
)

type Server struct {
	services.Service
	genericStorage schema.SchemaProto
	supportedTypes []string
}

var _ otelfleet_svc.HTTPExtension = (*Server)(nil)

func NewServer(
	l *slog.Logger,
	genericStorage schema.SchemaProto,
) *Server {
	s := &Server{
		genericStorage: genericStorage,
	}

	s.Service = services.NewBasicService(s.start, s.running, s.stop)
	s.supportedTypes = s.SupportedResources()
	return s
}

func (s *Server) start(ctx context.Context) error {
	return nil
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) stop(error) error {
	return nil
}

func (s *Server) ConfigureHTTP(mux *mux.Router, opts []connect.HandlerOption) {
	v1alpha1connect.RegisterResourceServiceHandler(mux, s, opts...)
}

// Returns the typeURLs of supported resources
func (s *Server) SupportedResources() []string {
	return []string{
		protoutil.GetTypeURL(&v1alpha1.Receiver{}),
		protoutil.GetTypeURL(&v1alpha1.ReceiverCollection{}),
		protoutil.GetTypeURL(&v1alpha1.Processor{}),
		protoutil.GetTypeURL(&v1alpha1.ProcessorCollection{}),
		protoutil.GetTypeURL(&v1alpha1.Exporter{}),
		protoutil.GetTypeURL(&v1alpha1.ExporterCollection{}),
		protoutil.GetTypeURL(&v1alpha1.Connector{}),
		protoutil.GetTypeURL(&v1alpha1.ConnectorCollection{}),
		protoutil.GetTypeURL(&v1alpha1.Extension{}),
		protoutil.GetTypeURL(&v1alpha1.ExtensionCollection{}),
		protoutil.GetTypeURL(&v1alpha1.Pipeline{}),
		protoutil.GetTypeURL(&v1alpha1.PipelineCollection{}),
		protoutil.GetTypeURL(&v1alpha1.CollectorConfig{}),
		protoutil.GetTypeURL(&v1alpha1.ConfigFilter{}),
	}
}
