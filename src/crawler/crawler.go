package crawler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/rs/zerolog/log"
)

type Crawler struct {
	config *Config
	colly  *colly.Collector
}

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

func (c *Crawler) WarmURL(ctx context.Context, targetURL string) (*CrawlResult, error) {
	start := time.Now()
	result := &CrawlResult{
		URL:       targetURL,
		Timestamp: time.Now().Unix(),
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Additional validation
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		err := fmt.Errorf("invalid URL format: %s", targetURL)
		result.Error = err.Error()
		return result, err
	}

	c.colly.OnResponse(func(r *colly.Response) {
		result.StatusCode = r.StatusCode
		result.CacheStatus = r.Headers.Get("CF-Cache-Status")

		// Treat non-2xx status codes as errors
		if r.StatusCode < 200 || r.StatusCode >= 300 {
			result.Error = fmt.Sprintf("HTTP %d: Non-successful status code", r.StatusCode)
		}
	})

	c.colly.OnError(func(r *colly.Response, err error) {
		if r != nil {
			result.StatusCode = r.StatusCode
		}
		result.Error = err.Error()
	})

	err = c.colly.Visit(targetURL)
	if err != nil {
		log.Error().Err(err).Str("url", targetURL).Msg("Failed to crawl URL")
		result.Error = err.Error()
		return result, err
	}

	c.colly.Wait()
	result.ResponseTime = time.Since(start).Milliseconds()

	// Return error if we got a non-2xx status code or any other error
	if result.Error != "" {
		return result, errors.New(result.Error)
	}

	return result, nil
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
