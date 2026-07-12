package schema

import (
	"context"
	"path"

	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type SchemaProto interface {
	Put(ctx context.Context, typeURL, key string, obj *anypb.Any) error
	Get(ctx context.Context, typeURL, key string) (*anypb.Any, error)
	ListKeys(ctx context.Context, typeURL string) ([]string, error)
	List(ctx context.Context, typeURL string) ([]*anypb.Any, error)
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

func decodeProto(data []byte) (*anypb.Any, error) {
	any := &anypb.Any{}
	err := unmarshalOptions.Unmarshal(data, any)
	if err != nil {
		return nil, err
	}
	return any, nil
}

const (
	defaultVersion = "v1alpha1"
)

type StorageSchemaProto struct {
	baseVersion string
	underlying  types.KV
}

func NewStorageSchemaProto(
	kv types.KV,
) *StorageSchemaProto {
	return &StorageSchemaProto{
		baseVersion: defaultVersion,
		underlying:  kv,
	}
}

var _ SchemaProto = (*StorageSchemaProto)(nil)

func (s *StorageSchemaProto) protoPath(typeURL string) string {
	return path.Join(s.baseVersion, typeURL)
}

func (s *StorageSchemaProto) keyPath(typeURL, key string) string {
	return path.Join(s.protoPath(typeURL), key)
}

func (s *StorageSchemaProto) Put(ctx context.Context, typeURL string, key string, obj *anypb.Any) error {
	return s.underlying.Put(ctx, s.keyPath(typeURL, key), encodeProto(obj))
}

func (s *StorageSchemaProto) Get(ctx context.Context, typeURL string, key string) (*anypb.Any, error) {
	data, err := s.underlying.Get(ctx, s.keyPath(typeURL, key))
	if err != nil {
		return nil, err
	}
	return decodeProto(data)
}

func (s *StorageSchemaProto) ListKeys(ctx context.Context, typeURL string) ([]string, error) {
	return s.underlying.ListKeys(ctx, s.protoPath(typeURL))
}

func (s *StorageSchemaProto) List(ctx context.Context, typeURL string) ([]*anypb.Any, error) {
	objs, err := s.underlying.List(ctx, s.protoPath(typeURL))
	if err != nil {
		return nil, err
	}
	anys := make([]*anypb.Any, len(objs))
	for i, obj := range objs {
		any, err := decodeProto(obj)
		if err != nil {
			// TODO : probably worth skipping instead of errorring
			return nil, err
		}
		anys[i] = any
	}
	return anys, nil
}

func (s *StorageSchemaProto) Delete(ctx context.Context, typeURL, key string) error {
	return s.underlying.Delete(ctx, s.keyPath(typeURL, key))
}
