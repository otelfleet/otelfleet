package storage

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/cockroachdb/pebble/v2"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/config"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/kv/driver/pebble"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/storage/object/driver/basekv"
	otelgrpc "github.com/otelfleet/otelfleet/pkg/storage/object/driver/grpc"
	"github.com/otelfleet/otelfleet/pkg/storage/transport"
)

type StorageService struct {
	logger *slog.Logger

	protoStore object.TypeURLStore
	cfg        *config.StorageConfig

	services.Service

	// set if filesystem is set
	pebbleDB *pebble.DB
	v1alpha1connect.KeyValueServiceHandler
}

var _ services.Service = (*StorageService)(nil)
var _ otelfleet_svc.HTTPExtension = (*StorageService)(nil)

// var _ types.KVBroker = (*StorageService)(nil)

func NewStorageService(
	logger *slog.Logger,
	cfg *config.StorageConfig,
) (*StorageService, error) {
	s := &StorageService{
		logger: logger,
		cfg:    cfg,
	}
	var protoStore object.TypeURLStore
	if cfg.File != nil {
		// TODO : this setup logic is a little wonky...
		kvDb, err := otelpebble.Open(
			cfg.File.Path,
			nil,
		)
		if err != nil {
			logger.Error("failed to start filesystem KV store")
			return nil, err
		}
		s.pebbleDB = kvDb
		broker := otelpebble.NewKVBroker(kvDb)
		kv := broker.KeyValue("")
		protoSchema := basekv.NewTypeURLStore(kv)
		protoStore = basekv.NewTypeURLStore(kv)
		s.KeyValueServiceHandler = transport.NewKVServer(protoSchema)
	}
	if cfg.Client != nil {
		logger.With("client-addr", cfg.Client.HttpAddr).Info("starting storage in remote mode")
		client := v1alpha1connect.NewKeyValueServiceClient(
			http.DefaultClient,
			// TODO : we need to validate the address is valid
			"http://"+cfg.Client.HttpAddr,
			connect.WithHTTPGet(),
		)
		protoStore = otelgrpc.NewRemoteKV(client)
		s.KeyValueServiceHandler = transport.NewErroringServer(connect.CodeUnimplemented, fmt.Errorf("unimplemented"))
	}
	s.protoStore = protoStore
	s.Service = services.NewBasicService(s.starting, s.running, s.stopping)
	return s, nil
}

func (s *StorageService) starting(_ context.Context) error {
	return nil
}

func (s *StorageService) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *StorageService) stopping(_ error) error {
	if s.cfg.File != nil && s.pebbleDB != nil {
		// TODO ? handle failure case
		return s.pebbleDB.Close()
	}
	return nil
}

func (s *StorageService) Schema() object.TypeURLStore {
	return s.protoStore
}

func (s *StorageService) ConfigureHTTP(mux *mux.Router, opts []connect.HandlerOption) {
	s.logger.Info("configuring routes")
	v1alpha1connect.RegisterKeyValueServiceHandler(mux, s, opts...)
}
