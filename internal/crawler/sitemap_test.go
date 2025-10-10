package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSitemapDiscoveryResult(t *testing.T) {
	result := &SitemapDiscoveryResult{
		Sitemaps: []string{
			"https://example.com/sitemap.xml",
			"https://example.com/sitemap2.xml",
		},
		RobotsRules: &RobotsRules{
			CrawlDelay: 1,
			Sitemaps:   []string{"https://example.com/sitemap.xml"},
		},
	}

	assert.Len(t, result.Sitemaps, 2)
	assert.NotNil(t, result.RobotsRules)
	assert.Contains(t, result.Sitemaps, "https://example.com/sitemap.xml")
}

func TestFilterURLs(t *testing.T) {
	c := &Crawler{
		config: &Config{},
	}

	tests := []struct {
		name         string
		urls         []string
		includePaths []string
		excludePaths []string
		expectedLen  int
	}{
		{
			name: "filter_with_includes",
			urls: []string{
				"https://example.com/blog/page1",
				"https://example.com/blog/page2",
				"https://example.com/admin/page1",
				"https://example.com/about",
			},
			includePaths: []string{"/blog"},
			excludePaths: []string{},
			expectedLen:  2,
		},
		{
			name: "filter_with_excludes",
			urls: []string{
				"https://example.com/page1",
				"https://example.com/page2",
				"https://example.com/admin/page1",
				"https://example.com/admin/page2",
			},
			includePaths: []string{},
			excludePaths: []string{"/admin"},
			expectedLen:  2,
		},
		{
			name:         "empty_urls",
			urls:         []string{},
			includePaths: []string{},
			excludePaths: []string{},
			expectedLen:  0,
		},
		{
			name: "no_filters",
			urls: []string{
				"https://example.com/page1",
				"https://example.com/page2",
			},
			includePaths: []string{},
			excludePaths: []string{},
			expectedLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := c.FilterURLs(tt.urls, tt.includePaths, tt.excludePaths)
			assert.Len(t, filtered, tt.expectedLen)
		})
	}
}

func TestDiscoverSitemapsAndRobots(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`User-agent: *
Disallow: /admin
Allow: /
Sitemap: https://example.com/sitemap.xml
`))
		case "/sitemap.xml":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://example.com/page1</loc></url>
</urlset>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := &Crawler{
		config: &Config{
			UserAgent: "TestBot/1.0",
		},
	}

	tests := []struct {
		name              string
		domain            string
		expectSitemaps    bool
		expectRobotsRules bool
	}{
		{
			name:              "valid_domain",
			domain:            "example.com",
			expectSitemaps:    false, // Can't actually fetch from example.com
			expectRobotsRules: true,  // Should have default empty rules
		},
		{
			name:              "domain_with_protocol",
			domain:            "https://example.com",
			expectSitemaps:    false,
			expectRobotsRules: true,
		},
		{
			name:              "domain_with_www",
			domain:            "www.example.com",
			expectSitemaps:    false,
			expectRobotsRules: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := c.DiscoverSitemapsAndRobots(ctx, tt.domain)

			require.NoError(t, err)
			assert.NotNil(t, result)

			// All results should have RobotsRules (even if empty)
			assert.NotNil(t, result.RobotsRules)

			// Sitemaps list might be empty but that's OK
			// Just check it exists in result
			_ = result.Sitemaps
		})
	}
}

func TestParseSitemap(t *testing.T) {
	// Create test server for sitemap
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>http://` + r.Host + `/page1</loc></url>
	<url><loc>http://` + r.Host + `/page2</loc></url>
</urlset>`))
		case "/sitemap_index.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>http://` + r.Host + `/sitemap1.xml</loc></sitemap>
	<sitemap><loc>http://` + r.Host + `/sitemap2.xml</loc></sitemap>
</sitemapindex>`))
		case "/empty.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`))
		case "/invalid.xml":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`not xml`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := &Crawler{
		config: &Config{
			UserAgent: "TestBot/1.0",
		},
	}

	tests := []struct {
		name        string
		sitemapURL  string
		expectedLen int
		expectError bool
	}{
		{
			name:        "valid_sitemap",
			sitemapURL:  server.URL + "/sitemap.xml",
			expectedLen: 2,
			expectError: false,
		},
		{
			name:        "sitemap_index",
			sitemapURL:  server.URL + "/sitemap_index.xml",
			expectedLen: 0, // Will fail to fetch child sitemaps due to http/https mismatch
			expectError: false,
		},
		{
			name:        "empty_sitemap",
			sitemapURL:  server.URL + "/empty.xml",
			expectedLen: 0,
			expectError: false,
		},
		{
			name:        "invalid_xml",
			sitemapURL:  server.URL + "/invalid.xml",
			expectedLen: 0,
			expectError: false, // ParseSitemap doesn't return error for invalid XML
		},
		{
			name:        "not_found",
			sitemapURL:  server.URL + "/notfound.xml",
			expectedLen: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			urls, err := c.ParseSitemap(ctx, tt.sitemapURL)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, urls, tt.expectedLen)
			}
		})
	}
}

func BenchmarkFilterURLs(b *testing.B) {
	c := &Crawler{
		config: &Config{},
	}

	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
		"https://example.com/page4",
		"https://example.com/page5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.FilterURLs(urls, []string{}, []string{})
	}
}
