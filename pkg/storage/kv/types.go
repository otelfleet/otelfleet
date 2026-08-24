package kv

import (
	"context"
)

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

	Watch(ctx context.Context, prefix string) (<-chan WatchEvent, error)
}

type WatchEvent struct {
	Key     string
	Deleted bool
}

type KVBroker interface {
	KeyValue(prefix string) BaseKV
}
