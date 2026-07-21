package resource

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1/v1alpha1connect"
)

func (s *Server) validateTypeURL(typeURL string) error {
	if !slices.Contains(s.supportedTypes, typeURL) {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("supported entity types : %s, provided : %s", strings.Join(s.supportedTypes, ","), typeURL))
	}
	return nil
}

func (s *Server) GetEntity(ctx context.Context, req *connect.Request[v1alpha1.GetEntityRequest]) (*connect.Response[v1alpha1.GetEntityResponse], error) {
	if err := s.validateTypeURL(req.Msg.GetTypeUrl()); err != nil {
		return nil, err
	}

	resp, err := s.genericStorage.Get(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1alpha1.GetEntityResponse{
		Entity: &v1alpha1.ResourceEntity{
			TypeUrl: resp.GetTypeUrl(),
			Key:     req.Msg.GetKey(),
			Obj:     resp.Obj,
		},
		Revision: resp.GetRevision(),
	}), nil
}
func (s *Server) PutEntity(ctx context.Context, req *connect.Request[v1alpha1.PutEntityRequest]) (*connect.Response[v1alpha1.PutEntityResponse], error) {
	if err := s.validateTypeURL(req.Msg.GetEntity().GetTypeUrl()); err != nil {
		return nil, err
	}

	resp, err := s.genericStorage.Put(ctx, req.Msg.Entity.GetTypeUrl(), req.Msg.GetEntity().GetKey(), 0, req.Msg.GetEntity().GetObj())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&v1alpha1.PutEntityResponse{
		Revision: resp.GetRevision(),
	}), nil
}

func (s *Server) DeleteEntity(ctx context.Context, req *connect.Request[v1alpha1.DeleteEntityRequest]) (*connect.Response[v1alpha1.DeleteEntityResponse], error) {
	if err := s.validateTypeURL(req.Msg.GetTypeUrl()); err != nil {
		return nil, err
	}

	if err := s.genericStorage.Delete(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1alpha1.DeleteEntityResponse{}), nil
}
func (s *Server) ListEntity(ctx context.Context, req *connect.Request[v1alpha1.ListEntityRequest]) (*connect.Response[v1alpha1.ListEntityResponse], error) {
	if err := s.validateTypeURL(req.Msg.GetTypeUrl()); err != nil {
		return nil, err
	}

	keys, err := s.genericStorage.ListKeys(ctx, req.Msg.GetTypeUrl())
	if err != nil {
		return nil, err
	}
	objs, err := s.genericStorage.List(ctx, req.Msg.GetTypeUrl())
	if err != nil {
		return nil, err
	}

	entities := make([]*v1alpha1.ResourceEntity, 0, len(objs))
	for i, obj := range objs {
		entities = append(entities, &v1alpha1.ResourceEntity{
			TypeUrl: obj.GetTypeUrl(),
			Key:     keys[i],
			Obj:     obj.GetObj(),
		})
	}
	return connect.NewResponse(&v1alpha1.ListEntityResponse{
		Entities: entities,
	}), nil
}
func (s *Server) WatchEntity(_ context.Context, req *connect.Request[v1alpha1.WatchEntityRequest], srv *connect.ServerStream[v1alpha1.WatchEntityResponse]) error {
	if err := s.validateTypeURL(req.Msg.GetTypeUrl()); err != nil {
		return err
	}
	return connect.NewError(connect.CodeUnimplemented, fmt.Errorf("unimplemented"))
}
func (s *Server) HistoryEntity(ctx context.Context, req *connect.Request[v1alpha1.HistoryEntityRequest]) (*connect.Response[v1alpha1.HistoryEntityResponse], error) {
	if err := s.validateTypeURL(req.Msg.GetTypeUrl()); err != nil {
		return nil, err
	}

	history, err := s.genericStorage.History(ctx, req.Msg.GetTypeUrl(), req.Msg.GetKey(), req.Msg.GetOffset(), req.Msg.GetLimit())
	if err != nil {
		return nil, err
	}

	entities := make([]*v1alpha1.ResourceEntity, 0, len(history.GetObjs()))
	for _, obj := range history.GetObjs() {
		entities = append(entities, &v1alpha1.ResourceEntity{
			TypeUrl: obj.GetTypeUrl(),
			Key:     req.Msg.GetKey(),
			Obj:     obj.GetObj(),
		})
	}
	return connect.NewResponse(&v1alpha1.HistoryEntityResponse{
		Entities: entities,
	}), nil
}

var _ v1alpha1connect.ResourceServiceHandler = (*Server)(nil)
