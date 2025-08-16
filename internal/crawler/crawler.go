package crawler

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog/log"
)

// Crawler represents a URL crawler with configuration and metrics
type Crawler struct {
	config     *Config
	colly      *colly.Collector
	id         string    // Add an ID field to identify each crawler instance
	metricsMap *sync.Map // Shared metrics storage for the transport
}

// GetUserAgent returns the user agent string for this crawler
func (c *Crawler) GetUserAgent() string {
	return c.config.UserAgent
}

// tracingRoundTripper captures HTTP trace metrics for each request
type tracingRoundTripper struct {
	transport  http.RoundTripper
	metricsMap *sync.Map // Maps URL -> PerformanceMetrics
}

// RoundTrip implements the http.RoundTripper interface with httptrace instrumentation
func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create performance metrics for this request
	metrics := &PerformanceMetrics{}

	// Create trace with callbacks that populate metrics
	var dnsStartTime, connectStartTime, tlsStartTime, requestStartTime time.Time
	requestStartTime = time.Now()

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStartTime = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if !dnsStartTime.IsZero() {
				metrics.DNSLookupTime = time.Since(dnsStartTime).Milliseconds()
			}
		},
		ConnectStart: func(network, addr string) {
			connectStartTime = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			if err == nil && !connectStartTime.IsZero() {
				metrics.TCPConnectionTime = time.Since(connectStartTime).Milliseconds()
			}
		},
		TLSHandshakeStart: func() {
			tlsStartTime = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if err == nil && !tlsStartTime.IsZero() {
				metrics.TLSHandshakeTime = time.Since(tlsStartTime).Milliseconds()
			}
		},
		GotFirstResponseByte: func() {
			metrics.TTFB = time.Since(requestStartTime).Milliseconds()
		},
	}

	// Store metrics for this URL (will be retrieved in OnResponse)
	t.metricsMap.Store(req.URL.String(), metrics)

	// Attach trace to request context
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Perform the request
	return t.transport.RoundTrip(req)
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

	// Create metrics map for this crawler instance
	metricsMap := &sync.Map{}

	// Set up base transport
	baseTransport := &http.Transport{
		MaxIdleConnsPerHost: 25,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true,
		ForceAttemptHTTP2:   true,
	}

	// Wrap the base transport with our custom tracing transport
	tracingTransport := &tracingRoundTripper{
		transport:  baseTransport,
		metricsMap: metricsMap,
	}

	// Set HTTP client with tracing transport and proper timeout
	httpClient := &http.Client{
		Timeout:   config.DefaultTimeout,
		Transport: tracingTransport,
	}
	c.SetClient(httpClient)

	// Add browser-like headers to avoid blocking
	c.OnRequest(func(r *colly.Request) {
		// Set browser-like headers
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Accept-Encoding", "gzip, deflate, br")

		log.Debug().
			Str("url", r.URL.String()).
			Msg("Crawler sending request")
	})

	// Note: OnHTML handler will be registered on the clone in WarmURL to ensure proper context access

	return &Crawler{
		config:     config,
		colly:      c,
		id:         crawlerID,
		metricsMap: metricsMap,
	}
}

// validateCrawlRequest validates the crawl request parameters and URL format
func validateCrawlRequest(ctx context.Context, targetURL string) (*url.URL, *CrawlResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		res := &CrawlResult{URL: targetURL, Timestamp: time.Now().Unix(), Error: err.Error()}
		return nil, res, err
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		err := fmt.Errorf("invalid URL format: %s", targetURL)
		res := &CrawlResult{URL: targetURL, Timestamp: time.Now().Unix(), Error: err.Error()}
		return nil, res, err
	}

	return parsed, nil, nil
}

