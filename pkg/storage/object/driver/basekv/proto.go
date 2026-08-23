package basekv

import (
	"context"
	"path"
	"sort"

	keyvalue_v1alpha1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/kv"
	"github.com/otelfleet/otelfleet/pkg/storage/kv/revision"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

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

const (
	defaultVersion = "v1alpha1"
)

type StorageProtoObject struct {
	baseVersion string
	revisions   *revision.RevisionEngine
}

func NewTypeURLStore(
	kv kv.BaseKV,
) *StorageProtoObject {
	return &StorageProtoObject{
		baseVersion: defaultVersion,
		revisions:   revision.NewRevisionEngine(kv),
	}
}

var _ object.TypeURLStore = (*StorageProtoObject)(nil)

func (s *StorageProtoObject) protoPath(typeURL string) string {
	return path.Join(s.baseVersion, typeURL)
}

func (s *StorageProtoObject) keyPath(typeURL, key string) string {
	return path.Join(s.protoPath(typeURL), key)
}

func (s *StorageProtoObject) Put(ctx context.Context, typeURL string, key string, revision uint64, obj *anypb.Any) (*keyvalue_v1alpha1.KeyValueObject, error) {
	return s.revisions.Put(ctx, s.keyPath(typeURL, key), typeURL, revision, obj)
}

func (s *StorageProtoObject) Get(ctx context.Context, typeURL string, key string) (*keyvalue_v1alpha1.KeyValueObject, error) {
	return s.revisions.Get(ctx, s.keyPath(typeURL, key))
}

func (s *StorageProtoObject) GetRevision(ctx context.Context, typeURL, key string, revision uint64) (*keyvalue_v1alpha1.KeyValueObject, error) {
	return s.revisions.GetRevision(ctx, s.keyPath(typeURL, key), revision)
}

func (s *StorageProtoObject) ListKeys(ctx context.Context, typeURL string) ([]string, error) {
	latest, err := s.revisions.ListLatest(ctx, s.protoPath(typeURL))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *StorageProtoObject) List(ctx context.Context, typeURL string) ([]*keyvalue_v1alpha1.KeyValueObject, error) {
	latest, err := s.revisions.ListLatest(ctx, s.protoPath(typeURL))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	objs := make([]*keyvalue_v1alpha1.KeyValueObject, 0, len(latest))
	for _, key := range keys {
		objs = append(objs, latest[key])
	}
	return objs, nil
}

func (s *StorageProtoObject) Delete(ctx context.Context, typeURL, key string) error {
	return s.revisions.Delete(ctx, s.keyPath(typeURL, key))
}

func (s *StorageProtoObject) History(ctx context.Context, typeURL, key string, offset, limit uint64) (*keyvalue_v1alpha1.GetHistoryResponse, error) {
	base := s.keyPath(typeURL, key)
	resp, err := s.revisions.History(ctx, base, offset, limit)
	if err != nil {
		return nil, err
	}
	resp.TypeUrl = typeURL
	return resp, nil
}

func (s *StorageProtoObject) Watch(ctx context.Context, typeURL, prefix string) (<-chan *keyvalue_v1alpha1.WatchEvent, error) {
	base := s.keyPath(typeURL, prefix)
	resp, err := s.revisions.Watch(ctx, base)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
