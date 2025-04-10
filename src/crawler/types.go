package crawler

// CrawlResult represents the result of a URL crawl operation
type CrawlResult struct {
	URL          string // The URL that was crawled
	ResponseTime int64  // Response time in milliseconds
	StatusCode   int    // HTTP status code
	Error        string // Error message if any
	Warning      string // Warning message if any
	CacheStatus  string // Cache status (e.g., HIT, MISS)
	ContentType  string // Content type of the response
	Timestamp    int64  // Unix timestamp of the crawl
	RetryCount   int    // Number of retries performed
}

// CrawlOptions defines configuration options for a crawl operation
type CrawlOptions struct {
	MaxDepth    int  // Maximum depth for crawling linked pages
	Concurrency int  // Number of concurrent crawlers
	RateLimit   int  // Maximum requests per second
	Timeout     int  // Request timeout in seconds
	FollowLinks bool // Whether to follow links on crawled pages
}
