package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider bundles the OpenTelemetry tracer provider and its shutdown
// hook. It is constructed once in the composition root (internal/app)
// and handed to every subsystem that needs to start spans.
type Provider struct {
	TracerProvider trace.TracerProvider
	shutdown       func(context.Context) error
}

// NewProvider configures tracing based on cfg.OTelExporter:
//   - "stdout": spans are pretty-printed to stdout (local dev default)
//   - "otlp":   spans are exported via OTLP/gRPC to cfg.OTLPEndpoint
//   - "none":   tracing is a no-op
func NewProvider(ctx context.Context, exporterKind, otlpEndpoint, serviceName string) (*Provider, error) {
	if exporterKind == "none" {
		tp := otel.GetTracerProvider() // no-op default
		return &Provider{TracerProvider: tp, shutdown: func(context.Context) error { return nil }}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch exporterKind {
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "otlp":
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("telemetry: unknown exporter kind %q", exporterKind)
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry: build exporter %q: %w", exporterKind, err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return &Provider{
		TracerProvider: tp,
		shutdown:       tp.Shutdown,
	}, nil
}

// Shutdown flushes and releases the underlying exporter. Safe to call
// even when tracing is disabled.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

// Tracer is a convenience for subsystems to grab a named tracer without
// importing otel directly everywhere.
func (p *Provider) Tracer(name string) trace.Tracer {
	return p.TracerProvider.Tracer(name)
}
