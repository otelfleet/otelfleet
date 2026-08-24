package revision

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/kv"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultLimit = 50
)

type RevisionEngine struct {
	kv kv.BaseKV
}

func NewRevisionEngine(kv kv.BaseKV) *RevisionEngine {
	return &RevisionEngine{kv: kv}
}

func (e *RevisionEngine) pathRevision(base string, revision uint64) string {
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
	hash, err := util.ProtoHash(obj)
	if err != nil {
		return nil, err
	}
	if cur != nil && cur.GetObj() != nil && bytes.Equal(cur.GetHash(), hash) {
		return cur, nil
	}

	newObj := &keyvaluev1.KeyValueObject{
		Revision:   target,
		Hash:       hash,
		TypeUrl:    typeURL,
		Obj:        obj,
		ModifiedAt: timestamppb.Now(),
	}
	if err := e.kv.Put(ctx, e.pathRevision(base, target), encodeProto(newObj)); err != nil {
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
	data, err := e.kv.Get(ctx, e.pathRevision(base, revision))
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
	return e.kv.Put(ctx, e.pathRevision(base, tombstone.GetRevision()), encodeProto(tombstone))
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

func (e *RevisionEngine) History(ctx context.Context, base string, offset, limit uint64) (*keyvaluev1.GetHistoryResponse, error) {
	if limit <= 0 {
		limit = defaultLimit
	}

	entries, err := e.kv.ListEntries(ctx, base)
	if err != nil {
		return nil, err
	}
	revisions := []uint64{}
	for _, entry := range entries {
		key := entry.Key
		if idx := strings.LastIndex(key, "/"); idx >= 0 {
			key = key[idx+1:]
		}
		rev, err := strconv.ParseUint(key, 16, 64)
		if err != nil {
			panic(err)
		}
		revisions = append(revisions, rev)
	}
	slices.Sort(revisions)

	n := uint64(len(revisions))
	empty := &keyvaluev1.GetHistoryResponse{
		Position: &keyvaluev1.RangeRequest{
			Offset: offset,
			Limit:  limit,
		},
		Objs: []*keyvaluev1.KeyValueObject{},
	}
	if offset >= n {
		return empty, nil
	}
	offsetN := n - offset
	limitN := uint64(0)
	if limit != 0 && limit < offsetN {
		limitN = offsetN - limit
	}
	actualRevisions := revisions[limitN:offsetN]
	objs := make([]*keyvaluev1.KeyValueObject, len(actualRevisions))
	last := len(actualRevisions) - 1
	for i, rev := range slices.Backward(actualRevisions) {
		kvobj, err := e.GetRevision(ctx, base, rev)
		if err != nil {
			return nil, err
		}
		objs[last-i] = kvobj
	}
	return &keyvaluev1.GetHistoryResponse{
		Objs: objs,
		Position: &keyvaluev1.RangeRequest{
			Offset: offset,
			Limit:  limit,
		},
	}, nil
}

const bufN = 16

func (e *RevisionEngine) Watch(ctx context.Context, base string) (<-chan *keyvaluev1.WatchEvent, error) {
	resp, err := e.kv.GetLatest(ctx, base)
	if err != nil {
		return nil, err
	}

	sendC := make(chan *keyvaluev1.WatchEvent, bufN)
	// TODO : not found
	first, err := decodeKeyValueObject(resp)
	if grpcutil.IsError(codes.NotFound, err) {
		first = nil
	} else if err != nil {
		return nil, err
	}

	recvObj, err := e.kv.Watch(ctx, base)
	if err != nil {
		return nil, err
	}
	go func() {
		if first != nil {
			sendC <- &keyvaluev1.WatchEvent{
				EventType: &keyvaluev1.WatchEvent_Modified{
					Modified: first,
				},
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case notifyEvt, ok := <-recvObj:
				if !ok {
					return
				}
				if notifyEvt.Deleted {
					sendC <- &keyvaluev1.WatchEvent{
						EventType: &keyvaluev1.WatchEvent_DeletedKey{
							DeletedKey: notifyEvt.Key,
						},
					}
				}
				data, err := e.kv.Get(ctx, notifyEvt.Key)
				if err != nil {
					return
				}
				obj, err := decodeKeyValueObject(data)
				if err != nil {
					continue
				}
				sendC <- &keyvaluev1.WatchEvent{
					EventType: &keyvaluev1.WatchEvent_Modified{
						Modified: obj,
					},
				}
			}
		}
	}()

	return sendC, nil
}

var (
	marshalOptions = proto.MarshalOptions{
		AllowPartial:  true,
		Deterministic: true,
	}
	unmarshalOptions = proto.UnmarshalOptions{
		AllowPartial:   true,
		DiscardUnknown: true,
	}
)

func encodeProto(msg proto.Message) []byte {
	data, err := marshalOptions.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return data
}

func decodeKeyValueObject(data []byte) (*keyvaluev1.KeyValueObject, error) {
	obj := &keyvaluev1.KeyValueObject{}
	if err := unmarshalOptions.Unmarshal(data, obj); err != nil {
		return nil, err
	}
	return obj, nil
}
