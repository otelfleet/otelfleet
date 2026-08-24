package pebble

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/otelfleet/otelcol-lsp/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/storage/kv"
	"github.com/otelfleet/otelfleet/pkg/util/contextutil"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
)

const notifyBufferSize = 64

type pebbleLogger struct {
	logger *slog.Logger
}

func (p pebbleLogger) Infof(msg string, args ...any) {
	p.logger.Debug(fmt.Sprintf(msg, args...))
}

func (p pebbleLogger) Errorf(msg string, args ...any) {
	p.logger.Error(msg, args...)
}

func (p pebbleLogger) Fatalf(msg string, args ...any) {
	log.Fatalf(msg, args...)
}

func (p pebbleLogger) Eventf(ctx context.Context, msg string, args ...any) {
	p.logger.Debug(fmt.Sprintf(msg, args...))
}

func (pebbleLogger) IsTracingEnabled(_ context.Context) bool { return false }

// enforce strict permissions on files (0600) and directories (0700)
type secureFS struct{ vfs.FS }

// NewSecureFS creates a new secure FS.
func NewSecureFS(underlying vfs.FS) vfs.FS {
	return secureFS{underlying}
}

// Open opens a pebble database. It sets options useful for pomerium.
func Open(dirname string, options *pebble.Options) (*pebble.DB, error) {
	if options == nil {
		options = new(pebble.Options)
	}
	options.LoggerAndTracer = pebbleLogger{
		logger: slog.Default().With("storage-engine", "pebble"),
	}
	eventListener := pebble.MakeLoggingEventListener(options.LoggerAndTracer)
	options.EventListener = &eventListener
	if options.FS == nil {
		options.FS = NewSecureFS(vfs.Default)
	}
	options.ApplyCompressionSettings(func() pebble.DBCompressionSettings {
		return pebble.DBCompressionBalanced
	})
	return pebble.Open(dirname, options)
}

type KVBroker struct {
	db       *pebble.DB
	versions *changeVersions
}

func NewKVBroker(db *pebble.DB) *KVBroker {
	return &KVBroker{
		db:       db,
		versions: loadChangeVersions(db),
	}
}

func (k *KVBroker) KeyValue(prefix string) kv.BaseKV {
	return k.newPrefixedKeyValue(prefix)
}

func (k *KVBroker) newPrefixedKeyValue(prefix string) *prefixedKV {
	return &prefixedKV{
		db:      k.db,
		prefix:  []byte(prefix),
		notifyC: make(chan kv.WatchEvent, notifyBufferSize),
		signal:  contextutil.NewSignal(fmt.Sprintf("storage-%s", prefix)),
		changes: newChangelog(k.db, prefix, k.versions),
	}
}

type prefixedKV struct {
	prefix  []byte
	db      *pebble.DB
	notifyC chan kv.WatchEvent
	signal  *contextutil.Signal
	changes *changelog
}

func (k *prefixedKV) key(key string) []byte {
	fullKey := make([]byte, len(k.prefix)+len(key)+1)
	copy(fullKey, k.prefix)
	fullKey[len(k.prefix)] = '/'
	copy(fullKey[len(k.prefix)+1:], key)
	return fullKey
}

func (k *prefixedKV) Put(ctx context.Context, key string, value []byte) error {
	b := k.db.NewBatch()
	defer b.Close()
	if err := b.Set(k.key(key), value, nil); err != nil {
		return err
	}
	if err := k.changes.putModifyChange(b, key); err != nil {
		return err
	}
	if err := b.Commit(&pebble.WriteOptions{}); err != nil {
		return err
	}
	k.signal.Broadcast(ctx)
	return nil
}

func (k *prefixedKV) Get(_ context.Context, key string) ([]byte, error) {
	data, closer, err := k.db.Get(k.key(key))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, grpcutil.ErrorNotFound(err)
		}
		return nil, err
	}
	defer closer.Close()
	return data, nil
}

