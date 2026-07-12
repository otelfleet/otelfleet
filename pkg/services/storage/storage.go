package storage

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/pebble/v2"
	"github.com/grafana/dskit/services"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
)

type StorageService struct {
	logger *slog.Logger
	db     *pebble.DB
	broker types.KVBroker

	services.Service
	storagePath string
}

var _ services.Service = (*StorageService)(nil)
var _ types.KVBroker = (*StorageService)(nil)

func NewStorageService(
	logger *slog.Logger,
	storagePath string,
) (*StorageService, error) {
	kvDb, err := otelpebble.Open(
		storagePath,
		nil,
	)
	if err != nil {
		logger.Error("failed to start KV store")
		return nil, err
	}
	broker := otelpebble.NewKVBroker(kvDb)
	s := &StorageService{
		logger:      logger,
		storagePath: storagePath,
		db:          kvDb,
		broker:      broker,
		Service:     nil,
	}

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
	// TODO ? handle failure case
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *StorageService) KeyValue(prefix string) types.KV {
	return s.broker.KeyValue(prefix)
}