// WarmURL performs a crawl of the specified URL and returns the result.
// It respects context cancellation, enforces timeout, and treats non-2xx statuses as errors.
func (c *Crawler) WarmURL(ctx context.Context, targetURL string, findLinks bool) (*CrawlResult, error) {
	// Validate the crawl request
	_, errorResult, err := validateCrawlRequest(ctx, targetURL)
	if err != nil {
		if errorResult != nil {
			return errorResult, err
		}
		return nil, err
	}

	start := time.Now()
	res := &CrawlResult{
		URL:       targetURL,
		Timestamp: start.Unix(),
		Links:     make(map[string][]string),
	}

	log.Debug().
		Str("url", targetURL).
		Bool("find_links", findLinks).
		Msg("Starting URL warming with Colly")

	// Use Colly for everything - single request handles cache warming and link extraction
	collyClone := c.colly.Clone()

	// Register link extractor on the clone to ensure proper context access
	log.Debug().
		Msg("Registering Colly link extractor on clone")

	collyClone.OnHTML("html", func(e *colly.HTMLElement) {
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

		result, ok := e.Request.Ctx.GetAny("result").(*CrawlResult)
		if !ok {
			log.Debug().
				Str("url", e.Request.URL.String()).
				Msg("No result context - not collecting links")
			return
		}

		extractLinks := func(selection *goquery.Selection, category string) {
			selection.Find("a[href]").Each(func(i int, s *goquery.Selection) {
				href := strings.TrimSpace(s.AttrOr("href", ""))
				if isElementHidden(s) || href == "" || href == "#" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
					return
				}

				var u string
				if strings.HasPrefix(href, "?") {
					base := e.Request.URL
					base.RawQuery = ""
					u = base.String() + href
				} else {
					u = e.Request.AbsoluteURL(href)
				}

				result.Links[category] = append(result.Links[category], u)
			})
		}

		// Extract from header and footer first
		extractLinks(e.DOM.Find("header"), "header")
		extractLinks(e.DOM.Find("footer"), "footer")

		// Remove header and footer to get body links
		e.DOM.Find("header").Remove()
		e.DOM.Find("footer").Remove()

		// Extract remaining links as "body"
		extractLinks(e.DOM, "body")

		log.Debug().
			Str("url", e.Request.URL.String()).
			Int("header_links", len(result.Links["header"])).
			Int("footer_links", len(result.Links["footer"])).
			Int("body_links", len(result.Links["body"])).
			Msg("Categorized links from page")
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

		// Retrieve performance metrics from the metrics map
		if metricsVal, ok := c.metricsMap.LoadAndDelete(r.Request.URL.String()); ok {
			performanceMetrics := metricsVal.(*PerformanceMetrics)
			// Content transfer time is total response time minus TTFB
			if performanceMetrics.TTFB > 0 {
				performanceMetrics.ContentTransferTime = time.Since(startTime).Milliseconds() - performanceMetrics.TTFB
			}
			result.Performance = *performanceMetrics
		}

		// Calculate response time
		result.ResponseTime = time.Since(startTime).Milliseconds()
		result.StatusCode = r.StatusCode
		result.ContentType = r.Headers.Get("Content-Type")
		result.ContentLength = int64(len(r.Body))
		result.Headers = r.Headers.Clone()
		result.RedirectURL = r.Request.URL.String()

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
		maxChecks := 10
		checkDelay := 2000 // Initial 2 seconds delay
		cacheHit := false

		for i := 0; i < maxChecks; i++ {
			// Check cache status with HEAD request
			cacheStatus, err := c.CheckCacheStatus(ctx, targetURL)

			// Record the attempt
			attempt := CacheCheckAttempt{
				Attempt:     i + 1,
				CacheStatus: cacheStatus,
				Delay:       checkDelay,
			}
			res.CacheCheckAttempts = append(res.CacheCheckAttempts, attempt)

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
				// Increase delay for the next iteration
				checkDelay += 1000
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
			res.SecondContentLength = secondResult.ContentLength
			res.SecondHeaders = secondResult.Headers
			res.SecondPerformance = &secondResult.Performance

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
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

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

// isElementHidden checks if an element is hidden based on common inline styles,
// accessibility attributes, and conventional CSS classes.
// This is a best-effort check based on raw HTML attributes, as it does not
// evaluate external or internal CSS stylesheets.
func isElementHidden(s *goquery.Selection) bool {
	// Define the list of common hiding classes
	hidingClasses := []string{
		"hide",
		"hidden",
		"display-none",
		"d-none",
		"invisible",
		"is-hidden",
		"sr-only",
		"visually-hidden",
	}

	// Loop through the current element and all its parents up to the body
	for n := s; n.Length() > 0 && !n.Is("body"); n = n.Parent() {
		// 1. Check for explicit data attributes
		if _, exists := n.Attr("data-hidden"); exists {
			return true
		}
		if val, exists := n.Attr("data-visible"); exists && val == "false" {
			return true
		}

		// 2. Check for aria-hidden="true" attribute
		if ariaHidden, exists := n.Attr("aria-hidden"); exists && ariaHidden == "true" {
			return true
		}

		// 3. Check for inline style attributes
		if style, exists := n.Attr("style"); exists {
			if strings.Contains(style, "display: none") || strings.Contains(style, "visibility: hidden") {
				return true
			}
		}

		// 4. Check for common hiding classes
		for _, class := range hidingClasses {
			if n.HasClass(class) {
				return true
			}
		}
	}

	// No hiding attributes or classes were found
	return false
}
