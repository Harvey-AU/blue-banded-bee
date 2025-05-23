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
