package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check that response starts with "OK"
	if !strings.HasPrefix(rr.Body.String(), "OK") {
		t.Errorf("handler returned unexpected body: got %v, want it to start with 'OK'",
			rr.Body.String())
	}
}

func TestTestCrawlEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/test-crawl?url=https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
		})
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if ctype := rr.Header().Get("Content-Type"); ctype != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ctype, "application/json")
	}
}

func TestRateLimiter(t *testing.T) {
	// Create a new rate limiter
	limiter := newRateLimiter()

	// Mock request with X-Forwarded-For
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Forwarded-For", "192.168.1.1")

	// Test basic allowance - should allow up to burst capacity (10)
	for i := range 10 {
		ip := getClientIP(req1)
		rLimiter := limiter.getLimiter(ip)
		if !rLimiter.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// This should be blocked (11th request exceeds burst capacity)
	ip := getClientIP(req1)
	rLimiter := limiter.getLimiter(ip)
	if rLimiter.Allow() {
		t.Errorf("Request should be blocked after burst capacity exceeded")
	}

	// Different IP should be allowed
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Forwarded-For", "192.168.1.2")
	ip2 := getClientIP(req2)
	rLimiter2 := limiter.getLimiter(ip2)
	if !rLimiter2.Allow() {
		t.Errorf("Request from different IP should be allowed")
	}
}
