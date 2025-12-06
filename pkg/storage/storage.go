package storage

import "context"

type KV interface {
	Put(ctx context.Context, key string, obj []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	ListKeys(ctx context.Context) ([]string, error)
	List(ctx context.Context) ([][]byte, error)
	Delete(ctx context.Context, key string) error
}

type KVBroker interface {
	KeyValue(prefix string) KV
}

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