func (k *prefixedKV) GetLatest(ctx context.Context, prefix string) ([]byte, error) {
	lp := k.listPrefix(prefix)
	upper := make([]byte, len(lp))
	copy(upper, lp)
	upper[len(lp)-1]++
	iter, err := k.db.NewIterWithContext(ctx, &pebble.IterOptions{
		LowerBound: lp,
		UpperBound: upper,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	if !iter.Last() {
		if err := iter.Error(); err != nil {
			return nil, err
		}
		return nil, grpcutil.ErrorNotFound(pebble.ErrNotFound)
	}
	value := make([]byte, len(iter.Value()))
	copy(value, iter.Value())
	return value, nil
}

func (k *prefixedKV) listPrefix(listPrefix string) []byte {
	size := len(k.prefix) + 1
	if listPrefix != "" {
		size += len(listPrefix) + 1
	}
	prefix := make([]byte, size)
	n := copy(prefix, k.prefix)
	prefix[n] = '/'
	n++
	if listPrefix != "" {
		n += copy(prefix[n:], listPrefix)
		prefix[n] = '/'
	}
	return prefix
}

func (k *prefixedKV) ListKeys(ctx context.Context, listPrefix string) ([]string, error) {
	prefix := k.listPrefix(listPrefix)
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

func (k *prefixedKV) List(ctx context.Context, listPrefix string) ([][]byte, error) {
	entries, err := k.ListEntries(ctx, listPrefix)
	if err != nil {
		return nil, err
	}
	vs := make([][]byte, len(entries))
	for i, e := range entries {
		vs[i] = e.Value
	}
	return vs, nil
}

func (k *prefixedKV) ListEntries(ctx context.Context, listPrefix string) ([]kv.KVEntry, error) {
	prefix := k.listPrefix(listPrefix)
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
	entries := []kv.KVEntry{}
	for iter.First(); iter.Valid(); iter.Next() {
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		entries = append(entries, kv.KVEntry{
			Key:   string(iter.Key()[pn:]),
			Value: value,
		})
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (k *prefixedKV) Delete(ctx context.Context, key string) error {
	b := k.db.NewBatch()
	defer b.Close()
	if err := b.Delete(k.key(key), nil); err != nil {
		return err
	}
	if err := k.changes.putDeleteChange(b, key); err != nil {
		return err
	}
	if err := b.Commit(&pebble.WriteOptions{}); err != nil {
		return err
	}
	k.signal.Broadcast(ctx)
	return nil
}

const bufN = 16

func (k *prefixedKV) Watch(ctx context.Context, prefix string) (<-chan kv.WatchEvent, error) {
	ret := make(chan kv.WatchEvent, bufN)
	wakeup := k.signal.Bind()
	cursor := k.changes.currentVersion()
	go func() {
		defer close(ret)
		defer k.signal.Unbind(wakeup)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-wakeup:
				if !ok {
					return
				}
				// TODO : this implementation might be pretty slow
				next, err := k.drainChanges(ctx, ret, prefix, cursor)
				if err != nil {
					logutil.From(ctx).With("prefix", prefix, "err", err).Error("failed to read changelog")
					return
				}
				cursor = next
			}
		}
	}()
	return ret, nil
}

func (k *prefixedKV) drainChanges(ctx context.Context, out chan<- kv.WatchEvent, prefix string, cursor uint64) (uint64, error) {
	changes, err := k.changes.since(ctx, cursor)
	if err != nil {
		return cursor, err
	}
	for _, c := range changes {
		cursor = c.version
		if !matchesWatchPrefix(prefix, c.event.Key) {
			continue
		}
		select {
		case out <- c.event:
		case <-ctx.Done():
			return cursor, ctx.Err()
		}
	}
	return cursor, nil
}

var _ kv.BaseKV = (*prefixedKV)(nil)
var _ kv.KVBroker = (*KVBroker)(nil)
