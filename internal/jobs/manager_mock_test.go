//go:build unit

package jobs

import (
	"context"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/mocks"
	"github.com/stretchr/testify/assert"
)

// TestJobManagerWithMockCrawler proves that our interface refactoring works
func TestJobManagerWithMockCrawler(t *testing.T) {
	ctx := context.Background()
	
	// Create mock crawler
	mockCrawler := new(mocks.MockCrawler)
	
	// Set expectation
	mockCrawler.On("DiscoverSitemaps", ctx, "example.com").
		Return([]string{"https://example.com/sitemap.xml"}, nil)
	
	// Create JobManager with mock
	jm := &JobManager{
		crawler: mockCrawler, // This works now because crawler is CrawlerInterface!
		processedPages: make(map[string]struct{}),
	}
	
	// Call the method that uses crawler
	sitemaps, err := jm.crawler.DiscoverSitemaps(ctx, "example.com")
	
	// Assert
	assert.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/sitemap.xml"}, sitemaps)
	mockCrawler.AssertExpectations(t)
}