package kv_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/otelfleet/otelfleet/pkg/storage/kv"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/kv/driver/pebble"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseKV(t *testing.T) {
	drivers := map[string]func(t *testing.T) kv.BaseKV{
		"pebble": func(t *testing.T) kv.BaseKV {
			db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			return otelpebble.NewKVBroker(db).KeyValue("test")
		},
	}

	for name, storeF := range drivers {
		t.Run(name, func(t *testing.T) {
			testBaseKVConformance(t, storeF)
		})
	}
}

func testBaseKVConformance(t *testing.T, storeF func(t *testing.T) kv.BaseKV) {
	t.Helper()
	t.Run("Put", func(t *testing.T) { testPut(t, storeF(t)) })
	t.Run("Get", func(t *testing.T) { testGet(t, storeF(t)) })
	t.Run("GetLatest", func(t *testing.T) { testGetLatest(t, storeF(t)) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, storeF(t)) })
	t.Run("List", func(t *testing.T) { testList(t, storeF(t)) })
	t.Run("Watch", func(t *testing.T) { testWatch(t, storeF) })
}

func put(t *testing.T, store kv.BaseKV, key, value string) {
	t.Helper()
	require.NoError(t, store.Put(t.Context(), key, []byte(value)))
}

func putAll(t *testing.T, store kv.BaseKV, entries map[string]string) {
	t.Helper()
	for k, v := range entries {
		put(t, store, k, v)
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, grpcutil.IsErrorNotFound(err), "expected NotFound, got %v", err)
}

func testPut(t *testing.T, store kv.BaseKV) {
	for name, tc := range map[string]struct {
		key   string
		value []byte
	}{
		"simple":       {key: "a", value: []byte("1")},
		"nested key":   {key: "a/b/c", value: []byte("2")},
		"empty value":  {key: "empty", value: []byte{}},
		"binary value": {key: "bin", value: []byte{0x00, 0xff, 0x10}},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			require.NoError(t, store.Put(ctx, tc.key, tc.value))
			got, err := store.Get(ctx, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.value, got)
		})
	}

	t.Run("overwrites", func(t *testing.T) {
		ctx := t.Context()
		put(t, store, "k", "v1")
		put(t, store, "k", "v2")
		got, err := store.Get(ctx, "k")
		require.NoError(t, err)
		assert.Equal(t, "v2", string(got))
	})

	t.Run("many put", func(t *testing.T) {
		for i := range 1000 {
			put(t, store, fmt.Sprintf("k-%d", i), fmt.Sprintf("v-%d", i))
		}
	})
}

func testGet(t *testing.T, store kv.BaseKV) {
	putAll(t, store, map[string]string{"a": "1", "p/b": "2"})

	for name, tc := range map[string]struct {
		key  string
		want string
	}{
		"top level": {key: "a", want: "1"},
		"nested":    {key: "p/b", want: "2"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := store.Get(t.Context(), tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}

	for name, key := range map[string]string{
		"missing key":        "nope",
		"prefix of a key":    "p",
		"exact prefix match": "p/",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := store.Get(t.Context(), key)
			assertNotFound(t, err)
		})
	}
}

