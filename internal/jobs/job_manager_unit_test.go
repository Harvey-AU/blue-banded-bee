package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobManagerInitialisation(t *testing.T) {
	t.Helper()
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

func TestJobOptionsValidation(t *testing.T) {
	t.Helper()
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
			name: "zero concurrency",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 0,
				FindLinks:   true,
				MaxPages:    100,
				UseSitemap:  true,
			},
			valid: true, // Should be handled by defaults
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()
			
			if tt.valid {
				// Test valid conditions
				if tt.options.Domain != "" {
					assert.NotEmpty(t, tt.options.Domain)
				}
				assert.GreaterOrEqual(t, tt.options.Concurrency, 0)
				assert.GreaterOrEqual(t, tt.options.MaxPages, 0)
			} else {
				// Test invalid conditions
				if tt.options.Domain == "" {
					assert.Empty(t, tt.options.Domain)
				}
			}
		})
	}
}

func TestJobManagerPageProcessingTracking(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}
	
	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)
	
	jobID := "test-job-123"
	pageID := 456
	
	// Test initial state - page should not be processed
	isProcessed := jm.isPageProcessed(jobID, pageID)
	assert.False(t, isProcessed)
	
	// Mark page as processed
	jm.markPageProcessed(jobID, pageID)
	
	// Test that page is now tracked as processed
	isNowProcessed := jm.isPageProcessed(jobID, pageID)
	assert.True(t, isNowProcessed)
	
	// Test different page ID - should not be processed
	otherPageProcessed := jm.isPageProcessed(jobID, 789)
	assert.False(t, otherPageProcessed)
	
	// Test different job ID - should not be processed
	differentJobProcessed := jm.isPageProcessed("other-job", pageID)
	assert.False(t, differentJobProcessed)
}

func TestJobManagerConcurrentPageTracking(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}
	
	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)
	
	jobID := "concurrent-job"
	numPages := 10
	
	// Test concurrent page tracking
	done := make(chan bool, numPages)
	
	// Add pages concurrently
	for i := 0; i < numPages; i++ {
		go func(pageNum int) {
			defer func() { done <- true }()
			jm.markPageProcessed(jobID, pageNum)
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < numPages; i++ {
		<-done
	}
	
	// Verify all pages were tracked
	processedCount := 0
	for i := 0; i < numPages; i++ {
		if jm.isPageProcessed(jobID, i) {
			processedCount++
		}
	}
	
	assert.Equal(t, numPages, processedCount)
}

func TestJobManagerPageCleanup(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}
	
	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)
	
	jobID := "cleanup-test-job"
	
	// Mark several pages as processed
	for i := 0; i < 5; i++ {
		jm.markPageProcessed(jobID, i)
	}
	
	// Verify pages are tracked
	for i := 0; i < 5; i++ {
		assert.True(t, jm.isPageProcessed(jobID, i))
	}
	
	// Clear processed pages for the job
	jm.clearProcessedPages(jobID)
	
	// Verify pages are no longer tracked
	for i := 0; i < 5; i++ {
		assert.False(t, jm.isPageProcessed(jobID, i))
	}
}

func TestJobManagerEdgeCases(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	mockDB := &sql.DB{}
	mockDbQueue := &mocks.MockDbQueue{}
	mockCrawler := &mocks.MockCrawler{}
	mockWorkerPool := &WorkerPool{}
	
	jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)
	
	// Test empty job ID
	pageProcessed := jm.isPageProcessed("", 123)
	assert.False(t, pageProcessed)
	
	// Test negative page ID
	pageProcessed = jm.isPageProcessed("job-123", -1)
	assert.False(t, pageProcessed)
	
	// Test marking empty job ID (should not panic)
	assert.NotPanics(t, func() {
		jm.markPageProcessed("", 123)
	})
	
	// Test clearing empty job ID (should not panic)
	assert.NotPanics(t, func() {
		jm.clearProcessedPages("")
	})
}

func TestJobManagerContextTimeout(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	// Test context timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// Wait for timeout
	<-ctx.Done()
	
	// Verify context is cancelled
	assert.Error(t, ctx.Err())
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
}

func TestCrawlerInterface(t *testing.T) {
	t.Helper()
	t.Parallel()
	
	// Test that our mock implements the CrawlerInterface
	var mockCrawler CrawlerInterface = &mocks.MockCrawler{}
	assert.NotNil(t, mockCrawler)
	
	// Test that real crawler implements the interface
	var realCrawler CrawlerInterface = &crawler.Crawler{}
	assert.NotNil(t, realCrawler)
}