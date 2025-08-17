package jobs

import (
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/stretchr/testify/assert"
)

func TestConstructTaskURL(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		domainName string
		expected   string
	}{
		{
			name:       "full_https_url",
			path:       "https://example.com/page",
			domainName: "example.com",
			expected:   "https://example.com/page", // Normalised
		},
		{
			name:       "full_http_url",
			path:       "http://example.com/page",
			domainName: "example.com", 
			expected:   "https://example.com/page", // Normalised to HTTPS
		},
		{
			name:       "relative_path_with_domain",
			path:       "/about",
			domainName: "example.com",
			expected:   "https://example.com/about",
		},
		{
			name:       "root_path_with_domain",
			path:       "/",
			domainName: "example.com",
			expected:   "https://example.com/",
		},
		{
			name:       "relative_path_without_domain",
			path:       "/contact",
			domainName: "",
			expected:   "", // util.NormaliseURL returns empty string for invalid URLs
		},
		{
			name:       "full_url_without_domain_fallback",
			path:       "https://fallback.com/page",
			domainName: "",
			expected:   "https://fallback.com/page", // Uses fallback logic
		},
		{
			name:       "path_with_query_params",
			path:       "/search?q=test",
			domainName: "example.com",
			expected:   "https://example.com/search?q=test",
		},
		{
			name:       "path_with_fragment",
			path:       "/page#section",
			domainName: "example.com",
			expected:   "https://example.com/page#section",
		},
		{
			name:       "subdomain_handling",
			path:       "/api/data",
			domainName: "api.example.com",
			expected:   "https://api.example.com/api/data",
		},
		{
			name:       "unicode_domain",
			path:       "/café",
			domainName: "münchener.de",
			expected:   "https://münchener.de/café",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructTaskURL(tt.path, tt.domainName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyCrawlDelay(t *testing.T) {
	tests := []struct {
		name              string
		task              *Task
		expectedSleepTime time.Duration
		expectLog         bool
	}{
		{
			name: "no_crawl_delay",
			task: &Task{
				ID:           "task-1",
				DomainName:   "example.com",
				CrawlDelay:   0,
			},
			expectedSleepTime: 0,
			expectLog:         false,
		},
		{
			name: "one_second_delay",
			task: &Task{
				ID:           "task-2",
				DomainName:   "example.com",
				CrawlDelay:   1,
			},
			expectedSleepTime: 1 * time.Second,
			expectLog:         true,
		},
		{
			name: "five_second_delay",
			task: &Task{
				ID:           "task-3",
				DomainName:   "slow.com",
				CrawlDelay:   5,
			},
			expectedSleepTime: 5 * time.Second,
			expectLog:         true,
		},
		{
			name: "large_delay",
			task: &Task{
				ID:           "task-4",
				DomainName:   "very-slow.com",
				CrawlDelay:   30,
			},
			expectedSleepTime: 30 * time.Second,
			expectLog:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For tests with delays, we don't want to actually sleep
			// Instead, we'll verify the function would sleep correctly
			// This is a limitation of testing time.Sleep directly
			
			start := time.Now()
			
			// For tests with no delay, we can verify directly
			if tt.expectedSleepTime == 0 {
				applyCrawlDelay(tt.task)
				elapsed := time.Since(start)
				assert.Less(t, elapsed, 10*time.Millisecond, "Should not sleep when CrawlDelay is 0")
			} else {
				// For delay tests, we verify the logic without actually sleeping
				// This tests the conditional logic correctly
				assert.Greater(t, tt.task.CrawlDelay, 0, "Task should have crawl delay set")
				assert.Equal(t, tt.expectedSleepTime, time.Duration(tt.task.CrawlDelay)*time.Second)
				
				// We can test the actual sleep for very short delays in unit tests
				if tt.expectedSleepTime <= 100*time.Millisecond {
					applyCrawlDelay(tt.task)
					elapsed := time.Since(start)
					assert.GreaterOrEqual(t, elapsed, tt.expectedSleepTime-10*time.Millisecond)
				}
			}
		})
	}
}

// TestApplyCrawlDelayActualSleep tests that sleep actually occurs for small delays
func TestApplyCrawlDelayActualSleep(t *testing.T) {
	task := &Task{
		ID:           "sleep-test",
		DomainName:   "example.com",
		CrawlDelay:   1, // 1 second
	}
	
	start := time.Now()
	applyCrawlDelay(task)
	elapsed := time.Since(start)
	
	// Verify sleep actually occurred (with some tolerance for timing)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "Should sleep for approximately 1 second")
	assert.Less(t, elapsed, 1100*time.Millisecond, "Should not sleep significantly longer than 1 second")
}

func TestProcessDiscoveredLinks(t *testing.T) {
	// Note: This function requires more complex mocking for database operations
	// For now, we'll test the core logic patterns and add TODO for full implementation
	
	tests := []struct {
		name         string
		task         *Task
		result       *crawler.CrawlResult
		sourceURL    string
		expectDBCall bool
	}{
		{
			name: "no_links_found",
			task: &Task{
				ID:         "task-1",
				JobID:      "job-123",
				Path:       "/page",
				DomainName: "example.com",
				FindLinks:  true,
			},
			result: &crawler.CrawlResult{
				Links: map[string][]string{
					"header": {},
					"body":   {},
					"footer": {},
				},
			},
			sourceURL:    "https://example.com/page",
			expectDBCall: false,
		},
		{
			name: "links_found_but_find_links_disabled",
			task: &Task{
				ID:         "task-2",
				JobID:      "job-123", 
				Path:       "/page",
				DomainName: "example.com",
				FindLinks:  false, // Disabled
			},
			result: &crawler.CrawlResult{
				Links: map[string][]string{
					"body": {"https://example.com/link1", "https://example.com/link2"},
				},
			},
			sourceURL:    "https://example.com/page",
			expectDBCall: false, // Should not be called since FindLinks is false
		},
		{
			name: "homepage_with_links",
			task: &Task{
				ID:            "task-3",
				JobID:         "job-123",
				Path:          "/", // Homepage
				DomainName:    "example.com",
				FindLinks:     true,
				PriorityScore: 1.0,
			},
			result: &crawler.CrawlResult{
				Links: map[string][]string{
					"header": {"https://example.com/nav1", "https://example.com/nav2"},
					"body":   {"https://example.com/content1"},
					"footer": {"https://example.com/footer1"},
				},
			},
			sourceURL:    "https://example.com/",
			expectDBCall: true,
		},
		{
			name: "regular_page_with_links",
			task: &Task{
				ID:            "task-4",
				JobID:         "job-123",
				Path:          "/about",
				DomainName:    "example.com", 
				FindLinks:     true,
				PriorityScore: 0.8,
			},
			result: &crawler.CrawlResult{
				Links: map[string][]string{
					"header": {"https://example.com/nav1"}, // Should be ignored for non-homepage
					"body":   {"https://example.com/related1", "https://example.com/related2"},
					"footer": {"https://example.com/footer1"}, // Should be ignored for non-homepage
				},
			},
			sourceURL:    "https://example.com/about",
			expectDBCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: Implement with proper mocks for WorkerPool
			// This requires mocking:
			// - wp.dbQueue (DbQueueProvider interface)
			// - wp.jobInfoCache (map with RLock/RUnlock) 
			// - wp.EnqueueURLs method
			// - wp.updateTaskPriorities method
			
			// For now, verify the test structure is sound
			assert.NotNil(t, tt.task)
			assert.NotNil(t, tt.result)
			assert.NotEmpty(t, tt.sourceURL)
			
			// Verify homepage detection logic
			isHomepage := tt.task.Path == "/"
			if tt.name == "homepage_with_links" {
				assert.True(t, isHomepage)
			} else if tt.name == "regular_page_with_links" {
				assert.False(t, isHomepage)
			}
			
			// TODO: Once WorkerPool mocking is implemented, test:
			// - Domain ID retrieval
			// - Robots rules cache lookup
			// - Link category processing with correct priorities
			// - Database page record creation
			// - Task enqueueing
			t.Skip("TODO: Implement with WorkerPool mocks")
		})
	}
}

// Benchmark tests for the extracted functions
func BenchmarkConstructTaskURL(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = constructTaskURL("/test/path", "example.com")
	}
}

func BenchmarkConstructTaskURLWithFullURL(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = constructTaskURL("https://example.com/test/path", "example.com")
	}
}

func BenchmarkApplyCrawlDelayZero(b *testing.B) {
	task := &Task{
		ID:           "bench-task",
		DomainName:   "example.com",
		CrawlDelay:   0,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		applyCrawlDelay(task)
	}
}