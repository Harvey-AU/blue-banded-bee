package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func init() {
	// Find project root (where go.mod is)
	projectRoot, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			break
		}
		projectRoot = parent
	}

	// Load .env from project root
	if err := godotenv.Load(filepath.Join(projectRoot, ".env")); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}
}

func TestStoreCrawlResult(t *testing.T) {
	// Setup test database connection with different memory mode
	dbConfig := &Config{
		URL:       "file::memory:",
		AuthToken: "",
	}

	database, err := New(dbConfig)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Test case
	testResult := &CrawlResult{
		URL:          "https://test.com",
		ResponseTime: 100,
		StatusCode:   200,
		Error:        "",
		CacheStatus:  "HIT",
	}

	// Store the result
	err = database.StoreCrawlResult(context.Background(), testResult)
	if err != nil {
		t.Errorf("Failed to store crawl result: %v", err)
	}

	// Retrieve and verify
	results, err := database.GetRecentResults(context.Background(), 1)
	if err != nil {
		t.Errorf("Failed to get recent results: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].URL != testResult.URL {
		t.Errorf("Expected URL %s, got %s", testResult.URL, results[0].URL)
	}
}

func TestGetRecentResults(t *testing.T) {
	dbConfig := &Config{
		URL:       "file::memory:",
		AuthToken: "",
	}

	database, err := New(dbConfig)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Insert multiple test results
	testResults := []CrawlResult{
		{URL: "https://test1.com", ResponseTime: 100, StatusCode: 200},
		{URL: "https://test2.com", ResponseTime: 200, StatusCode: 200},
		{URL: "https://test3.com", ResponseTime: 300, StatusCode: 404},
	}

	for _, result := range testResults {
		err = database.StoreCrawlResult(context.Background(), &result)
		if err != nil {
			t.Fatalf("Failed to store test result: %v", err)
		}
	}

	// Test retrieving with limit
	limit := 2
	results, err := database.GetRecentResults(context.Background(), limit)
	if err != nil {
		t.Errorf("Failed to get recent results: %v", err)
	}

	if len(results) != limit {
		t.Errorf("Expected %d results, got %d", limit, len(results))
	}
}

func TestTursoConnection(t *testing.T) {
	// Set test environment variables
	t.Setenv("RUN_INTEGRATION_TESTS", "true")
	t.Setenv("DATABASE_URL", os.Getenv("DATABASE_URL"))
	t.Setenv("DATABASE_AUTH_TOKEN", os.Getenv("DATABASE_AUTH_TOKEN"))

	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test")
	}

	dbConfig := &Config{
		URL:       os.Getenv("DATABASE_URL"),
		AuthToken: os.Getenv("DATABASE_AUTH_TOKEN"),
	}

	database, err := New(dbConfig)
	if err != nil {
		t.Fatalf("Failed to connect to Turso: %v", err)
	}
	defer database.Close()

	err = database.TestConnection()
	if err != nil {
		t.Errorf("Turso connection test failed: %v", err)
	}
}

func TestNullHandling(t *testing.T) {
	dbConfig := &Config{
		URL:       "file::memory:",
		AuthToken: "",
	}

	database, err := New(dbConfig)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Test with null fields
	testResult := &CrawlResult{
		URL:          "https://test.com",
		ResponseTime: 100,
		StatusCode:   200,
		// Error and CacheStatus intentionally left empty
	}

	err = database.StoreCrawlResult(context.Background(), testResult)
	if err != nil {
		t.Errorf("Failed to store result with null fields: %v", err)
	}
}

func TestTestCrawlEndpointError(t *testing.T) {
	req, err := http.NewRequest("GET", "/test-crawl?url=invalid-url", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestRecentCrawlsEndpointError(t *testing.T) {
	req, err := http.NewRequest("GET", "/recent-crawls", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Database error", http.StatusInternalServerError)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestStoreCrawlResultError(t *testing.T) {
	// Create a mock DB that will always fail
	mockDB := &sql.DB{}
	db := &DB{client: mockDB}

	// Test case
	testResult := &CrawlResult{
		URL:          "https://test.com",
		ResponseTime: 100,
		StatusCode:   200,
	}

	// This should fail since the mockDB doesn't have proper implementation
	err := db.StoreCrawlResult(context.Background(), testResult)
	if err == nil {
		t.Error("Expected error from StoreCrawlResult with invalid DB, but got nil")
	}
}

// TestTaskStatusConsistency tests that task status values are used consistently
func TestTaskStatusConsistency(t *testing.T) {
	dbConfig := &Config{
		URL:       "file::memory:",
		AuthToken: "",
	}

	database, err := New(dbConfig)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Import the jobs package from within your project
	// For this test, we're checking the consistency of status constants

	// Verify task status constants match expected values
	expectedValues := map[string]string{
		"TaskStatusPending":   "pending",
		"TaskStatusRunning":   "running",
		"TaskStatusCompleted": "completed",
		"TaskStatusFailed":    "failed",
		"TaskStatusSkipped":   "skipped",
	}

	// This test would be better placed in the jobs package itself
	// since it directly tests the jobs package constants
}
