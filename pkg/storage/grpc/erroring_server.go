package grpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
)

type erroringServer struct {
	err  error
	code connect.Code
}

func NewErroringServer(code connect.Code, err error) v1alpha1connect.KeyValueServiceHandler {
	return &erroringServer{
		code: code,
		err:  err,
	}
}

var _ v1alpha1connect.KeyValueServiceHandler = (*erroringServer)(nil)

func (e *erroringServer) Get(context.Context, *connect.Request[v1alpha1.GetRequest]) (*connect.Response[v1alpha1.GetResponse], error) {
	return nil, connect.NewError(e.code, e.err)
}

func (e *erroringServer) Put(context.Context, *connect.Request[v1alpha1.PutRequest]) (*connect.Response[v1alpha1.PutResponse], error) {
	return nil, connect.NewError(e.code, e.err)
}
func (e *erroringServer) ListKeys(context.Context, *connect.Request[v1alpha1.ListKeysRequest]) (*connect.Response[v1alpha1.ListKeysResponse], error) {
	return nil, connect.NewError(e.code, e.err)
}
func (e *erroringServer) List(context.Context, *connect.Request[v1alpha1.ListRequest]) (*connect.Response[v1alpha1.ListResponse], error) {
	return nil, connect.NewError(e.code, e.err)
}
func (e *erroringServer) Delete(context.Context, *connect.Request[v1alpha1.DeleteRequest]) (*connect.Response[v1alpha1.DeleteResponse], error) {
	return nil, connect.NewError(e.code, e.err)
}
