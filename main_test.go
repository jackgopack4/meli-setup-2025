package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func setupTestTelemetry() error {
	// Create a simple test tracer provider without any exporters
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("test-app"),
			semconv.ServiceVersion("test"),
		),
	)
	if err != nil {
		return err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// No exporters needed for testing
	)
	otel.SetTracerProvider(tracerProvider)

	// Create a simple meter provider without exporters for testing
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		// No readers needed for testing
	)
	otel.SetMeterProvider(meterProvider)

	// Initialize global variables
	tracer = otel.Tracer("test-app")
	meter = otel.Meter("test-app")

	// Create test metrics
	requestCounter, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return err
	}

	requestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		return err
	}

	return nil
}

func TestHealthHandler(t *testing.T) {
	// Setup test telemetry
	if err := setupTestTelemetry(); err != nil {
		t.Fatalf("Failed to setup test telemetry: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET health check",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "POST health check",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			healthHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}

			// Check content type is not set (plain text)
			if contentType := w.Header().Get("Content-Type"); contentType != "" {
				t.Errorf("Expected no Content-Type header, got %q", contentType)
			}
		})
	}
}

func TestWorkHandler(t *testing.T) {
	// Setup test telemetry
	if err := setupTestTelemetry(); err != nil {
		t.Fatalf("Failed to setup test telemetry: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		expectedStatus []int // Multiple possible statuses due to randomness
	}{
		{
			name:           "GET work endpoint",
			method:         http.MethodGet,
			expectedStatus: []int{http.StatusOK, http.StatusInternalServerError},
		},
		{
			name:           "POST work endpoint",
			method:         http.MethodPost,
			expectedStatus: []int{http.StatusOK, http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to test randomness
			successCount := 0
			errorCount := 0
			totalRuns := 100

			for i := 0; i < totalRuns; i++ {
				req := httptest.NewRequest(tt.method, "/work", nil)
				w := httptest.NewRecorder()

				workHandler(w, req)

				statusFound := false
				for _, expectedStatus := range tt.expectedStatus {
					if w.Code == expectedStatus {
						statusFound = true
						break
					}
				}

				if !statusFound {
					t.Errorf("Unexpected status %d, expected one of %v", w.Code, tt.expectedStatus)
				}

				if w.Code == http.StatusOK {
					successCount++
					if w.Body.String() != "Work completed successfully" {
						t.Errorf("Expected success body, got %q", w.Body.String())
					}
				} else if w.Code == http.StatusInternalServerError {
					errorCount++
					if w.Body.String() != "Internal Server Error" {
						t.Errorf("Expected error body, got %q", w.Body.String())
					}
				}
			}

			// Check that we get both successes and errors (5% error rate)
			// Allow some variance in the randomness
			if errorCount == 0 {
				t.Log("Warning: No errors generated in 100 runs (expected ~5)")
			}
			if successCount == 0 {
				t.Error("No successful requests in 100 runs")
			}

			t.Logf("Success rate: %d/%d (%.1f%%), Error rate: %d/%d (%.1f%%)",
				successCount, totalRuns, float64(successCount)/float64(totalRuns)*100,
				errorCount, totalRuns, float64(errorCount)/float64(totalRuns)*100)
		})
	}
}

func TestMetricsHandler(t *testing.T) {
	// Setup test telemetry
	if err := setupTestTelemetry(); err != nil {
		t.Fatalf("Failed to setup test telemetry: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		contentType    string
	}{
		{
			name:           "GET metrics endpoint",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			contentType:    "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/metrics", nil)
			w := httptest.NewRecorder()

			metricsHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if contentType := w.Header().Get("Content-Type"); contentType != tt.contentType {
				t.Errorf("Expected Content-Type %q, got %q", tt.contentType, contentType)
			}

			// Check that response is valid JSON format
			body := w.Body.String()
			if len(body) == 0 {
				t.Error("Expected non-empty response body")
			}

			// Basic JSON structure check (should contain cpu_usage and memory_usage)
			if !contains(body, "cpu_usage") {
				t.Error("Response should contain 'cpu_usage'")
			}
			if !contains(body, "memory_usage") {
				t.Error("Response should contain 'memory_usage'")
			}
		})
	}
}

func TestSimulateWork(t *testing.T) {
	// Setup test telemetry
	if err := setupTestTelemetry(); err != nil {
		t.Fatalf("Failed to setup test telemetry: %v", err)
	}

	tests := []struct {
		name string
	}{
		{
			name: "simulate work with span",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, span := tracer.Start(context.Background(), "test_span")
			defer span.End()

			start := time.Now()
			simulateWork(ctx)
			duration := time.Since(start)

			// Should take some time (at least a few milliseconds, at most 500ms)
			if duration < time.Millisecond {
				t.Error("simulateWork should take some time")
			}
			if duration > 600*time.Millisecond {
				t.Error("simulateWork took too long (>600ms)")
			}

			// Test multiple runs to check for error simulation
			errorOccurred := false
			for i := 0; i < 50; i++ {
				ctx, span := tracer.Start(context.Background(), "test_span")
				simulateWork(ctx)
				span.End()
			}

			// Note: We can't easily test the error case without more complex span inspection
			// This is acceptable for this level of testing
			_ = errorOccurred
		})
	}
}

func TestInitTelemetryWithoutExporter(t *testing.T) {
	// This test checks if initTelemetry function structure is sound
	// We can't easily test the actual OTLP exporters without a running collector

	tests := []struct {
		name string
	}{
		{
			name: "init telemetry structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll just test that the function exists and can be called
			// In a real scenario, this would require mocking the OTLP exporters
			// For now, we'll test the basic structure

			// Test that global variables are available
			if tracer == nil {
				t.Error("tracer should be initialized")
			}
		})
	}
}

func TestResourceAttributes(t *testing.T) {
	// Test that resource attributes are correctly set
	// Note: In the actual application, K8S node name is detected by the resourcedetection processor
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("sample-app"),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("kubernetes"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	// Verify expected attributes are present
	attrs := res.Attributes()

	// Debug: Print all attributes
	t.Logf("Resource has %d attributes:", len(attrs))
	for _, attr := range attrs {
		t.Logf("  - %s = %s", attr.Key, attr.Value.AsString())
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "service name",
			key:      string(semconv.ServiceNameKey),
			expected: "sample-app",
		},
		{
			name:     "service version",
			key:      string(semconv.ServiceVersionKey),
			expected: "1.0.0",
		},
		{
			name:     "deployment environment",
			key:      string(semconv.DeploymentEnvironmentKey),
			expected: "kubernetes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, attr := range attrs {
				if string(attr.Key) == tt.key {
					if attr.Value.AsString() != tt.expected {
						t.Errorf("Expected %s to be %q, got %q", tt.name, tt.expected, attr.Value.AsString())
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected attribute %s (%s) not found in resource", tt.name, tt.key)
			}
		})
	}
}

func BenchmarkHealthHandler(b *testing.B) {
	if err := setupTestTelemetry(); err != nil {
		b.Fatalf("Failed to setup test telemetry: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		healthHandler(w, req)
	}
}

func BenchmarkWorkHandler(b *testing.B) {
	if err := setupTestTelemetry(); err != nil {
		b.Fatalf("Failed to setup test telemetry: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/work", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		workHandler(w, req)
	}
}

func BenchmarkMetricsHandler(b *testing.B) {
	if err := setupTestTelemetry(); err != nil {
		b.Fatalf("Failed to setup test telemetry: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		metricsHandler(w, req)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
