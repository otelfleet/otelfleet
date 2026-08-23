package storage

import (
	"context"
	"fmt"

	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
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
	underlying schema.ProtoObjectStore
}

func NewProtoKVFromSchemaImpl[T proto.Message](
	schema schema.ProtoObjectStore,
) object.KeyValue[T] {
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

const bufN = 16

func (w *schemaWrapper[T]) Watch(ctx context.Context, prefix string) (<-chan object.RevisionObject[T], error) {
	resp, err := w.underlying.Watch(ctx, w.typeURL(), prefix)
	if err != nil {
		return nil, err
	}
	sendC := make(chan object.RevisionObject[T], bufN)
	go func() {
		defer close(sendC)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-resp:
				if !ok {
					return
				}
				evt, err := revisionObjectFromEvent[T](msg)
				if err != nil {
					continue
				}
				select {
				case sendC <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return sendC, nil
}

func revisionObjectFromEvent[T proto.Message](msg *keyvaluev1.WatchEvent) (object.RevisionObject[T], error) {
	switch e := msg.GetEventType().(type) {
	case *keyvaluev1.WatchEvent_DeletedKey:
		return object.RevisionObject[T]{
			Key:     e.DeletedKey,
			Deleted: true,
		}, nil
	case *keyvaluev1.WatchEvent_Modified:
		obj, err := unmarshalTyped[T](e.Modified)
		if err != nil {
			return object.RevisionObject[T]{}, err
		}
		return object.RevisionObject[T]{
			// TODO: KeyValueObject carries no key, so Key is unset for modifications.
			//Key:      "TODO",
			Revision: e.Modified.GetRevision(),
			Object:   obj,
		}, nil
	default:
		panic(fmt.Sprintf("unknown watch event type %T", e))
	}
}
