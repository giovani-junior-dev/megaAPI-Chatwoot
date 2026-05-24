package bridge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInitTracer_NoEndpoint_NoOp(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := InitTracer(context.Background())
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown(context.Background()))
	_, span := otel.Tracer("test").Start(context.Background(), "noop")
	span.End()
}

func TestInitTracer_WithEndpoint_RegistersSDKProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
	shutdown, err := InitTracer(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	_, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	require.True(t, ok, "tracer provider should be SDK-backed when endpoint set")
}
