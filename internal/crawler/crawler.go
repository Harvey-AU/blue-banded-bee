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

	// Add this to capture requests and responses
	c.OnRequest(func(r *colly.Request) {
		log.Debug().
			Str("url", r.URL.String()).
			Msg("Crawler sending request")
	})

	// Always register link extractor - we'll control it via context
	log.Debug().
		Msg("Registering Colly link extractor")

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// Only extract links that are likely visible and clickable to users
		if !isLinkVisibleAndClickable(e) {
			log.Debug().
				Str("href", e.Attr("href")).
				Msg("Skipping hidden/non-clickable link")
			return
		}

		href := e.Attr("href")
		// Normalise URL (absolute)
		u := e.Request.AbsoluteURL(href)

		log.Debug().
			Str("href", href).
			Str("absolute_url", u).
			Str("from_url", e.Request.URL.String()).
			Msg("Colly found clickable link")

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
	} else {
		log.Debug().
			Int("status", res.StatusCode).
			Str("url", targetURL).
			Str("cache_status", res.CacheStatus).
			Int("links_found", len(res.Links)).
			Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
			Msg("URL warming completed successfully")
	}

	return res, nil
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

// isLinkVisibleAndClickable determines if a link element is likely visible and clickable to users
// This helps filter out hidden navigation, framework-generated links, and other non-user-facing links
func isLinkVisibleAndClickable(e *colly.HTMLElement) bool {
	// Skip links with no href or empty href
	href := strings.TrimSpace(e.Attr("href"))
	if href == "" || href == "#" {
		return false
	}
	
	// Skip javascript: and mailto: links (not page navigation)
	if strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return false
	}
	
	// Check for common "hidden" indicators in CSS classes or attributes
	class := strings.ToLower(e.Attr("class"))
	style := strings.ToLower(e.Attr("style"))
	id := strings.ToLower(e.Attr("id"))
	
	// Skip links that are likely hidden or for accessibility
	hiddenIndicators := []string{
		"hidden", "invisible", "screen-reader", "sr-only", "visually-hidden",
		"skip-link", "skip-nav", "offscreen", "display:none", "visibility:hidden",
		"opacity:0", "w-condition-invisible", // Webflow hidden condition
	}
	
	for _, indicator := range hiddenIndicators {
		if strings.Contains(class, indicator) || strings.Contains(style, indicator) || strings.Contains(id, indicator) {
			log.Debug().
				Str("href", href).
				Str("reason", "hidden_indicator").
				Str("indicator", indicator).
				Msg("Link appears hidden")
			return false
		}
	}
	
	// Check if link has visible text content
	linkText := strings.TrimSpace(e.Text)
	if linkText == "" {
		// Links with no text might be purely decorative or hidden
		// But allow them if they have aria-label (could be icon links)
		ariaLabel := strings.TrimSpace(e.Attr("aria-label"))
		if ariaLabel == "" {
			log.Debug().
				Str("href", href).
				Str("reason", "no_text_content").
				Msg("Link has no visible text or aria-label")
			return false
		}
	}
	
	// Skip links that are likely framework/CMS generated pagination with complex parameters
	// Allow simple pagination (page=1, page=2) but skip complex multi-parameter combinations
	if strings.Contains(href, "?") {
		queryParts := strings.Split(strings.Split(href, "?")[1], "&")
		pageParams := 0
		for _, part := range queryParts {
			if strings.Contains(part, "_page=") || strings.Contains(part, "page=") {
				pageParams++
			}
		}
		// If there are multiple pagination parameters, likely framework-generated
		if pageParams > 2 {
			log.Debug().
				Str("href", href).
				Str("reason", "complex_pagination").
				Int("page_params", pageParams).
				Msg("Link appears to be complex pagination")
			return false
		}
	}
	
	// If we get here, the link appears to be visible and clickable
	return true
}
