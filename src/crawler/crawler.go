package crawler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog/log"
)

// Crawler represents a URL crawler with configuration and metrics
type Crawler struct {
	config *Config
	colly  *colly.Collector
}

// New creates a new Crawler instance with the given configuration
// If config is nil, default configuration is used
func New(config *Config) *Crawler {
	if config == nil {
		config = DefaultConfig()
	}

	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(1),
		colly.Async(true),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: config.MaxConcurrency,
		RandomDelay: time.Second / time.Duration(config.RateLimit),
	})

	return &Crawler{
		config: config,
		colly:  c,
	}
}

// WarmURL performs a crawl of the specified URL and returns the result
// It validates the URL, makes the request, and tracks metrics like response time
// and cache status. Any non-2xx status code is treated as an error.
// The context can be used to cancel the request or set a timeout.
func (c *Crawler) WarmURL(ctx context.Context, targetURL string) (*CrawlResult, error) {
	// Create a collector that allows URL revisits for retries
	collector := colly.NewCollector(
		colly.UserAgent(c.config.UserAgent),
		colly.MaxDepth(1),
		colly.Async(true),
		colly.AllowURLRevisit(),  // Allow retrying the same URL
	)
	
	collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: c.config.MaxConcurrency,
		RandomDelay: time.Second / time.Duration(c.config.RateLimit),
	})
	
	span := sentry.StartSpan(ctx, "crawler.warm_url")
	defer span.Finish()

	span.SetTag("crawler.url", targetURL)
	start := time.Now()
	
	result := &CrawlResult{
		URL:       targetURL,
		Timestamp: time.Now().Unix(),
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		span.SetTag("error", "true")
		span.SetTag("error.type", "url_parse_error")
		span.SetData("error.message", err.Error())
		result.Error = err.Error()
		span.Finish()
		sentry.CaptureException(err)
		return result, err
	}

	// Additional validation
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		err := fmt.Errorf("invalid URL format: %s", targetURL)
		span.SetTag("error", "true")
		span.SetTag("error.type", "url_validation_error")
		span.SetData("error.message", err.Error())
		result.Error = err.Error()
		span.Finish()
		sentry.CaptureException(err)
		return result, err
	}

	// Set up the collector handlers
	collector.OnResponse(func(r *colly.Response) {
		result.StatusCode = r.StatusCode
		result.CacheStatus = r.Headers.Get("CF-Cache-Status")

		// Treat non-2xx status codes as errors
		if r.StatusCode < 200 || r.StatusCode >= 300 {
			result.Error = fmt.Sprintf("HTTP %d: Non-successful status code", r.StatusCode)
		}
	})

	collector.OnError(func(r *colly.Response, err error) {
		if r != nil {
			result.StatusCode = r.StatusCode
		}
		result.Error = err.Error()
	})

	// Define the retry strategy
	retryAttempts := c.config.RetryAttempts
	retryDelay := c.config.RetryDelay

	var lastErr error
	var success bool

	// Retry loop
	for attempt := 0; attempt <= retryAttempts; attempt++ {
		// Add attempt information to the span
		if attempt > 0 {
			span.SetTag("retry.attempt", fmt.Sprintf("%d", attempt))
			log.Info().
				Str("url", targetURL).
				Int("attempt", attempt).
				Msg("Retrying crawl")
		}

		// Clear previous error for new attempt
		result.Error = ""
		
		// Attempt the crawl
		err = collector.Visit(targetURL)
		
		// If context is canceled, stop retrying
		if ctx.Err() != nil {
			result.Error = fmt.Sprintf("Context canceled: %v", ctx.Err())
			return result, ctx.Err()
		}

		// Wait for crawl to complete
		collector.Wait()
		
		// If no error or non-retryable error, break the loop
		if err == nil && result.Error == "" {
			success = true
			break
		} else if err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Str("url", targetURL).
				Int("attempt", attempt+1).
				Int("max_attempts", retryAttempts+1).
				Msg("Crawl attempt failed")
		}
		
		// Check if we should retry
		if shouldRetry(err, result.StatusCode) && attempt < retryAttempts {
			// Wait before retrying with exponential backoff
			backoff := retryDelay * time.Duration(math.Pow(2, float64(attempt)))
			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				result.Error = fmt.Sprintf("Context canceled during retry wait: %v", ctx.Err())
				return result, ctx.Err()
			}
		} else {
			// No more retries
			break
		}
	}

	result.ResponseTime = time.Since(start).Milliseconds()
	span.SetData("response_time_ms", result.ResponseTime)
	span.SetTag("status_code", fmt.Sprintf("%d", result.StatusCode))
	span.SetTag("cache_status", result.CacheStatus)

	// If we didn't succeed after all attempts
	if !success {
		if lastErr != nil {
			return result, fmt.Errorf("failed to warm URL %s after %d attempts: %w", 
				targetURL, retryAttempts+1, lastErr)
		}
		return result, errors.New(result.Error)
	}

	return result, nil
}

// Helper function to determine if we should retry based on the error or status code
func shouldRetry(err error, statusCode int) bool {
	// Retry on network errors
	if err != nil {
		return true
	}
	
	// Retry on 5xx server errors
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	
	// Retry on 429 Too Many Requests
	if statusCode == 429 {
		return true
	}
	
	return false
}

func setupSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS crawl_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			response_time INTEGER NOT NULL,
			status_code INTEGER NOT NULL,
			error TEXT NULL,           -- Changed to allow NULL
			cache_status TEXT NULL,    -- Changed to allow NULL
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}
