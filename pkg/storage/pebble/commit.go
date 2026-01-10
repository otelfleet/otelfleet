package pebble

import (
	"slices"

	"github.com/cockroachdb/pebble/v2"
)

type (
	reader interface {
		pebble.Reader
	}
	writer interface {
		pebble.Writer
	}
	readerWriter interface {
		reader
		writer
	}
)

type readOnlyTransaction struct {
	reader
}

func (kv *prefixedKV) WithReadOnlyTransaction(fn func(tx readOnlyTransaction) error) error {
	return fn(readOnlyTransaction{kv.db})
}

type readWriteTransaction struct {
	*pebble.Batch

	onCommitCallbacks []func()
}

func (tx *readWriteTransaction) onCommit(callback func()) {
	tx.onCommitCallbacks = append(tx.onCommitCallbacks, callback)
}

func (kv *prefixedKV) withReadWriteTransaction(fn func(tx *readWriteTransaction) error) error {
	batch := kv.db.NewIndexedBatch()
	tx := &readWriteTransaction{Batch: batch}
	err := batch.Commit(nil)
	if err != nil {
		for _, f := range slices.Backward(tx.onCommitCallbacks) {
			f()
		}
	}
	return err
}
