package storage

import (
	"context"
	"log/slog"
	"sort"

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

func NewProtoKV[T proto.Message](
	logger *slog.Logger,
	kv types.BaseKV,
) types.KeyValue[T] {
	return &protoKeyValue[T]{
		revisions: schema.NewRevisionEngine(kv),
		logger:    logger,
	}
}

type protoKeyValue[T proto.Message] struct {
	logger    *slog.Logger
	revisions *schema.RevisionEngine
}

func (kv *protoKeyValue[T]) Put(ctx context.Context, key string, obj T) error {
	_, err := kv.PutRevision(ctx, key, 0, obj)
	return err
}

func (kv *protoKeyValue[T]) PutRevision(ctx context.Context, key string, revision uint64, obj T) (uint64, error) {
	any, err := anypb.New(obj)
	if err != nil {
		return 0, err
	}
	stored, err := kv.revisions.Put(ctx, key, any.GetTypeUrl(), revision, any)
	if err != nil {
		return 0, err
	}
	return stored.GetRevision(), nil
}

func (kv *protoKeyValue[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	obj, err := kv.revisions.Get(ctx, key)
	if err != nil {
		return t, err
	}
	return unmarshalTyped[T](obj)
}

func (kv *protoKeyValue[T]) GetRevision(ctx context.Context, key string, revision uint64) (T, error) {
	var t T
	obj, err := kv.revisions.GetRevision(ctx, key, revision)
	if err != nil {
		return t, err
	}
	t, err = unmarshalTyped[T](obj)
	return t, err
}

func (kv *protoKeyValue[T]) ListKeys(ctx context.Context) ([]string, error) {
	latest, err := kv.revisions.ListLatest(ctx, "")
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

func (kv *protoKeyValue[T]) List(ctx context.Context) ([]T, error) {
	latest, err := kv.revisions.ListLatest(ctx, "")
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ret := make([]T, 0, len(latest))
	for _, key := range keys {
		t, err := unmarshalTyped[T](latest[key])
		if err != nil {
			return nil, err
		}
		ret = append(ret, t)
	}
	return ret, nil
}

func (kv *protoKeyValue[T]) Delete(ctx context.Context, key string) error {
	return kv.revisions.Delete(ctx, key)
}
