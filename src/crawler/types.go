package crawler

type CrawlResult struct {
	URL          string
	ResponseTime int64
	StatusCode   int
	Error        string
	CacheStatus  string
	Timestamp    int64
}

type CrawlOptions struct {
	MaxDepth    int
	Concurrency int
	RateLimit   int
	Timeout     int
	FollowLinks bool
}