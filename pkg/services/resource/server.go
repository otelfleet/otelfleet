package resource

import (
	"context"
	"log/slog"

	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/util/protoutil"
)

type Server struct {
	services.Service
	genericStorage schema.SchemaProto
	supportedTypes []string
}

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

func (s *Server) ConfigureHTTP(mux *mux.Router) {
	v1alpha1connect.RegisterResourceServiceHandler(mux, s)
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
	}
}
