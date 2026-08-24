package pebble

import (
	"context"
	"encoding/binary"
	"strings"
	"sync/atomic"

	"github.com/cockroachdb/pebble/v2"
	"github.com/otelfleet/otelfleet/pkg/storage/kv"
)

// changeNamespace is outside any prefixed keyspace: user keys are always
// written under "<prefix>/", so a leading 0x00 can never be listed or fetched
// through the BaseKV surface.
const changeNamespace = "\x00change/"

// changeTerminator sorts below '/', so prefix "a" cannot range over the
// records of prefix "a/b".
const changeTerminator = 0x00

// changeVersionKey holds the last version handed out, so a reopened DB resumes
// above it without scanning every prefix.
const changeVersionKey = "\x00changever"

const (
	opModify byte = 0
	opDelete byte = 1
)

// changeVersions hands out versions shared by every prefix of one DB. Versions
// only have to increase, so per-prefix gaps are fine and no per-prefix state is
// needed.
type changeVersions struct {
	atomic.Uint64
}

func loadChangeVersions(db *pebble.DB) *changeVersions {
	v := &changeVersions{}
	data, closer, err := db.Get([]byte(changeVersionKey))
	if err != nil {
		return v
	}
	defer closer.Close()
	if len(data) == 8 {
		v.Store(binary.BigEndian.Uint64(data))
	}
	return v
}

func encodeVersion(version uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, version)
	return b
}

// changelog appends packed change records under changeNamespace/<prefix>/<version>.
// Records are [op byte][key bytes]; the version is the 8 byte big endian key
// suffix, so iteration order is version order.
type changelog struct {
	db       *pebble.DB
	prefix   []byte
	versions *changeVersions
}

func newChangelog(db *pebble.DB, prefix string, versions *changeVersions) *changelog {
	return &changelog{
		db:       db,
		prefix:   append([]byte(changeNamespace+prefix), changeTerminator),
		versions: versions,
	}
}

func (c *changelog) bounds() ([]byte, []byte) {
	upper := make([]byte, len(c.prefix))
	copy(upper, c.prefix)
	upper[len(upper)-1]++
	return c.prefix, upper
}

func (c *changelog) changeKey(version uint64) []byte {
	key := make([]byte, len(c.prefix)+8)
	copy(key, c.prefix)
	binary.BigEndian.PutUint64(key[len(c.prefix):], version)
	return key
}

func packChange(op byte, key string) []byte {
	rec := make([]byte, len(key)+1)
	rec[0] = op
	copy(rec[1:], key)
	return rec
}

func unpackChange(rec []byte) (kv.WatchEvent, bool) {
	if len(rec) == 0 {
		return kv.WatchEvent{}, false
	}
	return kv.WatchEvent{Key: string(rec[1:]), Deleted: rec[0] == opDelete}, true
}

func (c *changelog) putModifyChange(b *pebble.Batch, key string) error {
	return c.appendChange(b, opModify, key)
}

func (c *changelog) putDeleteChange(b *pebble.Batch, key string) error {
	return c.appendChange(b, opDelete, key)
}

func (c *changelog) appendChange(b *pebble.Batch, op byte, key string) error {
	version := c.versions.Add(1)
	if err := b.Set(c.changeKey(version), packChange(op, key), nil); err != nil {
		return err
	}
	return b.Set([]byte(changeVersionKey), encodeVersion(version), nil)
}

func (c *changelog) currentVersion() uint64 {
	return c.versions.Load()
}

type change struct {
	version uint64
	event   kv.WatchEvent
}

// since reads changes with a version strictly greater than from.
func (c *changelog) since(ctx context.Context, from uint64) ([]change, error) {
	_, upper := c.bounds()
	iter, err := c.db.NewIterWithContext(ctx, &pebble.IterOptions{
		LowerBound: c.changeKey(from + 1),
		UpperBound: upper,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	changes := []change{}
	for iter.First(); iter.Valid(); iter.Next() {
		event, ok := unpackChange(iter.Value())
		if !ok {
			continue
		}
		changes = append(changes, change{
			version: binary.BigEndian.Uint64(iter.Key()[len(c.prefix):]),
			event:   event,
		})
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return changes, nil
}

func matchesWatchPrefix(watchPrefix, key string) bool {
	return watchPrefix == "" || key == watchPrefix || strings.HasPrefix(key, watchPrefix+"/")
}
