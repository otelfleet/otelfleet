package traceutil

import (
	"context"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {

	}
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
	)
	return tp, nil
}
