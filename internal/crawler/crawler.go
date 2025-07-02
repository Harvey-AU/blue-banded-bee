package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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

	// Set HTTP client with proper timeout
	httpClient := &http.Client{
		Timeout: config.DefaultTimeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 25,
			MaxConnsPerHost:     50,
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableCompression:  true,
			ForceAttemptHTTP2:   true,
		},
	}
	c.SetClient(httpClient)

	// Add this to capture requests and responses
	c.OnRequest(func(r *colly.Request) {
		log.Debug().
			Str("url", r.URL.String()).
			Msg("Crawler sending request")
	})

	// Note: OnHTML handler will be registered on the clone in WarmURL to ensure proper context access

	return &Crawler{
		config: config,
		colly:  c,
		id:     crawlerID,
	}
}

// WarmURL performs a crawl of the specified URL and returns the result.
// It respects context cancellation, enforces timeout, and treats non-2xx statuses as errors.
func (c *Crawler) WarmURL(ctx context.Context, targetURL string, findLinks bool) (*CrawlResult, error) {
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

	log.Debug().
		Str("url", targetURL).
		Bool("find_links", findLinks).
		Msg("Starting URL warming with Colly")

	// Use Colly for everything - single request handles cache warming and link extraction
	collyClone := c.colly.Clone()
	
	// Register link extractor on the clone to ensure proper context access
	log.Debug().
		Msg("Registering Colly link extractor on clone")

	collyClone.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// Check if link extraction is enabled for this request
		findLinksVal := e.Request.Ctx.GetAny("find_links")
		if findLinksVal == nil {
			log.Debug().
				Str("url", e.Request.URL.String()).
				Msg("find_links not set in context - defaulting to enabled")
		} else if findLinks, ok := findLinksVal.(bool); ok && !findLinks {
			log.Debug().
				Str("url", e.Request.URL.String()).
				Bool("find_links", findLinks).
				Msg("Link extraction disabled for this request")
			return
		}

		href := strings.TrimSpace(e.Attr("href"))
		
		// Skip empty hrefs and fragments
		if href == "" || href == "#" {
			return
		}
		
		// Skip non-navigation links
		if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
			return
		}

		// Normalise URL (absolute)
		var u string
		if strings.HasPrefix(href, "?") {
			// It's a query-only link, so resolve it against the base URL without its own query
			base := e.Request.URL
			base.RawQuery = ""
			u = base.String() + href
		} else {
			u = e.Request.AbsoluteURL(href)
		}

		log.Debug().
			Str("href", href).
			Str("absolute_url", u).
			Str("from_url", e.Request.URL.String()).
			Msg("Colly found link")

		// Append to result.Links via context
		if r, ok := e.Request.Ctx.GetAny("result").(*CrawlResult); ok {
			r.Links = append(r.Links, u)
			log.Debug().
				Str("url", e.Request.URL.String()).
				Int("links_count", len(r.Links)).
				Msg("Added link to result")
		} else {
			log.Debug().
				Str("url", e.Request.URL.String()).
				Msg("No result context - not collecting links")
		}
	})
	
	// Set up timing and result collection
	collyClone.OnRequest(func(r *colly.Request) {
		r.Ctx.Put("result", res)
		r.Ctx.Put("start_time", start)
		r.Ctx.Put("find_links", findLinks)
	})
	
	// Handle response - collect cache headers, status, timing
	collyClone.OnResponse(func(r *colly.Response) {
		startTime := r.Ctx.GetAny("start_time").(time.Time)
		result := r.Ctx.GetAny("result").(*CrawlResult)
		
		// Calculate response time
		result.ResponseTime = time.Since(startTime).Milliseconds()
		result.StatusCode = r.StatusCode
		result.ContentType = r.Headers.Get("Content-Type")
		
		// Log comprehensive Cloudflare headers for analysis
		cfCacheStatus := r.Headers.Get("CF-Cache-Status")
		cfRay := r.Headers.Get("CF-Ray")
		cfDatacenter := r.Headers.Get("CF-IPCountry")
		cfConnectingIP := r.Headers.Get("CF-Connecting-IP")
		cfVisitor := r.Headers.Get("CF-Visitor")
		
		log.Debug().
			Str("url", r.Request.URL.String()).
			Str("cf_cache_status", cfCacheStatus).
			Str("cf_ray", cfRay).
			Str("cf_datacenter", cfDatacenter).
			Str("cf_connecting_ip", cfConnectingIP).
			Str("cf_visitor", cfVisitor).
			Int64("response_time_ms", result.ResponseTime).
			Msg("Cloudflare headers analysis")
		
		// Check for cache status headers from different CDNs
		// Cloudflare
		if cacheStatus := r.Headers.Get("CF-Cache-Status"); cacheStatus != "" {
			result.CacheStatus = cacheStatus
		}
		// Fastly
		if cacheStatus := r.Headers.Get("X-Cache"); cacheStatus != "" && result.CacheStatus == "" {
			result.CacheStatus = cacheStatus
		}
		// Akamai
		if cacheStatus := r.Headers.Get("X-Cache-Remote"); cacheStatus != "" && result.CacheStatus == "" {
			result.CacheStatus = cacheStatus
		}
		// Vercel
		if cacheStatus := r.Headers.Get("x-vercel-cache"); cacheStatus != "" && result.CacheStatus == "" {
			result.CacheStatus = cacheStatus
		}
		// Standard Cache-Status header (newer standardized approach)
		if cacheStatus := r.Headers.Get("Cache-Status"); cacheStatus != "" && result.CacheStatus == "" {
			result.CacheStatus = cacheStatus
		}
		// Varnish (the presence of X-Varnish indicates it was processed by Varnish)
		if varnishID := r.Headers.Get("X-Varnish"); varnishID != "" && result.CacheStatus == "" {
			if strings.Contains(varnishID, " ") {
				result.CacheStatus = "HIT" // Multiple IDs indicate a cache hit
			} else {
				result.CacheStatus = "MISS" // Single ID indicates a cache miss
			}
		}
		
		// Set error for non-2xx status codes (to match test expectations)
		if r.StatusCode < 200 || r.StatusCode >= 300 {
			result.Error = fmt.Sprintf("non-success status code: %d", r.StatusCode)
		}
	})
	
	// Handle errors
	collyClone.OnError(func(r *colly.Response, err error) {
		result := r.Ctx.GetAny("result").(*CrawlResult)
		result.Error = err.Error()
		
		if r != nil {
			startTime := r.Ctx.GetAny("start_time").(time.Time)
			result.ResponseTime = time.Since(startTime).Milliseconds()
			result.StatusCode = r.StatusCode
		}
		
		log.Error().
			Err(err).
			Str("url", targetURL).
			Dur("duration_ms", time.Duration(result.ResponseTime)*time.Millisecond).
			Msg("URL warming failed")
	})
	
	// Set up context cancellation handling
	done := make(chan error, 1)
	
	// Visit the URL with Colly in a goroutine to support context cancellation
	go func() {
		visitErr := collyClone.Visit(targetURL)
		if visitErr != nil {
			done <- visitErr
			return
		}
		// Wait for async requests to complete
		collyClone.Wait()
		done <- nil
	}()
	
	// Wait for either completion or context cancellation
	select {
	case err = <-done:
		if err != nil {
			res.Error = err.Error()
			log.Error().
				Err(err).
				Str("url", targetURL).
				Msg("Colly visit failed")
			return res, err
		}
	case <-ctx.Done():
		res.Error = ctx.Err().Error()
		log.Error().
			Err(ctx.Err()).
			Str("url", targetURL).
			Msg("URL warming cancelled due to context")
		return res, ctx.Err()
	}

	// Log results and return error if needed
	if res.Error != "" {
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			log.Warn().
				Int("status", res.StatusCode).
				Str("url", targetURL).
				Str("error", res.Error).
				Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
				Msg("URL warming returned non-success status")
		} else {
			log.Error().
				Str("url", targetURL).
				Str("error", res.Error).
				Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
				Msg("URL warming failed")
		}
		return res, fmt.Errorf("%s", res.Error)
	}

	// Perform cache warming if first request was a MISS
	if shouldMakeSecondRequest(res.CacheStatus) {
		// Dynamic delay based on initial response time (1.5x with bounds)
		delayMs := int(float64(res.ResponseTime) * 1.5)
		if delayMs < 2000 {
			delayMs = 2000 // Minimum 2 seconds
		}
		if delayMs > 30000 {
			delayMs = 30000 // Maximum 30 seconds
		}
		
		log.Debug().
			Str("url", targetURL).
			Str("cache_status", res.CacheStatus).
			Int64("initial_response_time", res.ResponseTime).
			Int("calculated_delay_ms", delayMs).
			Msg("Cache MISS detected, using dynamic delay based on initial response time")
		
		// Wait for initial delay to allow CDN to process and cache
		select {
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
			// Continue with cache check loop
		case <-ctx.Done():
			// Context cancelled during wait
			log.Debug().Str("url", targetURL).Msg("Cache warming cancelled during initial delay")
			return res, nil // First request was successful, return that
		}
		
		// Check cache status with HEAD requests in a loop
		maxChecks := 5
		checkDelay := 2000 // 2 seconds between checks
		cacheHit := false
		
		for i := 0; i < maxChecks; i++ {
			// Check cache status with HEAD request
			cacheStatus, err := c.CheckCacheStatus(ctx, targetURL)
			if err != nil {
				log.Warn().
					Err(err).
					Str("url", targetURL).
					Int("check_attempt", i+1).
					Msg("Failed to check cache status")
			} else {
				log.Debug().
					Str("url", targetURL).
					Str("cache_status", cacheStatus).
					Int("check_attempt", i+1).
					Msg("Cache status check")
				
				// If cache is now HIT, we can proceed with second request
				if cacheStatus == "HIT" || cacheStatus == "STALE" || cacheStatus == "REVALIDATED" {
					cacheHit = true
					break
				}
			}
			
			// If not the last check, wait before next attempt
			if i < maxChecks-1 {
				select {
				case <-time.After(time.Duration(checkDelay) * time.Millisecond):
					// Continue to next check
				case <-ctx.Done():
					log.Debug().Str("url", targetURL).Msg("Cache warming cancelled during check loop")
					return res, nil
				}
			}
		}
		
		// Log whether cache became available
		if cacheHit {
			log.Debug().
				Str("url", targetURL).
				Msg("Cache is now available, proceeding with second request")
		} else {
			log.Warn().
				Str("url", targetURL).
				Int("max_checks", maxChecks).
				Msg("Cache did not become available after maximum checks")
		}
		
		// Perform second request to measure cached response time
		secondResult, err := c.makeSecondRequest(ctx, targetURL)
		if err != nil {
			log.Warn().
				Err(err).
				Str("url", targetURL).
				Msg("Second request failed, but first request succeeded")
			// Don't return error - first request was successful
		} else {
			res.SecondResponseTime = secondResult.ResponseTime
			res.SecondCacheStatus = secondResult.CacheStatus
			
			// Calculate improvement ratio for pattern analysis
			improvementRatio := float64(res.ResponseTime) / float64(res.SecondResponseTime)
			
			log.Debug().
				Str("url", targetURL).
				Str("first_cache_status", res.CacheStatus).
				Str("second_cache_status", res.SecondCacheStatus).
				Int64("first_response_time", res.ResponseTime).
				Int64("second_response_time", res.SecondResponseTime).
				Int("initial_delay_ms", delayMs).
				Float64("improvement_ratio", improvementRatio).
				Bool("cache_hit_before_second", cacheHit).
				Msg("Cache warming analysis - pattern data")
		}
	}

	log.Debug().
		Int("status", res.StatusCode).
		Str("url", targetURL).
		Str("cache_status", res.CacheStatus).
		Int("links_found", len(res.Links)).
		Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
		Msg("URL warming completed successfully")

	return res, nil
}

// shouldMakeSecondRequest determines if we should make a second request for cache warming
func shouldMakeSecondRequest(cacheStatus string) bool {
	// Make second request for cache misses and bypasses
	// Don't make second request for hits, expired, stale, etc.
	switch strings.ToUpper(cacheStatus) {
	case "MISS", "BYPASS", "EXPIRED":
		return true
	default:
		return false
	}
}

// makeSecondRequest performs a second request to verify cache warming
// Reuses the main WarmURL logic but disables link extraction
func (c *Crawler) makeSecondRequest(ctx context.Context, targetURL string) (*CrawlResult, error) {
	// Reuse the main WarmURL method but disable link extraction
	return c.WarmURL(ctx, targetURL, false)
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

// Config returns the Crawler's configuration.
func (c *Crawler) Config() *Config {
	return c.config
}

