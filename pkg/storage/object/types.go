package object

import (
	"context"

	keyvalue_v1alpha1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"google.golang.org/protobuf/types/known/anypb"
)

type TypeURLStore interface {
	Put(ctx context.Context, typeURL, key string, revision uint64, obj *anypb.Any) (*keyvalue_v1alpha1.KeyValueObject, error)
	Get(ctx context.Context, typeURL, key string) (*keyvalue_v1alpha1.KeyValueObject, error)
	GetRevision(ctx context.Context, typeURL, key string, revision uint64) (*keyvalue_v1alpha1.KeyValueObject, error)
	ListKeys(ctx context.Context, typeURL string) ([]string, error)
	List(ctx context.Context, typeURL string) ([]*keyvalue_v1alpha1.KeyValueObject, error)
	Delete(ctx context.Context, typeURL, key string) error
	History(ctx context.Context, typeURL, key string, offset, limit uint64) (*keyvalue_v1alpha1.GetHistoryResponse, error)
	Watch(ctx context.Context, typeURL, prefix string) (<-chan *keyvalue_v1alpha1.WatchEvent, error)
}

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
