package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWarmURL(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("CF-Cache-Status", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer ts.Close()

	crawler := New(nil)
	result, err := crawler.WarmURL(context.Background(), ts.URL, false)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}

	if result.CacheStatus != "HIT" {
		t.Errorf("Expected cache status HIT, got %s", result.CacheStatus)
	}

	// Check that performance metrics are captured
	if result.Performance.TTFB == 0 {
		t.Log("Warning: TTFB not captured (may be too fast for local test)")
	}
	if result.Performance.TCPConnectionTime == 0 {
		t.Log("Warning: TCP connection time not captured (may be reused connection)")
	}
}

func TestPerformanceMetrics(t *testing.T) {
	// Create a test server with a small delay to ensure metrics are captured
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)        // Small delay to ensure measurable times
		w.Header().Set("CF-Cache-Status", "HIT") // Use HIT to avoid cache warming loop
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Performance test response"))
	}))
	defer ts.Close()

	crawler := New(nil)
	result, err := crawler.WarmURL(context.Background(), ts.URL, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Log all performance metrics
	t.Logf("Performance Metrics:")
	t.Logf("  DNS Lookup: %dms", result.Performance.DNSLookupTime)
	t.Logf("  TCP Connection: %dms", result.Performance.TCPConnectionTime)
	t.Logf("  TLS Handshake: %dms", result.Performance.TLSHandshakeTime)
	t.Logf("  TTFB: %dms", result.Performance.TTFB)
	t.Logf("  Content Transfer: %dms", result.Performance.ContentTransferTime)
	t.Logf("  Total Response Time: %dms", result.ResponseTime)

	// Verify that at least TTFB is captured (should always be > 0 with delay)
	if result.Performance.TTFB == 0 {
		t.Error("TTFB should be greater than 0")
	}

	// Verify total response time is reasonable
	if result.ResponseTime < 10 {
		t.Error("Response time should be at least 10ms due to server delay")
	}
}

func TestPerformanceMetricsWithRealURL(t *testing.T) {
	// Skip in CI or if no internet connection
	if testing.Short() {
		t.Skip("Skipping test that requires internet connection")
	}

	// Use a real HTTPS URL to test DNS, TCP, and TLS metrics
	crawler := New(nil)
	result, err := crawler.WarmURL(context.Background(), "https://httpbin.org/status/200", false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Log all performance metrics
	t.Logf("Performance Metrics for HTTPS request:")
	t.Logf("  DNS Lookup: %dms", result.Performance.DNSLookupTime)
	t.Logf("  TCP Connection: %dms", result.Performance.TCPConnectionTime)
	t.Logf("  TLS Handshake: %dms", result.Performance.TLSHandshakeTime)
	t.Logf("  TTFB: %dms", result.Performance.TTFB)
	t.Logf("  Content Transfer: %dms", result.Performance.ContentTransferTime)
	t.Logf("  Total Response Time: %dms", result.ResponseTime)

	// For a real HTTPS request, we should capture at least some of these
	if result.Performance.DNSLookupTime == 0 &&
		result.Performance.TCPConnectionTime == 0 &&
		result.Performance.TLSHandshakeTime == 0 {
		t.Log("Warning: No connection metrics captured - connection might be reused")
	}

	// TTFB should always be captured
	if result.Performance.TTFB == 0 {
		t.Error("TTFB should be greater than 0 for real request")
	}
}

func TestWarmURLError(t *testing.T) {
	crawler := New(nil)
	// Use a malformed URL instead
	result, err := crawler.WarmURL(context.Background(), "not-a-valid-url", false)

	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	if result.Error == "" {
		t.Error("Expected error message in result, got empty string")
	}
}

func TestWarmURLWithDifferentStatuses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
	}{
		{"success", http.StatusOK, false},
		{"not found", http.StatusNotFound, true},
		{"server error", http.StatusInternalServerError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer ts.Close()

			crawler := New(nil)
			result, err := crawler.WarmURL(context.Background(), ts.URL, false)

			if (err != nil) != tt.wantError {
				t.Errorf("WarmURL() error = %v, wantError %v", err, tt.wantError)
			}
			if result.StatusCode != tt.statusCode {
				t.Errorf("WarmURL() status = %v, want %v", result.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestWarmURLContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	crawler := New(nil)

	// Create a test server that delays
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Cancel context immediately
	cancel()

	// Should fail due to cancelled context
	_, err := crawler.WarmURL(ctx, ts.URL, false)
	if err == nil {
		t.Error("Expected error due to cancelled context, got nil")
	}
}

func TestWarmURLWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	crawler := New(nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := crawler.WarmURL(ctx, ts.URL, false)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
