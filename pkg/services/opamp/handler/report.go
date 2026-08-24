package handler

import (
	"context"

	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	eventv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/event/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/util/protoutil"
	"github.com/samber/lo"
)

func (s *CollectorHandler) reportInfo(ctx context.Context, reason string, extraRefs ...*eventv1alpha1.EventRef) {
	s.reporter.Info(ctx, append([]*eventv1alpha1.EventRef{
		{
			TypeUrl: protoutil.GetTypeURL(&v1alpha1.CollectorDescription{}),
			Key:     lo.FromPtrOr(s.collectorID, ""),
		},
	}, extraRefs...), &eventv1alpha1.EventDetails{
		Reason: reason,
	})
}

func (s *CollectorHandler) reportError(ctx context.Context, reason string, extraRefs ...*eventv1alpha1.EventRef) {
	s.reporter.Error(ctx, append([]*eventv1alpha1.EventRef{
		{
			TypeUrl: protoutil.GetTypeURL(&v1alpha1.CollectorDescription{}),
			Key:     lo.FromPtrOr(s.collectorID, ""),
		},
	}, extraRefs...), &eventv1alpha1.EventDetails{
		Reason: reason,
	})
}

func (s *CollectorHandler) reportWarn(ctx context.Context, reason string, extraRefs ...*eventv1alpha1.EventRef) {
	s.reporter.Warn(ctx, append([]*eventv1alpha1.EventRef{
		{
			TypeUrl: protoutil.GetTypeURL(&v1alpha1.CollectorDescription{}),
			Key:     lo.FromPtrOr(s.collectorID, ""),
		},
	}, extraRefs...), &eventv1alpha1.EventDetails{
		Reason: reason,
	})
}
