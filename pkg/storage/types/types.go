package types

import "context"

type KVEntry struct {
	Key   string
	Value []byte
}

type BaseKV interface {
	Put(ctx context.Context, key string, obj []byte) error
	// Get is an exact-key point lookup.
	Get(ctx context.Context, key string) ([]byte, error)
	ListKeys(ctx context.Context, prefix string) ([]string, error)
	List(ctx context.Context, prefix string) ([][]byte, error)
	Delete(ctx context.Context, key string) error

	// GetLatest and ListEntries are internal primitives for the revision layer,
	// not for direct consumer use; go through KeyValue[T] or SchemaProto instead.

	// GetLatest returns the value of the lexicographically-last key under prefix,
	// so the revision layer can read the newest revision without scanning history.
	GetLatest(ctx context.Context, prefix string) ([]byte, error)
	// ListEntries returns key/value pairs under prefix, used to collapse
	// per-revision entries down to the latest per key.
	ListEntries(ctx context.Context, prefix string) ([]KVEntry, error)
}

type KVBroker interface {
	KeyValue(prefix string) BaseKV
}

type KeyValue[T any] interface {
	Put(ctx context.Context, key string, obj T) error
	PutRevision(ctx context.Context, key string, revision uint64, obj T) (uint64, error)
	Get(ctx context.Context, key string) (T, error)
	GetRevision(ctx context.Context, key string, revision uint64) (T, error)
	ListKeys(ctx context.Context) ([]string, error)
	List(ctx context.Context) ([]T, error)
	Delete(ctx context.Context, key string) error
}

type KeyValueBroker[T any] interface {
	KeyValue(prefix string) KeyValue[T]
}

type KVStorageFactory[T any] func() KeyValue[T]
