package crawler

import (
	"context"
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

func (c *Crawler) WarmURL(ctx context.Context, url string) (*CrawlResult, error) {
	start := time.Now()
	result := &CrawlResult{
		URL:       url,
		Timestamp: time.Now().Unix(),
	}

	err := c.colly.Visit(url)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Failed to crawl URL")
		result.Error = err.Error()
		return result, err
	}

	result.ResponseTime = time.Since(start).Milliseconds()
	return result, nil
}