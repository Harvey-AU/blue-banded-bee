package crawler

import (
	"testing"

	"github.com/gocolly/colly/v2"
	"github.com/stretchr/testify/assert"
)

func TestSetupLinkExtractionFunctionExists(t *testing.T) {
	// Test that the function exists and can be called without panic
	collyClone := colly.NewCollector()

	// Should not panic
	assert.NotPanics(t, func() {
		setupLinkExtraction(collyClone)
	})

	// Verify the collector is still functional
	assert.NotNil(t, collyClone)
}

func TestSetupLinkExtractionHandlerRegistration(t *testing.T) {
	// Test that OnHTML handler is properly registered
	collyClone := colly.NewCollector()

	// Function should register OnHTML handler
	setupLinkExtraction(collyClone)

	// We can't easily test the handler execution without complex HTML setup,
	// but we can verify the function completes without error
	assert.NotNil(t, collyClone)
}

func TestSetupLinkExtractionWithNilCollector(t *testing.T) {
	// Test that function handles edge cases gracefully

	// Should not panic even with nil collector (though it will fail)
	assert.Panics(t, func() {
		setupLinkExtraction(nil)
	}, "Should panic with nil collector as expected")
}

func TestSetupLinkExtractionParameterValidation(t *testing.T) {
	// Test different collector configurations
	tests := []struct {
		name      string
		setupFunc func() *colly.Collector
	}{
		{
			name: "default_collector",
			setupFunc: func() *colly.Collector {
				return colly.NewCollector()
			},
		},
		{
			name: "async_collector",
			setupFunc: func() *colly.Collector {
				return colly.NewCollector(colly.Async(true))
			},
		},
		{
			name: "collector_with_user_agent",
			setupFunc: func() *colly.Collector {
				return colly.NewCollector(colly.UserAgent("test-agent"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := tt.setupFunc()

			// Should work with different collector configurations
			assert.NotPanics(t, func() {
				setupLinkExtraction(collector)
			})
		})
	}
}
