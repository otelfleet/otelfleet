package storage

import (
	"context"

	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewMessage[T proto.Message]() T {
	var t T
	return t.ProtoReflect().New().Interface().(T)
}

func unmarshalTyped[T proto.Message](obj *keyvaluev1.KeyValueObject) (T, error) {
	t := NewMessage[T]()
	if err := obj.GetObj().UnmarshalTo(t); err != nil {
		return t, err
	}
	return t, nil
}

type schemaWrapper[T proto.Message] struct {
	underlying schema.SchemaProto
}

func NewProtoKVFromSchemaImpl[T proto.Message](
	schema schema.SchemaProto,
) types.KeyValue[T] {
	return &schemaWrapper[T]{
		underlying: schema,
	}
}

func (w *schemaWrapper[T]) typeURL() string {
	any, err := anypb.New(NewMessage[T]())
	if err != nil {
		panic(err)
	}
	return any.GetTypeUrl()
}

func (w *schemaWrapper[T]) Put(ctx context.Context, key string, obj T) error {
	_, err := w.PutRevision(ctx, key, 0, obj)
	return err
}

func (w *schemaWrapper[T]) PutRevision(ctx context.Context, key string, revision uint64, obj T) (uint64, error) {
	any, err := anypb.New(obj)
	if err != nil {
		return 0, err
	}
	stored, err := w.underlying.Put(ctx, any.GetTypeUrl(), key, revision, any)
	if err != nil {
		return 0, err
	}
	return stored.GetRevision(), nil
}

func (w *schemaWrapper[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	obj, err := w.underlying.Get(ctx, w.typeURL(), key)
	if err != nil {
		return t, err
	}
	return unmarshalTyped[T](obj)
}

func (w *schemaWrapper[T]) GetRevision(ctx context.Context, key string, revision uint64) (T, error) {
	var t T
	obj, err := w.underlying.GetRevision(ctx, w.typeURL(), key, revision)
	if err != nil {
		return t, err
	}
	t, err = unmarshalTyped[T](obj)
	return t, err
}

func (w *schemaWrapper[T]) ListKeys(ctx context.Context) ([]string, error) {
	return w.underlying.ListKeys(ctx, w.typeURL())
}

func (w *schemaWrapper[T]) List(ctx context.Context) ([]T, error) {
	objs, err := w.underlying.List(ctx, w.typeURL())
	if err != nil {
		return nil, err
	}
	ret := make([]T, len(objs))
	for idx, obj := range objs {
		t, err := unmarshalTyped[T](obj)
		if err != nil {
			return nil, err
		}
		ret[idx] = t
	}
	return ret, nil
}

func (w *schemaWrapper[T]) Delete(ctx context.Context, key string) error {
	return w.underlying.Delete(ctx, w.typeURL(), key)
}

func (w *schemaWrapper[T]) History(ctx context.Context, key string, offset uint64, limit uint64) ([]T, error) {
	resp, err := w.underlying.History(ctx, w.typeURL(), key, offset, limit)
	if err != nil {
		return nil, err
	}
	ret := make([]T, len(resp.GetObjs()))
	for idx, obj := range resp.GetObjs() {
		t, err := unmarshalTyped[T](obj)
		if err != nil {
			return nil, err
		}
		ret[idx] = t
	}
	return ret, nil
}
