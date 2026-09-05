package traceutil

import (
	"context"
	"errors"
	"sync"

	"github.com/otelfleet/otelfleet/pkg/util/serviceutil"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Providers owns one TracerProvider per logical service. Resources are
// provider-scoped in OpenTelemetry, so separate providers are required for
// different service.name values.
type Providers struct {
	base     *resource.Resource
	exporter sdktrace.SpanExporter

	mu        sync.Mutex
	providers map[string]*sdktrace.TracerProvider
}

func InitTracer(ctx context.Context) (*Providers, error) {
	base, err := resource.New(ctx, resource.WithFromEnv(), resource.WithHost())
	if err != nil {
		return nil, err
	}
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}
	return &Providers{base: base, exporter: exporter, providers: make(map[string]*sdktrace.TracerProvider)}, nil
}

func (p *Providers) Provider(mod serviceutil.Module) trace.TracerProvider {
	service := TracingService(mod)
	p.mu.Lock()
	defer p.mu.Unlock()
	if provider := p.providers[service]; provider != nil {
		return provider
	}
	serviceResource, err := resource.Merge(p.base, resource.NewSchemaless(attribute.String("service.name", service)))
	if err != nil {
		panic(err) // The resources above have no conflicting schema URLs.
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(nonClosingExporter{p.exporter})),
		sdktrace.WithResource(serviceResource),
	)
	p.providers[service] = provider
	return provider
}

func (p *Providers) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	providers := make([]*sdktrace.TracerProvider, 0, len(p.providers))
	for _, provider := range p.providers {
		providers = append(providers, provider)
	}
	p.mu.Unlock()

	errs := make([]error, 0, len(providers)+1)
	for _, provider := range providers {
		errs = append(errs, provider.Shutdown(ctx))
	}
	errs = append(errs, p.exporter.Shutdown(ctx))
	return errors.Join(errs...)
}

type nonClosingExporter struct{ sdktrace.SpanExporter }

func (nonClosingExporter) Shutdown(context.Context) error { return nil }

func TracingService(svc serviceutil.Module) string {
	return svc.TracingService()
}

// Continue starts a new span using the tracer provider of the span in the given
// context.
//
// In most cases, it is better to start spans directly from a specific tracer,
// obtained via dependency injection or some other mechanism. This function is
// useful in shared code where the tracer used to start the span is not
// necessarily the same every time, but can change based on the call site.
func Continue(
	ctx context.Context,
	name string,
	serviceName serviceutil.Module,
	o ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	return trace.SpanFromContext(ctx).
		TracerProvider().
		Tracer(TracingService(serviceName)).
		Start(ctx, name, o...)
}