func testGetLatest(t *testing.T, store kv.BaseKV) {
	putAll(t, store, map[string]string{
		"p/1": "first",
		"p/3": "last",
		"p/2": "middle",
		"q/9": "other",
	})

	for name, tc := range map[string]struct {
		prefix string
		want   string
	}{
		"lexicographically last under prefix": {prefix: "p", want: "last"},
		"single entry":                        {prefix: "q", want: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := store.GetLatest(t.Context(), tc.prefix)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}

	t.Run("no entries under prefix", func(t *testing.T) {
		_, err := store.GetLatest(t.Context(), "zz")
		assertNotFound(t, err)
	})
}

func testDelete(t *testing.T, store kv.BaseKV) {
	ctx := t.Context()
	putAll(t, store, map[string]string{"k": "v", "k/child": "c"})

	require.NoError(t, store.Delete(ctx, "k"))

	t.Run("key is gone", func(t *testing.T) {
		_, err := store.Get(ctx, "k")
		assertNotFound(t, err)
	})

	t.Run("leaves other keys", func(t *testing.T) {
		got, err := store.Get(ctx, "k/child")
		require.NoError(t, err)
		assert.Equal(t, "c", string(got))
	})

	t.Run("is idempotent", func(t *testing.T) {
		assert.NoError(t, store.Delete(ctx, "k"))
	})
}

func testList(t *testing.T, store kv.BaseKV) {
	putAll(t, store, map[string]string{
		"p/b":   "2",
		"p/a":   "1",
		"p/c/d": "3",
		"q/a":   "4",
	})

	for name, tc := range map[string]struct {
		prefix string
		want   []kv.KVEntry
	}{
		"whole store": {prefix: "", want: []kv.KVEntry{
			{Key: "p/a", Value: []byte("1")},
			{Key: "p/b", Value: []byte("2")},
			{Key: "p/c/d", Value: []byte("3")},
			{Key: "q/a", Value: []byte("4")},
		}},
		"subtree": {prefix: "p", want: []kv.KVEntry{
			{Key: "a", Value: []byte("1")},
			{Key: "b", Value: []byte("2")},
			{Key: "c/d", Value: []byte("3")},
		}},
		"nested subtree": {prefix: "p/c", want: []kv.KVEntry{
			{Key: "d", Value: []byte("3")},
		}},
		"prefix with no entries": {prefix: "zz", want: []kv.KVEntry{}},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			entries, err := store.ListEntries(ctx, tc.prefix)
			require.NoError(t, err)
			assert.Equal(t, tc.want, entries, "entries must be sorted and prefix-relative")

			keys, err := store.ListKeys(ctx, tc.prefix)
			require.NoError(t, err)
			assert.Equal(t, entryKeys(tc.want), keys)

			values, err := store.List(ctx, tc.prefix)
			require.NoError(t, err)
			assert.Equal(t, entryValues(tc.want), values)
		})
	}
}

func entryKeys(entries []kv.KVEntry) []string {
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return keys
}

func entryValues(entries []kv.KVEntry) [][]byte {
	values := make([][]byte, len(entries))
	for i, e := range entries {
		values[i] = e.Value
	}
	return values
}

const watchTimeout = 5 * time.Second

func recvEvent(t *testing.T, events <-chan kv.WatchEvent) kv.WatchEvent {
	t.Helper()
	select {
	case evt, ok := <-events:
		require.True(t, ok, "watch channel closed early")
		return evt
	case <-time.After(watchTimeout):
		require.Fail(t, "timed out waiting for watch event")
		return kv.WatchEvent{}
	}
}

func testWatch(t *testing.T, storeF func(t *testing.T) kv.BaseKV) {
	t.Run("put", func(t *testing.T) {
		store := storeF(t)
		events, err := store.Watch(t.Context(), "")
		require.NoError(t, err)

		put(t, store, "k", "v")
		assert.Equal(t, kv.WatchEvent{Key: "k"}, recvEvent(t, events))
	})

	t.Run("delete", func(t *testing.T) {
		store := storeF(t)
		events, err := store.Watch(t.Context(), "")
		require.NoError(t, err)

		put(t, store, "d", "v")
		require.Equal(t, kv.WatchEvent{Key: "d"}, recvEvent(t, events))

		require.NoError(t, store.Delete(t.Context(), "d"))
		assert.Equal(t, kv.WatchEvent{Key: "d", Deleted: true}, recvEvent(t, events))
	})

	t.Run("closes on context cancellation", func(t *testing.T) {
		ctx, ca := context.WithCancel(t.Context())
		events, err := storeF(t).Watch(ctx, "")
		require.NoError(t, err)
		ca()

		select {
		case <-events:
		case <-time.After(watchTimeout):
			assert.Fail(t, "watch channel not closed after context cancellation")
		}
	})
}
