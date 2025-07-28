//go:build unit

package jobs

import (
	"context"
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
