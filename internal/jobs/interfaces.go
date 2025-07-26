package jobs

import (
	"context"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
)

// CrawlerInterface defines the methods we need from the crawler
type CrawlerInterface interface {
	WarmURL(ctx context.Context, url string, findLinks bool) (*crawler.CrawlResult, error)
	DiscoverSitemaps(ctx context.Context, domain string) ([]string, error)
	ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error)
	FilterURLs(urls []string, includePaths, excludePaths []string) []string
}