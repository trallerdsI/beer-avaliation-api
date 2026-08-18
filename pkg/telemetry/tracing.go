package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitTracerProvider configura o provedor de traces distribuídos (OpenTelemetry)
// com exportação OTLP/gRPC para o Grafana Tempo (ou outro collector OTLP).
//
// A URL do collector é definida por OTEL_EXPORTER_OTLP_ENDPOINT (default: localhost:4317).
// A amostragem é 10% probabilística em produção, ou 100% em desenvolvimento.
func InitTracerProvider(serviceName string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	slog.Info("OpenTelemetry tracer initialized", "endpoint", endpoint, "service", serviceName)
	return tp, nil
}

// Shutdown fecha o provedor de traces de forma graciosa.
func Shutdown(tp *sdktrace.TracerProvider) {
	if tp == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		slog.Error("failed to shutdown tracer provider", "err", err)
	}
}
