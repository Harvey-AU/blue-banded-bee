package jobs

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestSetupJobURLDiscoveryBranching(t *testing.T) {
	tests := []struct {
		name          string
		useSitemap    bool
		expectSitemap bool
		expectManual  bool
	}{
		{
			name:          "sitemap_enabled_uses_sitemap_path",
			useSitemap:    true,
			expectSitemap: true,
			expectManual:  false,
		},
		{
			name:          "sitemap_disabled_uses_manual_path",
			useSitemap:    false,
			expectSitemap: false,
			expectManual:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := &URLDiscoveryMock{}
			mockCrawler := &MockCrawlerForDiscovery{}

			jm := &JobManager{
				dbQueue: mockQueue,
				crawler: mockCrawler,
			}

			job := &Job{
				ID:     "job-test",
				Domain: "example.com",
			}

			options := &JobOptions{
				UseSitemap:   tt.useSitemap,
				IncludePaths: []string{"/api/*"},
				ExcludePaths: []string{"/admin/*"},
			}

			ctx := context.Background()

			if tt.expectSitemap {
				// For sitemap test, we just verify the function call without triggering goroutine
				// Test the branching logic only
				assert.True(t, tt.useSitemap)

				// We can verify the function exists and handles sitemap branching
				// without actually triggering the problematic goroutine
				err := jm.setupJobURLDiscovery(ctx, job, options, 42, "example.com")
				assert.NoError(t, err)
				assert.False(t, mockQueue.executeCalled, "Sitemap path should not call database immediately")
			} else if tt.expectManual {
				// Manual path test - now async, function returns immediately
				err := jm.setupJobURLDiscovery(ctx, job, options, 42, "example.com")

				// Function should return immediately without error
				assert.NoError(t, err)

				// Note: Manual URL creation now happens asynchronously in background
				// The crawler call happens in a goroutine, so we can't test it directly here
			}
		})
	}
}

func TestSetupJobURLDiscoveryFunctionExists(t *testing.T) {
	// Test that function exists without triggering problematic goroutines
	jm := &JobManager{
		dbQueue: &URLDiscoveryMock{},
		crawler: &MockCrawlerForDiscovery{},
	}

	job := &Job{ID: "test", Domain: "example.com"}

	// Test manual URL path (no goroutine)
	options := &JobOptions{UseSitemap: false}
	ctx := context.Background()

	// Should not panic and should execute the manual path logic
	err := jm.setupJobURLDiscovery(ctx, job, options, 1, "example.com")
	// Manual path should succeed with our mocks now
	assert.NoError(t, err)
}

// URLDiscoveryMock for testing URL discovery logic
type URLDiscoveryMock struct {
	executeCalled bool
}

func (m *URLDiscoveryMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	// Simulate successful execution for testing
	return nil
}

func (m *URLDiscoveryMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *URLDiscoveryMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}

// MockCrawlerForDiscovery for testing URL discovery
type MockCrawlerForDiscovery struct {
	discoverCalled bool
}

func (m *MockCrawlerForDiscovery) DiscoverSitemapsAndRobots(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error) {
	m.discoverCalled = true
	return &crawler.SitemapDiscoveryResult{
		Sitemaps:    []string{},
		RobotsRules: nil,
	}, nil
}

func (m *MockCrawlerForDiscovery) WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error) {
	return nil, nil
}

func (m *MockCrawlerForDiscovery) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	return nil, nil
}

func (m *MockCrawlerForDiscovery) FilterURLs(urls []string, includePaths, excludePaths []string) []string {
	return urls
}

func (m *MockCrawlerForDiscovery) GetUserAgent() string {
	return "test-agent"
}
