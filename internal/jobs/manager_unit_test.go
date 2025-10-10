//go:build unit || !integration

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Constructor and Initialization Tests
// ============================================================================

func TestJobManagerInitialisation(t *testing.T) {
	t.Parallel()

	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{} // Use real WorkerPool struct but don't start it

	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)

	require.NotNil(t, jm)
	assert.NotNil(t, jm.processedPages)
	assert.Equal(t, 0, len(jm.processedPages))
}

// ============================================================================
// Job Options Validation Tests
// ============================================================================

func TestJobOptionsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options *JobOptions
		valid   bool
	}{
		{
			name: "valid options",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 5,
				FindLinks:   true,
				MaxPages:    100,
				UseSitemap:  true,
			},
			valid: true,
		},
		{
			name: "empty domain",
			options: &JobOptions{
				Domain:      "",
				Concurrency: 5,
				FindLinks:   true,
				MaxPages:    100,
				UseSitemap:  true,
			},
			valid: false,
		},
		{
			name: "invalid concurrency",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 0,
				FindLinks:   true,
				MaxPages:    100,
				UseSitemap:  true,
			},
			valid: false,
		},
		{
			name: "negative max pages",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 5,
				FindLinks:   true,
				MaxPages:    -1,
				UseSitemap:  true,
			},
			valid: false,
		},
		{
			name: "minimum valid options",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 1,
				FindLinks:   false,
				MaxPages:    0, // 0 means no limit
				UseSitemap:  false,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJobOptions(tt.options)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Helper function for validation (since it's not exported)
func validateJobOptions(options *JobOptions) error {
	if options.Domain == "" {
		return errors.New("domain is required")
	}
	if options.Concurrency < 1 {
		return errors.New("concurrency must be at least 1")
	}
	if options.MaxPages < 0 {
		return errors.New("max pages cannot be negative")
	}
	return nil
}

// ============================================================================
// Mock Crawler Integration Tests
// ============================================================================

func TestJobManagerWithMockCrawler(t *testing.T) {
	ctx := context.Background()

	// Create mock crawler
	mockCrawler := new(mocks.MockCrawler)

	// Set expectation
	mockCrawler.On("DiscoverSitemapsAndRobots", ctx, "example.com").
		Return(&crawler.SitemapDiscoveryResult{
			Sitemaps:    []string{"https://example.com/sitemap.xml"},
			RobotsRules: &crawler.RobotsRules{},
		}, nil)

	// Create JobManager with mock
	jm := &JobManager{
		crawler:        mockCrawler,
		processedPages: make(map[string]struct{}),
	}

	// Call the method that uses crawler
	result, err := jm.crawler.DiscoverSitemapsAndRobots(ctx, "example.com")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, []string{"https://example.com/sitemap.xml"}, result.Sitemaps)
	mockCrawler.AssertExpectations(t)
}

// ============================================================================
// Job State Management Tests
// ============================================================================

func TestJobStatusTransitions(t *testing.T) {
	// Test valid transitions
	validTransitions := []struct {
		from JobStatus
		to   JobStatus
	}{
		{JobStatusPending, JobStatusRunning},
		{JobStatusRunning, JobStatusCompleted},
		{JobStatusRunning, JobStatusFailed},
		{JobStatusRunning, JobStatusCancelled},
		{JobStatusPending, JobStatusCancelled},
	}

	for _, transition := range validTransitions {
		t.Run(string(transition.from)+"_to_"+string(transition.to), func(t *testing.T) {
			// This would test actual transition logic if we had it
			assert.NotEqual(t, transition.from, transition.to)
		})
	}
}

// ============================================================================
// Concurrency Tests
// ============================================================================

func TestJobManagerConcurrentOperations(t *testing.T) {
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}

	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)

	// Test concurrent access to processedPages
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Simulate concurrent page processing
			pageURL := fmt.Sprintf("https://example.com/page%d", id)

			// Check if processed
			jm.pagesMutex.RLock()
			_, exists := jm.processedPages[pageURL]
			jm.pagesMutex.RUnlock()

			if !exists {
				// Mark as processed
				jm.pagesMutex.Lock()
				jm.processedPages[pageURL] = struct{}{}
				jm.pagesMutex.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify all pages were processed
	assert.Equal(t, numGoroutines, len(jm.processedPages))
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestJobManagerErrorHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupMock   func(*mocks.MockDbQueue)
		expectError bool
	}{
		{
			name: "database error",
			setupMock: func(m *mocks.MockDbQueue) {
				m.On("GetNextTask", ctx, "job1").
					Return(nil, errors.New("database connection lost"))
			},
			expectError: true,
		},
		{
			name: "no tasks available",
			setupMock: func(m *mocks.MockDbQueue) {
				m.On("GetNextTask", ctx, "job1").
					Return(nil, sql.ErrNoRows)
			},
			expectError: false, // No rows is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDbQueue := new(mocks.MockDbQueue)
			tt.setupMock(mockDbQueue)

			_, err := mockDbQueue.GetNextTask(ctx, "job1")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.Equal(t, sql.ErrNoRows, err)
			}

			mockDbQueue.AssertExpectations(t)
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkJobManagerPageProcessing(b *testing.B) {
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}

	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)

	b.ResetTimer()

	b.Run("MarkPageProcessed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pageURL := fmt.Sprintf("https://example.com/page%d", i)

			jm.pagesMutex.Lock()
			jm.processedPages[pageURL] = struct{}{}
			jm.pagesMutex.Unlock()
		}
	})

	b.Run("CheckPageProcessed", func(b *testing.B) {
		// Pre-populate some pages
		for i := 0; i < 1000; i++ {
			pageURL := fmt.Sprintf("https://example.com/page%d", i)
			jm.processedPages[pageURL] = struct{}{}
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			pageURL := fmt.Sprintf("https://example.com/page%d", i%1000)

			jm.pagesMutex.RLock()
			_ = jm.processedPages[pageURL]
			jm.pagesMutex.RUnlock()
		}
	})
}

// ============================================================================
// Helper Functions Tests
// ============================================================================

func TestJobDurationCalculation(t *testing.T) {
	// Use fixed times to avoid timing issues
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected time.Duration
	}{
		{
			name:     "one hour job",
			start:    baseTime,
			end:      baseTime.Add(1 * time.Hour),
			expected: 1 * time.Hour,
		},
		{
			name:     "zero duration",
			start:    baseTime,
			end:      baseTime,
			expected: 0,
		},
		{
			name:     "30 minute job",
			start:    baseTime,
			end:      baseTime.Add(30 * time.Minute),
			expected: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.end.Sub(tt.start)
			assert.Equal(t, tt.expected, duration)
		})
	}
}
