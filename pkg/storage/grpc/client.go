package grpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"google.golang.org/protobuf/types/known/anypb"
)

type StorageClient struct {
	client v1alpha1connect.KeyValueServiceClient
}

func NewStorageClient(client v1alpha1connect.KeyValueServiceClient) *StorageClient {
	return &StorageClient{
		client: client,
	}
}

var _ schema.SchemaProto = (*StorageClient)(nil)

func (s *StorageClient) Put(ctx context.Context, typeURL, key string, obj *anypb.Any) error {
	_, err := s.client.Put(ctx, connect.NewRequest(&v1alpha1.PutRequest{
		TypeUrl: typeURL,
		Key:     key,
		Data:    obj,
	}))
	return err
}
func (s *StorageClient) Get(ctx context.Context, typeURL, key string) (*anypb.Any, error) {
	resp, err := s.client.Get(ctx, connect.NewRequest(&v1alpha1.GetRequest{
		TypeUrl: typeURL,
		Key:     key,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetData(), nil
}
func (s *StorageClient) ListKeys(ctx context.Context, typeURL string) ([]string, error) {
	resp, err := s.client.ListKeys(ctx, connect.NewRequest(&v1alpha1.ListKeysRequest{
		TypeUrl: typeURL,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetKeys(), nil
}
func (s *StorageClient) List(ctx context.Context, typeURL string) ([]*anypb.Any, error) {
	resp, err := s.client.List(ctx, connect.NewRequest(&v1alpha1.ListRequest{
		TypeUrl: typeURL,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetData(), nil
}
func (s *StorageClient) Delete(ctx context.Context, typeURL, key string) error {
	_, err := s.client.Delete(ctx, connect.NewRequest(&v1alpha1.DeleteRequest{
		TypeUrl: typeURL,
		Key:     key,
	}))
	return err
}
