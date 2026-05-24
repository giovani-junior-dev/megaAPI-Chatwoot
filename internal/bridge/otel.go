package bridge

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

// InitTracer wires an OTLP/HTTP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is
// set; otherwise it is a no-op so the binary stays usable without an
// observability backend.
func InitTracer(ctx context.Context) (ShutdownFunc, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noopShutdown, nil
	}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return noopShutdown, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(tracerResource()),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func tracerResource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("chatwoot-megaapi-bridge"),
	)
}
