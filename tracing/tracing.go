// Package tracing wires OpenTelemetry distributed tracing for Go services.
//
// Tracing is opt-in: when OTEL_EXPORTER_OTLP_ENDPOINT is unset (the default in
// local dev and tests) InitTracing is a no-op, so nothing tries to reach a
// collector. When it is set, spans are exported via OTLP/gRPC to that endpoint
// (e.g. an in-cluster otel-collector) and W3C trace-context propagation is
// enabled so traces stitch together across gRPC/HTTP service boundaries.
//
// The actual span creation at the gRPC boundary comes from
// otelgrpc.NewServerHandler() registered on each grpc.Server; this package only
// owns global SDK setup (provider, exporter, propagator).
package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// InitTracing configures the global OpenTelemetry tracer provider and text-map
// propagator and returns a shutdown func that flushes pending spans. Callers
// should defer the returned func.
//
// If OTEL_EXPORTER_OTLP_ENDPOINT is unset, tracing is disabled and the returned
// shutdown func is a no-op. The OTLP exporter otherwise reads its endpoint and
// transport settings from the standard OTEL_* environment variables; an
// http:// scheme selects an insecure connection (expected for in-cluster use).
func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noop, nil
	}

	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating otlp trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithFromEnv(), // honors OTEL_RESOURCE_ATTRIBUTES / OTEL_SERVICE_NAME
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("building otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// End ends span, first recording err on it (and marking its status failed) when
// err is non-nil. It is meant to be deferred against a named error return so a
// single line instruments every return path of a function:
//
//	func f(ctx context.Context) (err error) {
//		ctx, span := tracer.Start(ctx, "pkg.f")
//		defer func() { tracing.End(span, err) }()
//		...
//	}
//
// For a span wrapping a single call it can also be invoked inline after the call.
// When tracing is disabled the span is a no-op, so this is cheap to leave in.
func End(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
