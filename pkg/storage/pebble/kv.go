package pebble

import (
	"context"

	"github.com/cockroachdb/pebble/v2"
	"github.com/otelfleet/otelfleet/pkg/storage"
)

type KVBroker struct {
	db *pebble.DB
}

func NewKVBroker(db *pebble.DB) *KVBroker {
	return &KVBroker{
		db: db,
	}
}

func (k *KVBroker) KeyValue(prefix string) storage.KV {
	return k.newPrefixedKeyValue(prefix)
}

func (k *KVBroker) newPrefixedKeyValue(prefix string) *prefixedKV {
	return &prefixedKV{
		db:     k.db,
		prefix: []byte(prefix),
	}
}

type prefixedKV struct {
	prefix []byte
	db     *pebble.DB
}

func (k *prefixedKV) key(key string) []byte {
	fullKey := make([]byte, len(k.prefix)+len(key)+1)
	copy(fullKey, k.prefix)
	fullKey[len(k.prefix)] = '/'
	copy(fullKey[len(k.prefix)+1:], key)
	return fullKey
}

func (k *prefixedKV) Put(_ context.Context, key string, value []byte) error {
	return k.db.Set(k.key(key), value, &pebble.WriteOptions{})
}

func (k *prefixedKV) Get(_ context.Context, key string) ([]byte, error) {
	data, closer, err := k.db.Get(k.key(key))
	defer closer.Close()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (k *prefixedKV) listPrefix() []byte {
	prefix := make([]byte, len(k.prefix)+1)
	copy(prefix, k.prefix)
	prefix[len(k.prefix)] = '/'
	return prefix
}

func (k *prefixedKV) ListKeys(ctx context.Context) ([]string, error) {
	prefix := k.listPrefix()
	pn := len(prefix)
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
		iKey := iter.Key()[pn:]
		keys = append(keys, string(iKey))
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (k *prefixedKV) List(ctx context.Context) ([][]byte, error) {
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
	vs := [][]byte{}
	for iter.First(); iter.Valid(); iter.Next() {
		vs = append(vs, iter.Value())
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return vs, nil
}

func (k *prefixedKV) Delete(ctx context.Context, key string) error {
	return k.db.Delete(k.key(key), &pebble.WriteOptions{})
}

var _ storage.KV = (*prefixedKV)(nil)
var _ storage.KVBroker = (*KVBroker)(nil)
