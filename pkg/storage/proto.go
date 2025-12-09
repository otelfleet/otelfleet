package storage

import (
	"context"
	"log/slog"
	"reflect"

	"google.golang.org/protobuf/proto"
)

func NewProtoKV[T proto.Message](
	logger *slog.Logger,
	kv KV,
) KeyValue[T] {
	return &protoKeyValue[T]{
		underlying: kv,
		logger:     logger,
	}
}

type protoKeyValue[T proto.Message] struct {
	logger     *slog.Logger
	underlying KV
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
	return kv.underlying.ListKeys(ctx)
}
func (kv *protoKeyValue[T]) List(ctx context.Context) ([]T, error) {
	raw, err := kv.underlying.List(ctx)
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

func NewMessage[T proto.Message]() T {
	var t T
	return t.ProtoReflect().New().Interface().(T)
}
