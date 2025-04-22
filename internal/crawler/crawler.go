package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

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

	// Add this to capture requests and responses
	c.OnRequest(func(r *colly.Request) {
		log.Debug().
			Str("url", r.URL.String()).
			Msg("Crawler sending request")
	})

	// Conditionally register link extractor if enabled
	if config.FindLinks {
		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			href := e.Attr("href")
			// Normalize URL (absolute)
			u := e.Request.AbsoluteURL(href)
			// Append to result.Links via context
			if r, ok := e.Request.Ctx.GetAny("result").(*CrawlResult); ok {
				r.Links = append(r.Links, u)
			}
		})
	}

	return &Crawler{
		config: config,
		colly:  c,
		id:     crawlerID,
	}
}

// WarmURL performs a crawl of the specified URL and returns the result.
// It respects context cancellation, enforces timeout, and treats non-2xx statuses as errors.
func (c *Crawler) WarmURL(ctx context.Context, targetURL string) (*CrawlResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		res := &CrawlResult{URL: targetURL, Timestamp: time.Now().Unix(), Error: err.Error()}
		return res, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		err := fmt.Errorf("invalid URL format: %s", targetURL)
		res := &CrawlResult{URL: targetURL, Timestamp: time.Now().Unix(), Error: err.Error()}
		return res, err
	}
	start := time.Now()
	res := &CrawlResult{URL: targetURL, Timestamp: start.Unix()}
	// Seed result into collector context so OnHTML callback appends to res.Links
	if c.config.FindLinks {
		ctxColly := colly.NewContext()
		ctxColly.Put("result", res)
		c.colly.Request("GET", targetURL, nil, ctxColly, nil)
		c.colly.Wait()
	}
	client := &http.Client{Timeout: c.config.DefaultTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		res.Error = err.Error()
		res.ResponseTime = time.Since(start).Milliseconds()
		return res, err
	}
	resp, err := client.Do(req)
	res.ResponseTime = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	defer resp.Body.Close()
	// read body for link extraction
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.StatusCode = resp.StatusCode
	res.CacheStatus = resp.Header.Get("CF-Cache-Status")
	res.ContentType = resp.Header.Get("Content-Type")
	if c.config.FindLinks {
		res.Links = extractLinks(bodyBytes, targetURL)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch {
		case resp.StatusCode == 404:
			res.Error = "HTTP 404: Page not found"
		case resp.StatusCode == 403:
			res.Error = "HTTP 403: Access forbidden"
		case resp.StatusCode == 401:
			res.Error = "HTTP 401: Authentication required"
		case resp.StatusCode == 429:
			res.Error = "HTTP 429: Too many requests - rate limited"
		case resp.StatusCode >= 500 && resp.StatusCode < 600:
			res.Error = fmt.Sprintf("HTTP %d: Server error", resp.StatusCode)
		default:
			res.Error = fmt.Sprintf("HTTP %d: Non-successful status code", resp.StatusCode)
		}
		return res, nil
	}
	return res, nil
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

// extractLinks parses HTML body and returns all anchor hrefs as absolute URLs
func extractLinks(body []byte, base string) []string {
	var links []string
	baseURL, err := url.Parse(base)
	if err != nil {
		return links
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return links
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					u, err := url.Parse(attr.Val)
					if err == nil {
						abs := baseURL.ResolveReference(u)
						links = append(links, abs.String())
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return links
}

// Config returns the Crawler's configuration.
func (c *Crawler) Config() *Config {
	return c.config
}
