package crawler

import "net/http"

// CacheCheckAttempt stores the result of a single cache status check.
type CacheCheckAttempt struct {
	Attempt     int    `json:"attempt"`
	CacheStatus string `json:"cache_status"`
	Delay       int    `json:"delay_ms"`
}

// PerformanceMetrics holds detailed timing information for a request.
type PerformanceMetrics struct {
	DNSLookupTime       int64 `json:"dns_lookup_time"`
	TCPConnectionTime   int64 `json:"tcp_connection_time"`
	TLSHandshakeTime    int64 `json:"tls_handshake_time"`
	TTFB                int64 `json:"ttfb"`
	ContentTransferTime int64 `json:"content_transfer_time"`
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
	ContentLength       int64               `json:"content_length"`
	Headers             http.Header         `json:"headers"`
	RedirectURL         string              `json:"redirect_url"`
	Performance         PerformanceMetrics  `json:"performance"`
	Timestamp           int64               `json:"timestamp"`
	RetryCount          int                 `json:"retry_count"`
	SkippedCrawl        bool                `json:"skipped_crawl,omitempty"`
	Links               map[string][]string `json:"links,omitempty"`
	SecondResponseTime  int64               `json:"second_response_time,omitempty"`
	SecondCacheStatus   string              `json:"second_cache_status,omitempty"`
	SecondContentLength int64               `json:"second_content_length,omitempty"`
	SecondHeaders       http.Header         `json:"second_headers,omitempty"`
	SecondPerformance   *PerformanceMetrics `json:"second_performance,omitempty"`
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
