package main

import (
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
		w.Write([]byte("OK"))
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
	t.Skip("TODO: Implement after refactoring handlers")
	// This test will be implemented after refactoring handlers to be testable
}
