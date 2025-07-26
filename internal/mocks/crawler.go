package mocks

import (
	"context"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/stretchr/testify/mock"
)

// MockCrawler is a mock implementation of the Crawler
type MockCrawler struct {
	mock.Mock
}

// WarmURL mocks the WarmURL method
func (m *MockCrawler) WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error) {
	args := m.Called(ctx, url, findLinks)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).(*crawler.CrawlResult), args.Error(1)
}

// DiscoverSitemaps mocks the DiscoverSitemaps method
func (m *MockCrawler) DiscoverSitemaps(ctx context.Context, domain string) ([]string, error) {
	args := m.Called(ctx, domain)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).([]string), args.Error(1)
}

// ParseSitemap mocks the ParseSitemap method
func (m *MockCrawler) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	args := m.Called(ctx, sitemapURL)
	
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	
	return args.Get(0).([]string), args.Error(1)
}