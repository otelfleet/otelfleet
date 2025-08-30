package storage

import "context"

type KeyValue[T any] interface {
	Put(ctx context.Context, key string, obj T) error
	Get(ctx context.Context, key string) (T, error)
	ListKeys(ctx context.Context) ([]string, error)
	List(ctx context.Context) ([]T, error)
	Delete(ctx context.Context, key string) error
}

type KeyValueBroker[T any] interface {
	KeyValue(prefix string) KeyValue[T]
}

type KVStorageFactory[T any] func() KeyValue[T]
