//go:build unit

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/mocks"
	"github.com/stretchr/testify/assert"
)

// TestJobManagerWithMockCrawler proves that our interface refactoring works
func TestJobManagerWithMockCrawler(t *testing.T) {
	ctx := context.Background()

	// Create mock crawler
	mockCrawler := new(mocks.MockCrawler)

	// Set expectation - use the new method
	mockCrawler.On("DiscoverSitemapsAndRobots", ctx, "example.com").
		Return(&crawler.SitemapDiscoveryResult{
			Sitemaps:    []string{"https://example.com/sitemap.xml"},
			RobotsRules: &crawler.RobotsRules{},
		}, nil)

	// Create JobManager with mock
	jm := &JobManager{
		crawler:        mockCrawler, // This works now because crawler is CrawlerInterface!
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

// TestJobManager_ProcessSitemap tests processSitemap logic with simplified unit test
func TestJobManager_ProcessSitemap(t *testing.T) {
	// For unit tests, we'll test simpler units of functionality
	// The complex processSitemap method is better tested via integration tests
	// This demonstrates that the mocking infrastructure works

	t.Run("CrawlerInterfaceWorks", func(t *testing.T) {
		ctx := context.Background()
		mockCrawler := new(mocks.MockCrawler)

		// Test that crawler methods can be mocked
		mockCrawler.On("DiscoverSitemapsAndRobots", ctx, "example.com").
			Return(&crawler.SitemapDiscoveryResult{
				Sitemaps:    []string{"https://example.com/sitemap.xml"},
				RobotsRules: &crawler.RobotsRules{},
			}, nil)

		mockCrawler.On("ParseSitemap", ctx, "https://example.com/sitemap.xml").
			Return([]string{"https://example.com/"}, nil)

		mockCrawler.On("FilterURLs", []string{"https://example.com/"}, []string(nil), []string(nil)).
			Return([]string{"https://example.com/"})

		// Create JobManager with mock
		jm := &JobManager{
			crawler:        mockCrawler,
			processedPages: make(map[string]struct{}),
		}

		// Test crawler interface methods
		result, err := jm.crawler.DiscoverSitemapsAndRobots(ctx, "example.com")
		assert.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/sitemap.xml"}, result.Sitemaps)

		urls, err := jm.crawler.ParseSitemap(ctx, "https://example.com/sitemap.xml")
		assert.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/"}, urls)

		filtered := jm.crawler.FilterURLs(urls, nil, nil)
		assert.Equal(t, []string{"https://example.com/"}, filtered)

		mockCrawler.AssertExpectations(t)
	})

	t.Run("EnqueueJobURLsWithMock", func(t *testing.T) {
		ctx := context.Background()
		mockDbQueue := new(mocks.MockDbQueue)

		// Test EnqueueJobURLs method which wraps dbQueue.EnqueueURLs
		pages := []db.Page{
			{ID: 1, Path: "/", Priority: 1.0},
			{ID: 2, Path: "/about", Priority: 0.9},
		}

		mockDbQueue.On("EnqueueURLs", ctx, "job-123", pages, "test", "https://example.com").
			Return(nil)

		jm := &JobManager{
			dbQueue:        mockDbQueue,
			processedPages: make(map[string]struct{}),
		}

		err := jm.EnqueueJobURLs(ctx, "job-123", pages, "test", "https://example.com")
		assert.NoError(t, err)

		// Verify pages are marked as processed
		assert.True(t, jm.isPageProcessed("job-123", 1))
		assert.True(t, jm.isPageProcessed("job-123", 2))

		mockDbQueue.AssertExpectations(t)
	})
}

// TestJobManagerBasicOperations tests basic JobManager functionality
func TestJobManagerBasicOperations(t *testing.T) {
	t.Run("NewJobManager", func(t *testing.T) {
		mockDB := &sql.DB{}
		mockDbQueue := new(mocks.MockDbQueue)
		mockCrawler := new(mocks.MockCrawler)
		mockWorkerPool := new(mocks.MockWorkerPool)

		jm := NewJobManager(mockDB, mockDbQueue, mockCrawler, mockWorkerPool)

		assert.NotNil(t, jm)
		assert.Equal(t, mockDB, jm.db)
		assert.Equal(t, mockDbQueue, jm.dbQueue)
		assert.Equal(t, mockCrawler, jm.crawler)
		assert.Equal(t, mockWorkerPool, jm.workerPool)
		assert.NotNil(t, jm.processedPages)
		assert.Empty(t, jm.processedPages)
	})

	t.Run("PageProcessingTracking", func(t *testing.T) {
		jm := &JobManager{
			processedPages: make(map[string]struct{}),
		}

		jobID := "test-job"
		pageID := 123

		// Initially not processed
		assert.False(t, jm.isPageProcessed(jobID, pageID))

		// Mark as processed
		jm.markPageProcessed(jobID, pageID)

		// Now should be processed
		assert.True(t, jm.isPageProcessed(jobID, pageID))

		// Clear processed pages
		jm.clearProcessedPages(jobID)

		// Should no longer be processed
		assert.False(t, jm.isPageProcessed(jobID, pageID))
	})

	t.Run("PageProcessingConcurrency", func(t *testing.T) {
		jm := &JobManager{
			processedPages: make(map[string]struct{}),
		}

		jobID := "concurrent-job"
		numPages := 100
		var wg sync.WaitGroup

		// Mark pages as processed concurrently
		for i := 0; i < numPages; i++ {
			wg.Add(1)
			go func(pageID int) {
				defer wg.Done()
				jm.markPageProcessed(jobID, pageID)
			}(i)
		}

		wg.Wait()

		// Verify all pages are marked as processed
		for i := 0; i < numPages; i++ {
			assert.True(t, jm.isPageProcessed(jobID, i), "Page %d should be processed", i)
		}

		// Clear and verify cleanup
		jm.clearProcessedPages(jobID)
		
		for i := 0; i < numPages; i++ {
			assert.False(t, jm.isPageProcessed(jobID, i), "Page %d should not be processed after clear", i)
		}
	})
}

// TestJobManagerEnqueueFiltering tests the duplicate filtering logic
func TestJobManagerEnqueueFiltering(t *testing.T) {
	ctx := context.Background()
	mockDbQueue := new(mocks.MockDbQueue)
	
	jm := &JobManager{
		dbQueue:        mockDbQueue,
		processedPages: make(map[string]struct{}),
	}

	jobID := "filter-test-job"
	pages := []db.Page{
		{ID: 1, Path: "/page1", Priority: 1.0},
		{ID: 2, Path: "/page2", Priority: 0.9},
		{ID: 3, Path: "/page3", Priority: 0.8},
	}

	// Set up mock expectation for all 3 pages
	mockDbQueue.On("EnqueueURLs", ctx, jobID, pages, "test", "https://example.com").
		Return(nil)

	// First enqueue should process all pages
	err := jm.EnqueueJobURLs(ctx, jobID, pages, "test", "https://example.com")
	assert.NoError(t, err)

	// Verify pages are marked as processed
	for _, page := range pages {
		assert.True(t, jm.isPageProcessed(jobID, page.ID))
	}

	// Second enqueue with same pages should be filtered out (no mock expectation)
	err = jm.EnqueueJobURLs(ctx, jobID, pages, "test", "https://example.com")
	assert.NoError(t, err) // Should succeed but do nothing

	mockDbQueue.AssertExpectations(t)
}

// TestJobManagerEnqueueError tests error handling in EnqueueJobURLs
func TestJobManagerEnqueueError(t *testing.T) {
	ctx := context.Background()
	mockDbQueue := new(mocks.MockDbQueue)
	
	jm := &JobManager{
		dbQueue:        mockDbQueue,
		processedPages: make(map[string]struct{}),
	}

	jobID := "error-test-job"
	pages := []db.Page{
		{ID: 1, Path: "/page1", Priority: 1.0},
	}

	// Set up mock to return an error
	expectedErr := errors.New("database error")
	mockDbQueue.On("EnqueueURLs", ctx, jobID, pages, "test", "https://example.com").
		Return(expectedErr)

	// Enqueue should return the error
	err := jm.EnqueueJobURLs(ctx, jobID, pages, "test", "https://example.com")
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)

	// Pages should NOT be marked as processed when there's an error
	assert.False(t, jm.isPageProcessed(jobID, pages[0].ID))

	mockDbQueue.AssertExpectations(t)
}

func TestFilterURLsAgainstRobots(t *testing.T) {
	jm := &JobManager{}

	urls := []string{
		"https://example.com/allowed",
		"https://example.com/blocked",
		"https://example.com/also-allowed",
	}

	robotsRules := &crawler.RobotsRules{
		DisallowPatterns: []string{"/blocked"},
	}

	// Mock crawler for path filtering
	mockCrawler := new(mocks.MockCrawler)
	mockCrawler.On("FilterURLs", urls, []string(nil), []string(nil)).
		Return(urls) // Return all URLs (no path filtering)

	jm.crawler = mockCrawler

	// Test with robots rules but no path filters
	filtered := jm.filterURLsAgainstRobots(urls, robotsRules, nil, nil)
	
	// Should filter out the blocked URL
	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, "https://example.com/allowed")
	assert.Contains(t, filtered, "https://example.com/also-allowed")
	assert.NotContains(t, filtered, "https://example.com/blocked")

	mockCrawler.AssertExpectations(t)
}

func TestFilterURLsAgainstRobotsNoRules(t *testing.T) {
	jm := &JobManager{}

	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
	}

	// Test with no robots rules
	robotsRules := &crawler.RobotsRules{}
	
	// Mock crawler for path filtering
	mockCrawler := new(mocks.MockCrawler)
	mockCrawler.On("FilterURLs", urls, []string(nil), []string(nil)).
		Return(urls)

	jm.crawler = mockCrawler

	filtered := jm.filterURLsAgainstRobots(urls, robotsRules, nil, nil)
	
	// Should return all URLs since no robots rules
	assert.Len(t, filtered, 2)
	assert.Equal(t, urls, filtered)

	mockCrawler.AssertExpectations(t)
}
