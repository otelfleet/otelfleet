package event

import (
	"context"
	"sort"

	"connectrpc.com/connect"
	"github.com/grafana/dskit/services"
	"github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/event"
	otelfleet_svc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
)

type Server struct {
	eventKv object.KeyValue[*v1alpha1.Event]
	services.Service
}

var _ otelfleet_svc.HTTPService = (*Server)(nil)
var _ v1alpha1connect.EventServiceHandler = (*Server)(nil)

// FIXME: this is not optimized. We need some querying / indexing primitives in storage layer
func NewServer(kv object.KeyValue[*v1alpha1.Event]) *Server {
	srv := &Server{
		eventKv: kv,
	}

	srv.Service = services.NewBasicService(srv.starting, srv.running, srv.stopping)

	return srv
}

func (s *Server) starting(ctx context.Context) error {
	return nil
}

func (s *Server) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *Server) stopping(_ error) error {
	return nil
}

type eventFilter struct {
	severity *v1alpha1.EventSeverity
	group    string
}

func (f eventFilter) matches(evt *v1alpha1.Event) bool {
	if f.severity != nil && evt.GetSeverity() != *f.severity {
		return false
	}
	if f.group != "" && evt.GetGroup() != f.group {
		return false
	}
	return true
}

func sortByReportedAtDesc(events []*v1alpha1.Event) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].GetReportedAt().AsTime().After(events[j].GetReportedAt().AsTime())
	})
}

func (s *Server) listFiltered(ctx context.Context, f eventFilter) ([]*v1alpha1.Event, error) {
	events, err := s.eventKv.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	matched := make([]*v1alpha1.Event, 0, len(events))
	for _, evt := range events {
		if f.matches(evt) {
			matched = append(matched, evt)
		}
	}
	return matched, nil
}

func (s *Server) GetGroups(context.Context, *connect.Request[v1alpha1.GetGroupRequest]) (*connect.Response[v1alpha1.GetGroupResponse], error) {
	// FIXME: hack
	return connect.NewResponse(&v1alpha1.GetGroupResponse{
		Groups: []string{
			event.GroupCollector,
		},
	}), nil
}

func (s *Server) List(ctx context.Context, req *connect.Request[v1alpha1.ListEventRequest]) (*connect.Response[v1alpha1.ListEventResponse], error) {
	events, err := s.listFiltered(ctx, eventFilter{
		severity: req.Msg.Severity,
		group:    req.Msg.GetGroup(),
	})
	if err != nil {
		return nil, err
	}
	sortByReportedAtDesc(events)
	if limit := int(req.Msg.GetLimit()); limit > 0 && limit < len(events) {
		events = events[:limit]
	}
	return connect.NewResponse(&v1alpha1.ListEventResponse{Events: events}), nil
}

func (s *Server) Watch(ctx context.Context, req *connect.Request[v1alpha1.WatchEventRequest], stream *connect.ServerStream[v1alpha1.WatchEventResponse]) error {
	f := eventFilter{
		severity: req.Msg.Severity,
		group:    req.Msg.GetGroup(),
	}

	events, err := s.listFiltered(ctx, f)
	if err != nil {
		return err
	}
	if err := stream.Send(&v1alpha1.WatchEventResponse{Events: events}); err != nil {
		return err
	}

	watchC, err := s.eventKv.Watch(ctx, "")
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case obj, ok := <-watchC:
			if !ok {
				return nil
			}
			if obj.Deleted || !f.matches(obj.Object) {
				continue
			}
			if err := stream.Send(&v1alpha1.WatchEventResponse{
				Events: []*v1alpha1.Event{obj.Object},
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Server) ConfigureHTTP(reg otelfleet_svc.HTTPRegistrar) {
	v1alpha1connect.RegisterEventServiceHandler(reg.Router(), s, reg.ConnectOptions()...)
}
