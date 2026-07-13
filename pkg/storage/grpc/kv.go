package grpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
)

type GrpcKeyValue struct {
	underlying *schema.StorageSchemaProto
}

// TODO : handle storage errors and return appropriate error codes?

func NewGrpcKeyValue(underlying *schema.StorageSchemaProto) *GrpcKeyValue {
	return &GrpcKeyValue{
		underlying: underlying,
	}
}

var _ v1alpha1connect.KeyValueServiceHandler = (*GrpcKeyValue)(nil)

func (k *GrpcKeyValue) Get(ctx context.Context, req *connect.Request[v1alpha1.GetRequest]) (*connect.Response[v1alpha1.GetResponse], error) {
	typeURL := req.Msg.GetTypeUrl()

	got, err := k.underlying.Get(ctx, typeURL, req.Msg.GetKey())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1alpha1.GetResponse{
		TypeUrl: typeURL,
		Data:    got,
	}), nil
}
func (k *GrpcKeyValue) Put(ctx context.Context, req *connect.Request[v1alpha1.PutRequest]) (*connect.Response[v1alpha1.PutResponse], error) {
	if err := k.underlying.Put(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey(), req.Msg.GetData()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1alpha1.PutResponse{}), nil
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
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1alpha1.ListResponse{
		TypeUrl: req.Msg.GetTypeUrl(),
		Data:    objs,
	}), nil
}
func (k *GrpcKeyValue) Delete(ctx context.Context, req *connect.Request[v1alpha1.DeleteRequest]) (*connect.Response[v1alpha1.DeleteResponse], error) {
	if err := k.underlying.Delete(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1alpha1.DeleteResponse{}), nil
}
