package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer          trace.Tracer
	meter           metric.Meter
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
)

func initTelemetry() error {
	ctx := context.Background()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("sample-app"),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("kubernetes"),
			semconv.K8SNodeName("meli-otel-test-control-plane"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracing
	// Use environment variables for endpoint configuration
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Initialize metrics
	// Use environment variables for endpoint configuration
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// Get tracer and meter
	tracer = otel.Tracer("sample-app", trace.WithInstrumentationVersion("1.0.0"))
	meter = otel.Meter("sample-app", metric.WithInstrumentationVersion("1.0.0"))

	// Create metrics
	requestCounter, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return fmt.Errorf("failed to create counter: %w", err)
	}

	requestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to create histogram: %w", err)
	}

	return nil
}

func simulateWork(ctx context.Context) {
	span := trace.SpanFromContext(ctx)

	// Simulate some work
	workDuration := time.Duration(rand.Intn(500)) * time.Millisecond
	time.Sleep(workDuration)

	span.SetAttributes(
		attribute.String("work.type", "processing"),
		attribute.Int("work.duration_ms", int(workDuration.Milliseconds())),
	)

	// Sometimes simulate an error
	if rand.Intn(10) == 0 {
		span.SetAttributes(attribute.Bool("error", true))
		log.Printf("Simulated error occurred")
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "health_check")
	defer span.End()

	start := time.Now()

	requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/health"),
		attribute.String("status", "200"),
	))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

	duration := time.Since(start).Seconds()
	requestDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/health"),
	))
}

func workHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "do_work")
	defer span.End()

	start := time.Now()

	// Add some attributes
	span.SetAttributes(
		attribute.String("user.id", "user-"+strconv.Itoa(rand.Intn(100))),
		attribute.String("request.id", fmt.Sprintf("req-%d", rand.Intn(10000))),
	)

	// Simulate nested work
	childCtx, childSpan := tracer.Start(ctx, "nested_operation")
	simulateWork(childCtx)
	childSpan.End()

	status := "200"
	if rand.Intn(20) == 0 { // 5% error rate
		status = "500"
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		span.SetAttributes(attribute.Bool("error", true))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Work completed successfully"))
	}

	requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/work"),
		attribute.String("status", status),
	))

	duration := time.Since(start).Seconds()
	requestDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/work"),
	))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "metrics")
	defer span.End()

	start := time.Now()

	// Generate some random metrics
	cpuUsage := rand.Float64() * 100
	memoryUsage := rand.Float64() * 1024 * 1024 * 1024 // GB

	span.SetAttributes(
		attribute.Float64("system.cpu.usage", cpuUsage),
		attribute.Float64("system.memory.usage", memoryUsage),
	)

	requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/metrics"),
		attribute.String("status", "200"),
	))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"cpu_usage": %.2f, "memory_usage": %.2f}`, cpuUsage, memoryUsage)

	duration := time.Since(start).Seconds()
	requestDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("endpoint", "/metrics"),
	))
}

func main() {
	if err := initTelemetry(); err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/work", workHandler)
	http.HandleFunc("/metrics", metricsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
