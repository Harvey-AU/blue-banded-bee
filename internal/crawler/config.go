package crawler

import (
	"time"
)

// Config holds the configuration for a crawler instance
type Config struct {
	DefaultTimeout time.Duration // Default timeout for requests
	MaxConcurrency int           // Maximum number of concurrent requests
	RateLimit      int           // Maximum requests per second
	UserAgent      string        // User agent string for requests
	RetryAttempts  int           // Number of retry attempts for failed requests
	RetryDelay     time.Duration // Delay between retry attempts
	SkipCachedURLs bool          // Whether to skip URLs that are already cached (HIT)
	Port           string        // Server port
	Env            string        // Environment (development/production)
	LogLevel       string        // Logging level
	DatabaseURL    string        // Database connection URL
	AuthToken      string        // Database authentication token
	SentryDSN      string        // Sentry DSN for error tracking
	FindLinks      bool          // Whether to extract links (e.g. PDFs/docs) from pages
}

// DefaultConfig returns a Config instance with default values
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout: 30 * time.Second,
		MaxConcurrency: 10,
		RateLimit:      3,
		UserAgent:      "Blue Banded Bee (Cache-warmer)",
		RetryAttempts:  3,
		RetryDelay:     500 * time.Millisecond,
		SkipCachedURLs: false, // Default to crawling all URLs
		FindLinks:      false,
	}
}
