package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog/log"
)

// Crawler represents a URL crawler with configuration and metrics
type Crawler struct {
	config *Config
	colly  *colly.Collector
	id     string // Add an ID field to identify each crawler instance
}

// New creates a new Crawler instance with the given configuration and optional ID
// If config is nil, default configuration is used
func New(config *Config, id ...string) *Crawler {
	if config == nil {
		config = DefaultConfig()
	}

	crawlerID := ""
	if len(id) > 0 {
		crawlerID = id[0]
	}

	userAgent := config.UserAgent
	if crawlerID != "" {
		userAgent = fmt.Sprintf("%s Worker-%s", config.UserAgent, crawlerID)
	}

	c := colly.NewCollector(
		colly.UserAgent(userAgent),
		colly.MaxDepth(1),
		colly.Async(true),
		colly.AllowURLRevisit(),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: config.MaxConcurrency,
		RandomDelay: time.Second / time.Duration(config.RateLimit),
	})

	return &Crawler{
		config: config,
		colly:  c,
		id:     crawlerID,
	}
}

// WarmURL performs a crawl of the specified URL and returns the result
// It validates the URL, makes the request, and tracks metrics like response time
// and cache status. Any non-2xx status code is treated as an error.
// The context can be used to cancel the request or set a timeout.
func (c *Crawler) WarmURL(ctx context.Context, targetURL string) (*CrawlResult, error) {
	span := sentry.StartSpan(ctx, "crawler.warm_url")
	defer span.Finish()

	span.SetTag("crawler.url", targetURL)
	span.SetTag("crawler.id", c.id) // Add crawler ID to spans
	start := time.Now()

	result := &CrawlResult{
		URL:       targetURL,
		Timestamp: time.Now().Unix(),
	}

	// Add this block after URL validation but before crawling
	if c.config.SkipCachedURLs {
		cacheStatus, checkErr := c.CheckCacheStatus(ctx, targetURL)
		if checkErr == nil && cacheStatus == "HIT" {
			log.Debug().
				Str("url", targetURL).
				Msg("URL already cached (HIT), skipping full crawl")

			result.StatusCode = http.StatusOK
			result.CacheStatus = cacheStatus
			result.ResponseTime = time.Since(start).Milliseconds()
			result.SkippedCrawl = true

			return result, nil
		}
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

	// Set up the response handlers
	c.colly.OnResponse(func(r *colly.Response) {
		result.StatusCode = r.StatusCode
		result.CacheStatus = r.Headers.Get("CF-Cache-Status")
		c.handleResponseType(result, r)
	})

	c.colly.OnError(func(r *colly.Response, err error) {
		if r != nil {
			result.StatusCode = r.StatusCode
		}
		result.Error = err.Error()
	})

	// Use existing collector for the request
	err = c.colly.Visit(targetURL)

	result.ResponseTime = time.Since(start).Milliseconds()
	span.SetData("response_time_ms", result.ResponseTime)
	span.SetTag("status_code", fmt.Sprintf("%d", result.StatusCode))
	span.SetTag("cache_status", result.CacheStatus)

	// Validate cache status
	c.validateCacheStatus(result)

	return result, err
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

// Add this function to crawler.go to improve error handling for different response types
func (c *Crawler) handleResponseType(result *CrawlResult, response *colly.Response) {
	// Get content type from headers
	contentType := response.Headers.Get("Content-Type")

	// Set content type for metrics
	result.ContentType = contentType

	// Check status code
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch {
		case response.StatusCode == 404:
			result.Error = "HTTP 404: Page not found"
		case response.StatusCode == 403:
			result.Error = "HTTP 403: Access forbidden"
		case response.StatusCode == 401:
			result.Error = "HTTP 401: Authentication required"
		case response.StatusCode == 429:
			result.Error = "HTTP 429: Too many requests - rate limited"
		case response.StatusCode >= 500 && response.StatusCode < 600:
			result.Error = fmt.Sprintf("HTTP %d: Server error", response.StatusCode)
		default:
			result.Error = fmt.Sprintf("HTTP %d: Non-successful status code", response.StatusCode)
		}
		return
	}

	// For 200-level responses, check for specific content types
	switch {
	case strings.Contains(contentType, "text/html"):
		// HTML content - check for specific patterns
		if len(response.Body) < 100 {
			// Very small HTML response might indicate an error page
			result.Warning = "Warning: Unusually small HTML response"
		}

		// Check for common error patterns in the body
		bodyStr := string(response.Body)
		if strings.Contains(bodyStr, "<title>404") ||
			strings.Contains(bodyStr, "not found") ||
			strings.Contains(bodyStr, "page doesn't exist") {
			result.Warning = "Warning: Page content suggests a 404 despite 200 status code"
		}

	case strings.Contains(contentType, "application/json"):
		// JSON content - validate it's proper JSON
		var jsonObj map[string]interface{}
		if err := json.Unmarshal(response.Body, &jsonObj); err != nil {
			result.Warning = "Warning: Invalid JSON response"
		}

		// Check for error fields in the JSON
		if errorMsg, ok := jsonObj["error"].(string); ok {
			result.Warning = fmt.Sprintf("Warning: JSON contains error field: %s", errorMsg)
		}

	case strings.Contains(contentType, "text/plain"):
		// Plain text - check for obvious error messages
		bodyStr := string(response.Body)
		if strings.Contains(strings.ToLower(bodyStr), "error") ||
			strings.Contains(strings.ToLower(bodyStr), "not found") {
			result.Warning = "Warning: Text appears to contain error message"
		}
	}

	// Check for very small response sizes that might indicate an error
	if len(response.Body) == 0 {
		result.Warning = "Warning: Empty response body"
	}
}

// Add this function to crawler.go to validate cache status
func (c *Crawler) validateCacheStatus(result *CrawlResult) {
	// Don't validate if there was an error
	if result.Error != "" {
		return
	}

	// Check the cache status
	switch result.CacheStatus {
	case "HIT":
		// Successful cache hit
		log.Debug().
			Str("url", result.URL).
			Msg("Cache hit confirmed")

	case "MISS":
		// Cache miss - this might be expected for the first request
		log.Debug().
			Str("url", result.URL).
			Msg("Cache miss detected")

	case "EXPIRED":
		// The cached resource was expired
		result.Warning = "Cache expired - resource needed revalidation"

	case "BYPASS":
		// Cache was bypassed
		result.Warning = "Cache was bypassed - check cache headers"

	case "DYNAMIC":
		// Content was dynamically generated
		result.Warning = "Content served dynamically - not cacheable"

	case "":
		// No cache status header found
		result.Warning = "No cache status header found - CDN might not be enabled"

	default:
		// Unknown cache status
		result.Warning = fmt.Sprintf("Unknown cache status: %s", result.CacheStatus)
	}
}

func (c *Crawler) CheckCacheStatus(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", c.config.UserAgent)

	client := &http.Client{
		Timeout: c.config.DefaultTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Header.Get("CF-Cache-Status"), nil
}

// CreateHTTPClient returns a configured HTTP client
func (c *Crawler) CreateHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = c.config.DefaultTimeout
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 25,
			MaxConnsPerHost:     50,
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableCompression:  true,
			ForceAttemptHTTP2:   true,
		},
	}
}
