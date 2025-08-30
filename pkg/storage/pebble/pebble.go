package pebble

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/pebble/v2"
	"github.com/otelfleet/otelfleet/pkg/storage"
)

type KeyValueBroker[T any] struct {
	db *pebble.DB
}

func NewPebbleBroker[T any](db *pebble.DB) *KeyValueBroker[T] {
	return &KeyValueBroker[T]{
		db: db,
	}
}

// func (k *KeyValueBroker[T]) Start(ctx context.Context) error {
// 	db, err := pebble.Open("./otelfleet.kv", &pebble.Options{})
// 	if err != nil {
// 		return err
// 	}
// 	k.db = db
// 	return nil
// }

// func (k *KeyValueBroker[T]) Shutdown(ctx context.Context) error {
// 	if k.db == nil {
// 		return nil
// 	}
// 	if err := k.db.Close(); err != nil {
// 		return err
// 	}
// 	return nil
// }

func (k *KeyValueBroker[T]) KeyValue(prefix string) storage.KeyValue[T] {
	return k.newPrefixedKeyValue(prefix)
}

func (k *KeyValueBroker[T]) newPrefixedKeyValue(prefix string) *prefixedKeyValue[T] {
	return &prefixedKeyValue[T]{
		db:     k.db,
		prefix: []byte(prefix),
	}
}

type prefixedKeyValue[T any] struct {
	prefix []byte
	db     *pebble.DB
}

func (k *prefixedKeyValue[T]) key(key string) []byte {
	fullKey := make([]byte, len(k.prefix)+len(key)+1)
	copy(fullKey, k.prefix)
	fullKey[len(k.prefix)] = '/'
	copy(fullKey[len(k.prefix)+1:], key)
	return fullKey
}

func (k *prefixedKeyValue[T]) Put(_ context.Context, key string, value T) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = k.db.Set(k.key(key), v, &pebble.WriteOptions{})
	return err
}

func (k *prefixedKeyValue[T]) Get(_ context.Context, key string) (T, error) {
	var t T
	data, closer, err := k.db.Get(k.key(key))
	defer closer.Close()
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, err
	}
	return t, nil
}

func (k *prefixedKeyValue[T]) listPrefix() []byte {
	prefix := make([]byte, len(k.prefix)+1)
	copy(prefix, k.prefix)
	prefix[len(k.prefix)] = '/'
	return prefix
}

func (k *prefixedKeyValue[T]) ListKeys(ctx context.Context) ([]string, error) {
	prefix := k.listPrefix()
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	upper[len(prefix)-1]++
	iter, err := k.db.NewIterWithContext(ctx, &pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upper,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	keys := []string{}
	for iter.First(); iter.Valid(); iter.Next() {
		keys = append(keys, string(iter.Key()))
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (k *prefixedKeyValue[T]) List(ctx context.Context) ([]T, error) {
	prefix := k.listPrefix()
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	upper[len(prefix)-1]++
	iter, err := k.db.NewIterWithContext(ctx, &pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upper,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	vs := []T{}
	for iter.First(); iter.Valid(); iter.Next() {
		var t T
		if err := json.Unmarshal(iter.Value(), &t); err != nil {
			return nil, err
		}
		vs = append(vs, t)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return vs, nil
}

func (k *prefixedKeyValue[T]) Delete(ctx context.Context, key string) error {
	return k.db.Delete(k.key(key), &pebble.WriteOptions{})
}

var _ storage.KeyValue[any] = (*prefixedKeyValue[any])(nil)
var _ storage.KeyValueBroker[any] = (*KeyValueBroker[any])(nil)
