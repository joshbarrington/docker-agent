package root

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const AppName = "cagent"

// initOTelSDK initializes OpenTelemetry SDK with OTLP exporter
func initOTelSDK(ctx context.Context) (err error) {
	res, err := newOTelResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	var traceExporter trace.SpanExporter
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	// Only initialize if endpoint is configured
	if endpoint != "" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}
		if isLocalhostEndpoint(endpoint) {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		traceExporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("failed to create trace exporter: %w", err)
		}
	}

	// Configure tracer provider
	tracerProviderOpts := []trace.TracerProviderOption{
		trace.WithResource(res),
	}

	if traceExporter != nil {
		tracerProviderOpts = append(tracerProviderOpts,
			trace.WithBatcher(traceExporter,
				trace.WithBatchTimeout(5*time.Second),
				trace.WithMaxExportBatchSize(512),
			),
		)
	}

	tp := trace.NewTracerProvider(tracerProviderOpts...)
	otel.SetTracerProvider(tp)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
	}()

	return nil
}

func newOTelResource() (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(AppName),
			semconv.ServiceVersion("dev"), // TODO: use actual version
		),
	)
}

// isLocalhostEndpoint reports whether the given endpoint refers to a
// loopback address so that we can safely skip TLS.
func isLocalhostEndpoint(endpoint string) bool {
	host := endpoint
	// Strip port if present.
	if h, _, err := net.SplitHostPort(endpoint); err == nil {
		host = h
	}
	// Strip brackets from IPv6 addresses (e.g. "[::1]" without a port).
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
