package pebble_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/kv/driver/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangelogIsNotVisible(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := otelpebble.NewKVBroker(db).KeyValue("test")
	ctx := t.Context()
	require.NoError(t, store.Put(ctx, "a/b", []byte("1")))
	require.NoError(t, store.Delete(ctx, "a/b"))

	keys, err := store.ListKeys(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, keys)

	entries, err := store.ListEntries(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, entries)

	_, err = store.Get(ctx, "\x00change/test/")
	assert.Error(t, err)
}

func TestChangelogVersionSurvivesReopen(t *testing.T) {
	fs := vfs.NewMem()
	db, err := pebble.Open("db", &pebble.Options{FS: fs})
	require.NoError(t, err)
	store := otelpebble.NewKVBroker(db).KeyValue("test")
	require.NoError(t, store.Put(t.Context(), "a", []byte("1")))
	require.NoError(t, db.Close())

	db, err = pebble.Open("db", &pebble.Options{FS: fs})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store = otelpebble.NewKVBroker(db).KeyValue("test")
	events, err := store.Watch(t.Context(), "")
	require.NoError(t, err)
	require.NoError(t, store.Put(t.Context(), "b", []byte("2")))

	evt := <-events
	assert.Equal(t, "b", evt.Key)

	assert.Equal(t, 2, countChangeRecords(t, db), "reopen must not overwrite prior change records")
}

func countChangeRecords(t *testing.T, db *pebble.DB) int {
	t.Helper()
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("\x00change/"),
		UpperBound: []byte("\x00change0"),
	})
	require.NoError(t, err)
	defer iter.Close()
	n := 0
	for iter.First(); iter.Valid(); iter.Next() {
		n++
	}
	return n
}

func TestChangelogPrefixesDoNotNest(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	broker := otelpebble.NewKVBroker(db)
	outer, inner := broker.KeyValue("a"), broker.KeyValue("a/b")

	events, err := outer.Watch(t.Context(), "")
	require.NoError(t, err)

	require.NoError(t, inner.Put(t.Context(), "k", []byte("1")))
	require.NoError(t, outer.Put(t.Context(), "mine", []byte("2")))

	assert.Equal(t, "mine", (<-events).Key)
	select {
	case evt := <-events:
		assert.Fail(t, "leaked event from nested prefix", evt.Key)
	default:
	}
}

func TestChangelogSharedAcrossHandles(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	broker := otelpebble.NewKVBroker(db)
	require.NoError(t, broker.KeyValue("test").Put(t.Context(), "a", []byte("1")))
	require.NoError(t, broker.KeyValue("test").Put(t.Context(), "b", []byte("2")))

	assert.Equal(t, 2, countChangeRecords(t, db))
}
