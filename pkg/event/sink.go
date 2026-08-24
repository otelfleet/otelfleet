package event

import (
	"context"

	"github.com/google/uuid"
	"github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	GroupCollector = "collector"
)

type EventSink interface {
	Info(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error
	Warn(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error
	Error(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error
}

type EventKV struct {
	kv    object.KeyValue[*v1alpha1.Event]
	group string
}

func NewEventSink(
	kv object.KeyValue[*v1alpha1.Event],
	group string,
) *EventKV {
	return &EventKV{
		group: group,
		kv:    kv,
	}
}

func (e *EventKV) Info(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error {
	if details == nil {
		return status.Error(codes.FailedPrecondition, "details and ref must be non-nil")
	}

	return e.kv.Put(ctx, uuid.NewString(), &v1alpha1.Event{
		Severity:   v1alpha1.EventSeverity_EVENT_SEVERITY_INFO,
		Details:    details,
		Group:      e.group,
		ObjectRefs: refs,
		ReportedAt: timestamppb.Now(),
	})
}
func (e *EventKV) Warn(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error {
	if details == nil {
		return status.Error(codes.FailedPrecondition, "details and ref must be non-nil")
	}

	return e.kv.Put(ctx, uuid.NewString(), &v1alpha1.Event{
		Severity:   v1alpha1.EventSeverity_EVENT_SEVERITY_WARN,
		Details:    details,
		Group:      e.group,
		ObjectRefs: refs,
		ReportedAt: timestamppb.Now(),
	})
}
func (e *EventKV) Error(ctx context.Context, refs []*v1alpha1.EventRef, details *v1alpha1.EventDetails) error {
	if details == nil {
		return status.Error(codes.FailedPrecondition, "details and ref must be non-nil")
	}

	return e.kv.Put(ctx, uuid.NewString(), &v1alpha1.Event{
		Severity:   v1alpha1.EventSeverity_EVENT_SEVERITY_ERROR,
		Details:    details,
		Group:      e.group,
		ObjectRefs: refs,
		ReportedAt: timestamppb.Now(),
	})
}
