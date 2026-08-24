package deployment

import (
	"context"

	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// TODO : consolidate these proto helpers somewhere else in pkg/storage maybe.
func newMessage[T proto.Message]() T {
	var t T
	return t.ProtoReflect().New().Interface().(T)
}

func typeURL[T proto.Message]() string {
	any, err := anypb.New(newMessage[T]())
	if err != nil {
		panic(err)
	}
	return any.GetTypeUrl()
}

func putProto[T proto.Message](ctx context.Context, s object.TypeURLStore, key string, msg T) error {
	any, err := anypb.New(msg)
	if err != nil {
		return err
	}
	_, err = s.Put(ctx, any.GetTypeUrl(), key, 0, any)
	return err
}

func getProto[T proto.Message](ctx context.Context, s object.TypeURLStore, key string) (T, error) {
	obj, err := s.Get(ctx, typeURL[T](), key)
	if err != nil {
		var zero T
		return zero, err
	}
	msg := newMessage[T]()
	if err := obj.GetObj().UnmarshalTo(msg); err != nil {
		var zero T
		return zero, err
	}
	return msg, nil
}

// getProtoOrNil returns the zero value when the key has never been written.
func getProtoOrNil[T proto.Message](ctx context.Context, s object.TypeURLStore, key string) (T, error) {
	msg, err := getProto[T](ctx, s, key)
	if grpcutil.IsErrorNotFound(err) {
		var zero T
		return zero, nil
	}
	return msg, err
}

func historyProto[T proto.Message](ctx context.Context, s object.TypeURLStore, key string, offset, limit uint64) ([]T, error) {
	resp, err := s.History(ctx, typeURL[T](), key, offset, limit)
	if grpcutil.IsErrorNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(resp.GetObjs()))
	for _, obj := range resp.GetObjs() {
		if obj.GetObj() == nil {
			continue
		}
		msg := newMessage[T]()
		if err := obj.GetObj().UnmarshalTo(msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}
