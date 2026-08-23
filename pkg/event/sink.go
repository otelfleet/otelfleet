package event

import "context"

type EventSink interface {
	Info(ctx context.Context)
	Warn(ctx context.Context)
	Error(ctx context.Context)
}
