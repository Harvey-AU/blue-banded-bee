package crawler

// CacheCheckAttempt stores the result of a single cache status check.
type CacheCheckAttempt struct {
	Attempt     int    `json:"attempt"`
	CacheStatus string `json:"cache_status"`
	Delay       int    `json:"delay_ms"`
}

// CrawlResult represents the result of a URL crawl operation
type CrawlResult struct {
	URL                 string              `json:"url"`
	ResponseTime        int64               `json:"response_time"`
	StatusCode          int                 `json:"status_code"`
	Error               string              `json:"error,omitempty"`
	Warning             string              `json:"warning,omitempty"`
	CacheStatus         string              `json:"cache_status"`
	ContentType         string              `json:"content_type"`
	Timestamp           int64               `json:"timestamp"`
	RetryCount          int                 `json:"retry_count"`
	SkippedCrawl        bool                `json:"skipped_crawl,omitempty"`
	Links               map[string][]string `json:"links,omitempty"`
	SecondResponseTime  int64               `json:"second_response_time,omitempty"`
	SecondCacheStatus   string              `json:"second_cache_status,omitempty"`
	CacheCheckAttempts  []CacheCheckAttempt `json:"cache_check_attempts,omitempty"`
}

// CrawlOptions defines configuration options for a crawl operation
type CrawlOptions struct {
	MaxPages    int  // Maximum pages to crawl
	Concurrency int  // Number of concurrent crawlers
	RateLimit   int  // Maximum requests per second
	Timeout     int  // Request timeout in seconds
	FollowLinks bool // Whether to follow links on crawled pages
}
