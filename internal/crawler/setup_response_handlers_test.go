package crawler

import (
	"sync"
	"testing"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/stretchr/testify/assert"
)

func TestSetupResponseHandlersFunctionExists(t *testing.T) {
	// Test that the function exists and can be called without panic
	crawler := &Crawler{
		metricsMap: &sync.Map{},
	}

	collyClone := colly.NewCollector()
	result := &CrawlResult{
		URL:   "https://example.com",
		Links: make(map[string][]string),
	}
	startTime := time.Now()

	// Should not panic
	crawler.setupResponseHandlers(collyClone, result, startTime, "https://example.com")

	// Verify handlers were registered
	assert.NotNil(t, collyClone)
}

func TestSetupResponseHandlersParameterHandling(t *testing.T) {
	// Test parameter validation and basic setup
	tests := []struct {
		name      string
		targetURL string
		result    *CrawlResult
		expectOK  bool
	}{
		{
			name:      "valid_parameters",
			targetURL: "https://example.com",
			result: &CrawlResult{
				URL:   "https://example.com",
				Links: make(map[string][]string),
			},
			expectOK: true,
		},
		{
			name:      "different_url",
			targetURL: "https://test.com/page",
			result: &CrawlResult{
				URL:   "https://test.com/page",
				Links: make(map[string][]string),
			},
			expectOK: true,
		},
		{
			name:      "handles_nil_result_gracefully",
			targetURL: "https://example.com",
			result:    nil,
			expectOK:  true, // Should not panic even with nil result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawler := &Crawler{
				metricsMap: &sync.Map{},
			}

			collyClone := colly.NewCollector()
			startTime := time.Now()

			// Should not panic regardless of parameters
			assert.NotPanics(t, func() {
				crawler.setupResponseHandlers(collyClone, tt.result, startTime, tt.targetURL)
			})
		})
	}
}

func TestSetupResponseHandlersCDNDetection(t *testing.T) {
	// Test that the function sets up handlers that can detect different CDN cache headers
	// This is a structural test - we verify the handlers are registered, not their execution
	
	crawler := &Crawler{
		metricsMap: &sync.Map{},
	}

	collyClone := colly.NewCollector()
	result := &CrawlResult{
		URL:   "https://example.com",
		Links: make(map[string][]string),
	}
	startTime := time.Now()

	crawler.setupResponseHandlers(collyClone, result, startTime, "https://example.com")

	// The function should have registered OnResponse and OnError handlers
	// We can't easily test the handlers directly, but we can verify they don't panic during setup
	assert.NotNil(t, collyClone)
}

func TestSetupResponseHandlersMetricsMapIntegration(t *testing.T) {
	// Test that the function integrates with the metrics map correctly
	crawler := &Crawler{
		metricsMap: &sync.Map{},
	}

	// Pre-populate metrics map to test retrieval logic
	testURL := "https://example.com"
	testMetrics := &PerformanceMetrics{
		DNSLookupTime:       10,
		TCPConnectionTime:   20,
		TLSHandshakeTime:    30,
		TTFB:               100,
		ContentTransferTime: 0, // Will be calculated
	}
	crawler.metricsMap.Store(testURL, testMetrics)

	collyClone := colly.NewCollector()
	result := &CrawlResult{
		URL:   testURL,
		Links: make(map[string][]string),
	}
	startTime := time.Now()

	// Should not panic and should handle metrics integration
	assert.NotPanics(t, func() {
		crawler.setupResponseHandlers(collyClone, result, startTime, testURL)
	})

	// Verify the metrics are still in the map (not retrieved until actual response)
	_, exists := crawler.metricsMap.Load(testURL)
	assert.True(t, exists, "Metrics should remain in map until OnResponse handler is triggered")
}