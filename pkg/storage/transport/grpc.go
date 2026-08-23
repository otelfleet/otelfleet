package transport

import (
	"context"
	"io"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/object/driver/basekv"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
)

type GrpcKeyValue struct {
	underlying *basekv.StorageProtoObject
}

func NewKVServer(underlying *basekv.StorageProtoObject) *GrpcKeyValue {
	return &GrpcKeyValue{
		underlying: underlying,
	}
}

var _ v1alpha1connect.KeyValueServiceHandler = (*GrpcKeyValue)(nil)

func toConnectError(err error) error {
	switch {
	case err == nil:
		return nil
	case grpcutil.IsError(codes.Aborted, err):
		return connect.NewError(connect.CodeAborted, err)
	case grpcutil.IsError(codes.InvalidArgument, err):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case grpcutil.IsError(codes.NotFound, err):
		return connect.NewError(connect.CodeNotFound, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func (k *GrpcKeyValue) Get(ctx context.Context, req *connect.Request[v1alpha1.GetRequest]) (*connect.Response[v1alpha1.GetResponse], error) {
	typeURL := req.Msg.GetTypeUrl()

	var got *v1alpha1.KeyValueObject
	var err error
	if rev := req.Msg.GetRevision(); rev > 0 {
		got, err = k.underlying.GetRevision(ctx, typeURL, req.Msg.GetKey(), rev)
	} else {
		got, err = k.underlying.Get(ctx, typeURL, req.Msg.GetKey())
	}
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&v1alpha1.GetResponse{
		TypeUrl: typeURL,
		Object:  got,
	}), nil
}

func (k *GrpcKeyValue) Put(ctx context.Context, req *connect.Request[v1alpha1.PutRequest]) (*connect.Response[v1alpha1.PutResponse], error) {
	stored, err := k.underlying.Put(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey(), req.Msg.GetRevision(), req.Msg.GetData())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1alpha1.PutResponse{Object: stored}), nil
}

func (k *GrpcKeyValue) ListKeys(ctx context.Context, req *connect.Request[v1alpha1.ListKeysRequest]) (*connect.Response[v1alpha1.ListKeysResponse], error) {
	keys, err := k.underlying.ListKeys(ctx, req.Msg.GetTypeUrl())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1alpha1.ListKeysResponse{
		Keys: keys,
	}), nil
}

func (k *GrpcKeyValue) List(ctx context.Context, req *connect.Request[v1alpha1.ListRequest]) (*connect.Response[v1alpha1.ListResponse], error) {
	objs, err := k.underlying.List(ctx, req.Msg.GetTypeUrl())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1alpha1.ListResponse{
		TypeUrl: req.Msg.GetTypeUrl(),
		Objects: objs,
	}), nil
}

func (k *GrpcKeyValue) Delete(ctx context.Context, req *connect.Request[v1alpha1.DeleteRequest]) (*connect.Response[v1alpha1.DeleteResponse], error) {
	if err := k.underlying.Delete(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1alpha1.DeleteResponse{}), nil
}

func (k *GrpcKeyValue) History(ctx context.Context, req *connect.Request[v1alpha1.GetHistoryRequest]) (*connect.Response[v1alpha1.GetHistoryResponse], error) {
	resp, err := k.underlying.History(
		ctx, req.Msg.GetTypeUrl(),
		req.Msg.GetKey(),
		req.Msg.GetQuery().GetOffset(),
		req.Msg.GetQuery().GetLimit(),
	)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (k *GrpcKeyValue) Watch(ctx context.Context, req *connect.Request[v1alpha1.WatchRequest], srv *connect.ServerStream[v1alpha1.WatchEvent]) error {
	resp, err := k.underlying.Watch(ctx, req.Msg.GetTypeUrl(), req.Msg.GetPrefix())
	if err != nil {
		return toConnectError(err)
	}

	eg, eCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for {
			select {
			case <-eCtx.Done():
				return context.Cause(eCtx)
			case msg, ok := <-resp:
				if !ok {
					return io.EOF
				}
				srv.Send(msg)
			}
		}
	})

	return eg.Wait()
}
