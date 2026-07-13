package storage

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewMessage[T proto.Message]() T {
	var t T
	return t.ProtoReflect().New().Interface().(T)
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
	any, err := anypb.New(obj)
	if err != nil {
		return err
	}
	return w.underlying.Put(ctx, any.GetTypeUrl(), key, any)
}

func (w *schemaWrapper[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	any, err := w.underlying.Get(ctx, w.typeURL(), key)
	if err != nil {
		return t, err
	}
	t = NewMessage[T]()
	if err := any.UnmarshalTo(t); err != nil {
		return t, err
	}
	return t, nil
}

func (w *schemaWrapper[T]) ListKeys(ctx context.Context) ([]string, error) {
	return w.underlying.ListKeys(ctx, w.typeURL())
}

func (w *schemaWrapper[T]) List(ctx context.Context) ([]T, error) {
	anys, err := w.underlying.List(ctx, w.typeURL())
	if err != nil {
		return nil, err
	}
	ret := make([]T, len(anys))
	for idx, any := range anys {
		t := NewMessage[T]()
		if err := any.UnmarshalTo(t); err != nil {
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
	kv types.KV,
) types.KeyValue[T] {
	return &protoKeyValue[T]{
		underlying: kv,
		logger:     logger,
	}
}

type protoKeyValue[T proto.Message] struct {
	logger     *slog.Logger
	underlying types.KV
}

func (kv *protoKeyValue[T]) Put(ctx context.Context, key string, obj T) error {
	data, err := proto.Marshal(obj)
	if err != nil {
		return err
	}

	return kv.underlying.Put(ctx, key, data)
}
func (kv *protoKeyValue[T]) Get(ctx context.Context, key string) (T, error) {
	var t T
	raw, err := kv.underlying.Get(ctx, key)
	if err != nil {
		return t, err
	}
	t = NewMessage[T]()
	if err := proto.Unmarshal(raw, t); err != nil {
		return t, err
	}
	return t, nil
}

func (kv *protoKeyValue[T]) ListKeys(ctx context.Context) ([]string, error) {
	return kv.underlying.ListKeys(ctx, "")
}
func (kv *protoKeyValue[T]) List(ctx context.Context) ([]T, error) {
	raw, err := kv.underlying.List(ctx, "")
	if err != nil {
		return nil, err
	}
	ret := make([]T, len(raw))
	for idx, el := range raw {
		t := NewMessage[T]()
		if err := proto.Unmarshal(el, t); err != nil {
			kv.logger.With("type", reflect.TypeOf(t)).With("error", err).Error("failed to unmarshal proto-type")
			continue
		}
		ret[idx] = t
	}
	return ret, nil

}
func (kv *protoKeyValue[T]) Delete(ctx context.Context, key string) error {
	return kv.underlying.Delete(ctx, key)
}
