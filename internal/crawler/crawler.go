package crawler

import (
	"bytes"
	"context"
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
		log.Debug().
			Bool("find_links", config.FindLinks).
			Msg("Registering Colly link extractor")

		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			href := e.Attr("href")
			// Normalize URL (absolute)
			u := e.Request.AbsoluteURL(href)

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
				log.Warn().
					Str("url", e.Request.URL.String()).
					Msg("Could not get result from Colly context")
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
		Msg("Starting URL warming")

	// Single HTTP request for both cache warming and link extraction
	client := &http.Client{Timeout: c.config.DefaultTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		res.Error = err.Error()
		res.ResponseTime = time.Since(start).Milliseconds()
		return res, err
	}

	// Set a specific User-Agent to identify our crawler
	req.Header.Set("User-Agent", "Blue-Banded-Bee-Cache-Warmer/1.0")

	resp, err := client.Do(req)
	res.ResponseTime = time.Since(start).Milliseconds()
	if err != nil {
		log.Error().
			Err(err).
			Str("url", targetURL).
			Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
			Msg("URL warming failed")
		res.Error = err.Error()
		return res, err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", targetURL).
			Msg("Failed to read response body")
		res.Error = fmt.Sprintf("failed to read response body: %s", err.Error())
		return res, err
	}

	res.StatusCode = resp.StatusCode

	// Check for cache status headers from different CDNs
	// Cloudflare
	if cacheStatus := resp.Header.Get("CF-Cache-Status"); cacheStatus != "" {
		res.CacheStatus = cacheStatus
	}
	// Fastly
	if cacheStatus := resp.Header.Get("X-Cache"); cacheStatus != "" && res.CacheStatus == "" {
		res.CacheStatus = cacheStatus
	}
	// Akamai
	if cacheStatus := resp.Header.Get("X-Cache-Remote"); cacheStatus != "" && res.CacheStatus == "" {
		res.CacheStatus = cacheStatus
	}
	// Vercel
	if cacheStatus := resp.Header.Get("x-vercel-cache"); cacheStatus != "" && res.CacheStatus == "" {
		res.CacheStatus = cacheStatus
	}
	// Standard Cache-Status header (newer standardized approach)
	if cacheStatus := resp.Header.Get("Cache-Status"); cacheStatus != "" && res.CacheStatus == "" {
		res.CacheStatus = cacheStatus
	}
	// Varnish (the presence of X-Varnish indicates it was processed by Varnish)
	if varnishID := resp.Header.Get("X-Varnish"); varnishID != "" && res.CacheStatus == "" {
		if strings.Contains(varnishID, " ") {
			res.CacheStatus = "HIT" // Multiple IDs indicate a cache hit
		} else {
			res.CacheStatus = "MISS" // Single ID indicates a cache miss
		}
	}

	res.ContentType = resp.Header.Get("Content-Type")

	// Extract links only if requested
	if findLinks {
		res.Links = extractLinks(bodyBytes, targetURL)
		log.Debug().
			Str("url", targetURL).
			Int("links_found", len(res.Links)).
			Msg("Link extraction completed")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Error = fmt.Sprintf("non-success status code: %d", resp.StatusCode)
		log.Warn().
			Int("status", resp.StatusCode).
			Str("url", targetURL).
			Dur("duration_ms", time.Duration(res.ResponseTime)*time.Millisecond).
			Msg("URL warming returned non-success status")
	} else {
		log.Debug().
			Int("status", resp.StatusCode).
			Str("url", targetURL).
			Str("cache_status", res.CacheStatus).
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

// extractLinks parses HTML body and returns all anchor hrefs as absolute URLs
func extractLinks(body []byte, base string) []string {
	var links []string
	baseURL, err := url.Parse(base)
	if err != nil {
		log.Error().
			Err(err).
			Str("base_url", base).
			Msg("Failed to parse base URL for link extraction")
		return links
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		log.Error().
			Err(err).
			Str("base_url", base).
			Msg("Failed to parse HTML for link extraction")
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

						// Log every 10th link to avoid excessive logging
						if len(links)%10 == 0 {
							log.Debug().
								Str("base_url", base).
								Int("links_found", len(links)).
								Msg("Extracting links from HTML")
						}
					} else {
						log.Debug().
							Err(err).
							Str("href", attr.Val).
							Msg("Failed to parse link URL")
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	log.Debug().
		Str("base_url", base).
		Int("total_links_found", len(links)).
		Msg("HTML link extraction completed")

	return links
}

// Config returns the Crawler's configuration.
func (c *Crawler) Config() *Config {
	return c.config
}
