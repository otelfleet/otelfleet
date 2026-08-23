package grpc

import (
	"context"

	"connectrpc.com/connect"
	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"google.golang.org/protobuf/types/known/anypb"
)

type StorageClient struct {
	client v1alpha1connect.KeyValueServiceClient
}

func NewRemoteKV(client v1alpha1connect.KeyValueServiceClient) *StorageClient {
	return &StorageClient{
		client: client,
	}
}

var _ schema.ProtoObjectStore = (*StorageClient)(nil)

func (s *StorageClient) Put(ctx context.Context, typeURL, key string, revision uint64, obj *anypb.Any) (*keyvaluev1.KeyValueObject, error) {
	resp, err := s.client.Put(ctx, connect.NewRequest(&keyvaluev1.PutRequest{
		TypeUrl:  typeURL,
		Key:      key,
		Data:     obj,
		Revision: revision,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetObject(), nil
}
func (s *StorageClient) Get(ctx context.Context, typeURL, key string) (*keyvaluev1.KeyValueObject, error) {
	resp, err := s.client.Get(ctx, connect.NewRequest(&keyvaluev1.GetRequest{
		TypeUrl: typeURL,
		Key:     key,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetObject(), nil
}
func (s *StorageClient) GetRevision(ctx context.Context, typeURL, key string, revision uint64) (*keyvaluev1.KeyValueObject, error) {
	resp, err := s.client.Get(ctx, connect.NewRequest(&keyvaluev1.GetRequest{
		TypeUrl:  typeURL,
		Key:      key,
		Revision: revision,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetObject(), nil
}
func (s *StorageClient) ListKeys(ctx context.Context, typeURL string) ([]string, error) {
	resp, err := s.client.ListKeys(ctx, connect.NewRequest(&keyvaluev1.ListKeysRequest{
		TypeUrl: typeURL,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetKeys(), nil
}
func (s *StorageClient) List(ctx context.Context, typeURL string) ([]*keyvaluev1.KeyValueObject, error) {
	resp, err := s.client.List(ctx, connect.NewRequest(&keyvaluev1.ListRequest{
		TypeUrl: typeURL,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetObjects(), nil
}
func (s *StorageClient) Delete(ctx context.Context, typeURL, key string) error {
	_, err := s.client.Delete(ctx, connect.NewRequest(&keyvaluev1.DeleteRequest{
		TypeUrl: typeURL,
		Key:     key,
	}))
	return err
}

func (s *StorageClient) History(ctx context.Context, typeURL, key string, offset, limit uint64) (*keyvaluev1.GetHistoryResponse, error) {
	resp, err := s.client.History(ctx, connect.NewRequest(&keyvaluev1.GetHistoryRequest{}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

const bufN = 16

func (s *StorageClient) Watch(ctx context.Context, typeURL, prefix string) (<-chan *keyvaluev1.WatchEvent, error) {
	srv, err := s.client.Watch(ctx, connect.NewRequest(&keyvaluev1.WatchRequest{
		TypeUrl: typeURL,
		Prefix:  prefix,
	}))
	if err != nil {
		return nil, err
	}
	sendC := make(chan *keyvaluev1.WatchEvent, 16)
	go func() {
		for {
			select {
			case <-ctx.Done():
			default:
				if !srv.Receive() {
					close(sendC)
				}
				msg := srv.Msg()
				sendC <- msg
			}
		}
	}()
	return sendC, nil
}
