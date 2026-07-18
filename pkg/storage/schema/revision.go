package schema

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/cespare/xxhash/v2"
	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/anypb"
)

type RevisionEngine struct {
	kv types.KV
}

func NewRevisionEngine(kv types.KV) *RevisionEngine {
	return &RevisionEngine{kv: kv}
}

func revEntryKey(base string, revision uint64) string {
	return fmt.Sprintf("%s/%016x", base, revision)
}

func contentHash(data []byte) []byte {
	h := make([]byte, 8)
	binary.BigEndian.PutUint64(h, xxhash.Sum64(data))
	return h
}

func (e *RevisionEngine) current(ctx context.Context, base string) (*keyvaluev1.KeyValueObject, error) {
	data, err := e.kv.GetLatest(ctx, base)
	if err != nil {
		if grpcutil.IsErrorNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return decodeKeyValueObject(data)
}

func (e *RevisionEngine) Put(ctx context.Context, base, typeURL string, revision uint64, obj *anypb.Any) (*keyvaluev1.KeyValueObject, error) {
	cur, err := e.current(ctx, base)
	if err != nil {
		return nil, err
	}
	curRev := cur.GetRevision()

	target := revision
	if revision == 0 {
		target = curRev + 1
	} else {
		switch {
		case revision == curRev:
			return nil, grpcutil.Error(codes.Aborted, fmt.Errorf("revision %d conflicts with current revision", revision))
		case revision < curRev || revision > curRev+1:
			return nil, grpcutil.ErrorInvalid(fmt.Errorf("revision %d is not the next revision after %d", revision, curRev))
		}
	}

	// TODO : I don't think this should be included in the engine.
	hash := contentHash(encodeProto(obj))
	if cur != nil && cur.GetObj() != nil && bytes.Equal(cur.GetHash(), hash) {
		return cur, nil
	}

	newObj := &keyvaluev1.KeyValueObject{
		Revision: target,
		Hash:     hash,
		TypeUrl:  typeURL,
		Obj:      obj,
	}
	if err := e.kv.Put(ctx, revEntryKey(base, target), encodeProto(newObj)); err != nil {
		return nil, err
	}
	return newObj, nil
}

func (e *RevisionEngine) Get(ctx context.Context, base string) (*keyvaluev1.KeyValueObject, error) {
	cur, err := e.current(ctx, base)
	if err != nil {
		return nil, err
	}
	if cur == nil || cur.GetObj() == nil {
		return nil, grpcutil.ErrorNotFound(fmt.Errorf("key not found"))
	}
	return cur, nil
}

func (e *RevisionEngine) GetRevision(ctx context.Context, base string, revision uint64) (*keyvaluev1.KeyValueObject, error) {
	data, err := e.kv.Get(ctx, revEntryKey(base, revision))
	if err != nil {
		return nil, err
	}
	obj, err := decodeKeyValueObject(data)
	if err != nil {
		return nil, err
	}
	if obj.GetObj() == nil {
		return nil, grpcutil.ErrorNotFound(fmt.Errorf("key not found"))
	}
	return obj, nil
}

func (e *RevisionEngine) Delete(ctx context.Context, base string) error {
	cur, err := e.current(ctx, base)
	if err != nil {
		return err
	}
	if cur == nil || cur.GetObj() == nil {
		return nil
	}
	tombstone := &keyvaluev1.KeyValueObject{Revision: cur.GetRevision() + 1}
	return e.kv.Put(ctx, revEntryKey(base, tombstone.GetRevision()), encodeProto(tombstone))
}

func (e *RevisionEngine) ListLatest(ctx context.Context, listPrefix string) (map[string]*keyvaluev1.KeyValueObject, error) {
	entries, err := e.kv.ListEntries(ctx, listPrefix)
	if err != nil {
		return nil, err
	}
	latest := map[string]*keyvaluev1.KeyValueObject{}
	for _, entry := range entries {
		idx := strings.LastIndex(entry.Key, "/")
		if idx < 0 {
			continue
		}
		key := entry.Key[:idx]
		obj, err := decodeKeyValueObject(entry.Value)
		if err != nil {
			return nil, err
		}
		latest[key] = obj
	}
	for key, obj := range latest {
		if obj.GetObj() == nil {
			delete(latest, key)
		}
	}
	return latest, nil
}
