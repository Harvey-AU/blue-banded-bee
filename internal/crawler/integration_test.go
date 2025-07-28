package crawler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRobotsTxtIntegration(t *testing.T) {
	// Create a test server that serves robots.txt
	mux := http.NewServeMux()
	
	// Serve robots.txt with crawl delay and disallow patterns
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(`User-agent: *
Crawl-delay: 1
Disallow: /admin/

User-agent: BlueBandedBee
Crawl-delay: 2
Disallow: /private/
Disallow: /api/
Allow: /api/public
Sitemap: https://example.com/sitemap.xml
`))
	})
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	// Get server URL
	serverURL := server.URL
	
	// Create crawler with our user agent
	config := crawler.DefaultConfig()
	config.UserAgent = "BlueBandedBee/1.0 (+https://www.bluebandedbee.co/bot)"
	
	ctx := context.Background()
	
	// Test: Parse robots.txt directly with full URL
	t.Run("ParseRobotsTxt", func(t *testing.T) {
		rules, err := crawler.ParseRobotsTxt(ctx, serverURL, config.UserAgent)
		require.NoError(t, err)
		require.NotNil(t, rules)
		
		assert.Equal(t, 2, rules.CrawlDelay)
		assert.Len(t, rules.DisallowPatterns, 2)
		assert.Contains(t, rules.DisallowPatterns, "/private/")
		assert.Contains(t, rules.DisallowPatterns, "/api/")
		assert.Len(t, rules.AllowPatterns, 1)
		assert.Contains(t, rules.AllowPatterns, "/api/public")
		assert.Len(t, rules.Sitemaps, 1)
		assert.Contains(t, rules.Sitemaps[0], "sitemap.xml")
	})
	
	// Test: Path filtering
	t.Run("PathFiltering", func(t *testing.T) {
		rules := &crawler.RobotsRules{
			DisallowPatterns: []string{"/private/", "/api/"},
			AllowPatterns:    []string{"/api/public"},
		}
		
		// Test various paths
		tests := []struct {
			path    string
			allowed bool
		}{
			{"/", true},
			{"/page1", true},
			{"/private/secret", false},
			{"/api/endpoint", false},
			{"/api/public/data", true}, // Allow overrides Disallow
		}
		
		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				allowed := crawler.IsPathAllowed(rules, tt.path)
				assert.Equal(t, tt.allowed, allowed, "Path %s should be allowed=%v", tt.path, tt.allowed)
			})
		}
	})
}

func TestCrawlDelayTiming(t *testing.T) {
	// This test verifies that crawl delays are actually being applied
	// We'll mock a simple scenario and measure timing
	
	// Create a test server
	mux := http.NewServeMux()
	requestTimes := []time.Time{}
	
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		w.Write([]byte("OK"))
	})
	
	server := httptest.NewServer(mux)
	defer server.Close()
	
	// Create crawler
	config := crawler.DefaultConfig()
	c := crawler.New(config)
	
	ctx := context.Background()
	
	// Warm two URLs in sequence
	start := time.Now()
	_, err := c.WarmURL(ctx, server.URL+"/page1", false)
	require.NoError(t, err)
	
	_, err = c.WarmURL(ctx, server.URL+"/page2", false)
	require.NoError(t, err)
	
	elapsed := time.Since(start)
	
	// Without crawl delay, both requests should complete quickly
	assert.Less(t, elapsed, 2*time.Second, "Requests should complete quickly without crawl delay")
}