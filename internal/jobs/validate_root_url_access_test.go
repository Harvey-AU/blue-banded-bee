package jobs

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestValidateRootURLAccessWithCrawler(t *testing.T) {
	tests := []struct {
		name          string
		robotsRules   *crawler.RobotsRules
		crawlerError  error
		expectError   bool
		expectAllowed bool
	}{
		{
			name:          "no_robots_rules_allows_access",
			robotsRules:   nil,
			crawlerError:  nil,
			expectError:   false,
			expectAllowed: true,
		},
		{
			name: "robots_allows_root_path",
			robotsRules: &crawler.RobotsRules{
				CrawlDelay:       1,
				Sitemaps:         []string{},
				DisallowPatterns: []string{}, // Empty disallow patterns allow everything
				AllowPatterns:    []string{},
			},
			crawlerError:  nil,
			expectError:   false,
			expectAllowed: true,
		},
		{
			name:          "crawler_error_propagated",
			robotsRules:   nil,
			crawlerError:  assert.AnError,
			expectError:   true,
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := &RobotsValidationMock{}
			mockCrawler := &RobotsValidationCrawlerMock{
				returnRules: tt.robotsRules,
				returnError: tt.crawlerError,
			}

			jm := &JobManager{
				dbQueue: mockQueue,
				crawler: mockCrawler,
			}

			job := &Job{
				ID:     "job-test",
				Domain: "example.com",
			}

			ctx := context.Background()
			rules, err := jm.validateRootURLAccess(ctx, job, "example.com", "/")

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, rules)
				// Should have attempted to update job status on error
				assert.True(t, mockQueue.executeCalled)
			} else {
				assert.NoError(t, err)
				if tt.robotsRules != nil {
					assert.Equal(t, tt.robotsRules, rules)
				}
			}

			assert.True(t, mockCrawler.discoverCalled)
		})
	}
}

func TestValidateRootURLAccessWithoutCrawler(t *testing.T) {
	// Test when crawler is nil
	jm := &JobManager{
		dbQueue: &RobotsValidationMock{},
		crawler: nil, // No crawler
	}

	job := &Job{
		ID:     "job-test",
		Domain: "example.com",
	}

	ctx := context.Background()
	rules, err := jm.validateRootURLAccess(ctx, job, "example.com", "/")

	assert.NoError(t, err)
	assert.Nil(t, rules) // Should return nil when no crawler
}

func TestValidateRootURLAccessParameterHandling(t *testing.T) {
	// Test parameter validation
	mockQueue := &RobotsValidationMock{}
	mockCrawler := &RobotsValidationCrawlerMock{}

	jm := &JobManager{
		dbQueue: mockQueue,
		crawler: mockCrawler,
	}

	tests := []struct {
		name             string
		job              *Job
		normalisedDomain string
		rootPath         string
	}{
		{
			name: "standard_parameters",
			job: &Job{
				ID:     "job-123",
				Domain: "example.com",
			},
			normalisedDomain: "example.com",
			rootPath:         "/",
		},
		{
			name: "different_domains",
			job: &Job{
				ID:     "job-456",
				Domain: "test.co.uk",
			},
			normalisedDomain: "test.co.uk",
			rootPath:         "/",
		},
		{
			name: "custom_root_path",
			job: &Job{
				ID:     "job-789",
				Domain: "api.com",
			},
			normalisedDomain: "api.com",
			rootPath:         "/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := jm.validateRootURLAccess(ctx, tt.job, tt.normalisedDomain, tt.rootPath)

			// Should not panic and should call crawler
			assert.NoError(t, err)
			assert.True(t, mockCrawler.discoverCalled)

			// Reset for next test
			mockCrawler.discoverCalled = false
		})
	}
}

// RobotsValidationMock for testing robots validation
type RobotsValidationMock struct {
	executeCalled bool
}

func (m *RobotsValidationMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	return nil // Simulate successful execution
}

func (m *RobotsValidationMock) ExecuteMaintenance(ctx context.Context, fn func(*sql.Tx) error) error {
	return m.Execute(ctx, fn)
}

func (m *RobotsValidationMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *RobotsValidationMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}

// RobotsValidationCrawlerMock for testing robots validation
type RobotsValidationCrawlerMock struct {
	discoverCalled bool
	returnRules    *crawler.RobotsRules
	returnError    error
}

func (m *RobotsValidationCrawlerMock) DiscoverSitemapsAndRobots(ctx context.Context, domain string) (*crawler.SitemapDiscoveryResult, error) {
	m.discoverCalled = true
	return &crawler.SitemapDiscoveryResult{
		Sitemaps:    []string{},
		RobotsRules: m.returnRules,
	}, m.returnError
}

func (m *RobotsValidationCrawlerMock) WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error) {
	return nil, nil
}

func (m *RobotsValidationCrawlerMock) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	return nil, nil
}

func (m *RobotsValidationCrawlerMock) FilterURLs(urls []string, includePaths, excludePaths []string) []string {
	return urls
}

func (m *RobotsValidationCrawlerMock) GetUserAgent() string {
	return "test-agent"
}
