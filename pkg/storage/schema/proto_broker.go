package schema

import (
	"context"
	"path"
	"sort"

	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type SchemaProto interface {
	Put(ctx context.Context, typeURL, key string, revision uint64, obj *anypb.Any) (*keyvaluev1.KeyValueObject, error)
	Get(ctx context.Context, typeURL, key string) (*keyvaluev1.KeyValueObject, error)
	GetRevision(ctx context.Context, typeURL, key string, revision uint64) (*keyvaluev1.KeyValueObject, error)
	ListKeys(ctx context.Context, typeURL string) ([]string, error)
	List(ctx context.Context, typeURL string) ([]*keyvaluev1.KeyValueObject, error)
	Delete(ctx context.Context, typeURL, key string) error
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

const (
	defaultVersion = "v1alpha1"
)

type StorageSchemaProto struct {
	baseVersion string
	underlying  types.KV
	revisions   *RevisionEngine
}

func NewStorageSchemaProto(
	kv types.KV,
) *StorageSchemaProto {
	return &StorageSchemaProto{
		baseVersion: defaultVersion,
		underlying:  kv,
		revisions:   NewRevisionEngine(kv),
	}
}

var _ SchemaProto = (*StorageSchemaProto)(nil)

func (s *StorageSchemaProto) protoPath(typeURL string) string {
	return path.Join(s.baseVersion, typeURL)
}

func (s *StorageSchemaProto) keyPath(typeURL, key string) string {
	return path.Join(s.protoPath(typeURL), key)
}

func (s *StorageSchemaProto) Put(ctx context.Context, typeURL string, key string, revision uint64, obj *anypb.Any) (*keyvaluev1.KeyValueObject, error) {
	return s.revisions.Put(ctx, s.keyPath(typeURL, key), typeURL, revision, obj)
}

func (s *StorageSchemaProto) Get(ctx context.Context, typeURL string, key string) (*keyvaluev1.KeyValueObject, error) {
	return s.revisions.Get(ctx, s.keyPath(typeURL, key))
}

func (s *StorageSchemaProto) GetRevision(ctx context.Context, typeURL, key string, revision uint64) (*keyvaluev1.KeyValueObject, error) {
	return s.revisions.GetRevision(ctx, s.keyPath(typeURL, key), revision)
}

func (s *StorageSchemaProto) ListKeys(ctx context.Context, typeURL string) ([]string, error) {
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

func (s *StorageSchemaProto) List(ctx context.Context, typeURL string) ([]*keyvaluev1.KeyValueObject, error) {
	latest, err := s.revisions.ListLatest(ctx, s.protoPath(typeURL))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	objs := make([]*keyvaluev1.KeyValueObject, 0, len(latest))
	for _, key := range keys {
		objs = append(objs, latest[key])
	}
	return objs, nil
}

func (s *StorageSchemaProto) Delete(ctx context.Context, typeURL, key string) error {
	return s.revisions.Delete(ctx, s.keyPath(typeURL, key))
}
