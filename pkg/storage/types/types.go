package types

import (
	"context"
)

type KeyValue[T any] interface {
	Put(ctx context.Context, key string, obj T) error
	PutRevision(ctx context.Context, key string, revision uint64, obj T) (uint64, error)
	Get(ctx context.Context, key string) (T, error)
	GetRevision(ctx context.Context, key string, revision uint64) (T, error)
	ListKeys(ctx context.Context) ([]string, error)
	List(ctx context.Context) ([]T, error)
	Delete(ctx context.Context, key string) error
	History(ctx context.Context, key string, offset uint64, limit uint64) ([]T, error)
	Watch(ctx context.Context, prefix string) (<-chan RevisionObject[T], error)
}

type RevisionObject[T any] struct {
	Key      string
	Revision uint64
	Object   T
	Deleted  bool
}

type KeyValueBroker[T any] interface {
	KeyValue(prefix string) KeyValue[T]
}

type KVStorageFactory[T any] func() KeyValue[T]
