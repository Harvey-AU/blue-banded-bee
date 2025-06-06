package crawler

// CrawlResult represents the result of a URL crawl operation
type CrawlResult struct {
	URL                 string   // The URL that was crawled
	ResponseTime        int64    // Response time in milliseconds
	StatusCode          int      // HTTP status code
	Error               string   // Error message if any
	Warning             string   // Warning message if any
	CacheStatus         string   // Cache status (e.g., HIT, MISS)
	ContentType         string   // Content type of the response
	Timestamp           int64    // Unix timestamp of the crawl
	RetryCount          int      // Number of retries performed
	SkippedCrawl        bool     // Whether full crawl was skipped due to cache hit
	Links               []string // Extracted hyperlinks (including PDFs/docs)
	SecondResponseTime  int64    // Response time of second request in milliseconds (if made)
	SecondCacheStatus   string   // Cache status of second request (if made)
}

// CrawlOptions defines configuration options for a crawl operation
type CrawlOptions struct {
	MaxPages    int  // Maximum pages to crawl
	Concurrency int  // Number of concurrent crawlers
	RateLimit   int  // Maximum requests per second
	Timeout     int  // Request timeout in seconds
	FollowLinks bool // Whether to follow links on crawled pages
}
