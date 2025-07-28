package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/util"
	"github.com/rs/zerolog/log"
)

// SitemapDiscoveryResult contains both sitemaps and robots.txt rules
type SitemapDiscoveryResult struct {
	Sitemaps    []string
	RobotsRules *RobotsRules
}

// DiscoverSitemapsAndRobots attempts to find sitemaps and parse robots.txt rules for a domain
func (c *Crawler) DiscoverSitemapsAndRobots(ctx context.Context, domain string) (*SitemapDiscoveryResult, error) {
	// Normalise the domain first to handle different input formats
	normalisedDomain := util.NormaliseDomain(domain)
	log.Debug().
		Str("original_domain", domain).
		Str("normalised_domain", normalisedDomain).
		Msg("Starting sitemap and robots.txt discovery")

	result := &SitemapDiscoveryResult{
		Sitemaps:    []string{},
		RobotsRules: &RobotsRules{}, // Default empty rules
	}

	// Parse robots.txt first - this gets us both sitemaps and crawl rules
	robotRules, err := ParseRobotsTxt(ctx, normalisedDomain, c.config.UserAgent)
	if err != nil {
		// Log error but don't fail - no robots.txt means no restrictions
		log.Debug().
			Err(err).
			Str("domain", normalisedDomain).
			Msg("Failed to parse robots.txt, proceeding with no restrictions")
	} else {
		result.RobotsRules = robotRules
		result.Sitemaps = robotRules.Sitemaps
	}

	// Log if sitemaps were found in robots.txt
	if len(result.Sitemaps) > 0 {
		log.Debug().
			Strs("sitemaps", result.Sitemaps).
			Msg("Sitemaps found in robots.txt")
	} else {
		log.Debug().Msg("No sitemaps found in robots.txt")
	}

	// If no sitemaps found in robots.txt, check common locations
	if len(result.Sitemaps) == 0 {
		commonPaths := []string{
			"https://" + normalisedDomain + "/sitemap.xml",
			"https://" + normalisedDomain + "/sitemap_index.xml",
		}

		// Create a client for checking common locations
		client := &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		// Check common locations concurrently with a timeout
		for _, sitemapURL := range commonPaths {
			log.Debug().Str("checking_sitemap_url", sitemapURL).Msg("Checking common sitemap location")
			req, err := http.NewRequestWithContext(ctx, "HEAD", sitemapURL, nil)
			if err != nil {
				log.Debug().Err(err).Str("url", sitemapURL).Msg("Error creating request for sitemap")
				continue
			}
			req.Header.Set("User-Agent", c.config.UserAgent)

			resp, err := client.Do(req)
			if err != nil {
				log.Debug().Err(err).Str("url", sitemapURL).Msg("Error fetching sitemap")
				continue
			}

			resp.Body.Close()
			log.Debug().Str("url", sitemapURL).Int("status", resp.StatusCode).Msg("Sitemap check response")
			if resp.StatusCode == http.StatusOK {
				result.Sitemaps = append(result.Sitemaps, sitemapURL)
				log.Debug().Str("url", sitemapURL).Msg("Found sitemap at common location")
			}
		}
	}

	// Deduplicate sitemaps
	seen := make(map[string]bool)
	var uniqueSitemaps []string
	for _, sitemap := range result.Sitemaps {
		if !seen[sitemap] {
			seen[sitemap] = true
			uniqueSitemaps = append(uniqueSitemaps, sitemap)
		}
	}
	result.Sitemaps = uniqueSitemaps

	// Log final result
	if len(result.Sitemaps) > 0 {
		log.Debug().
			Strs("sitemaps", result.Sitemaps).
			Int("count", len(result.Sitemaps)).
			Int("crawl_delay", result.RobotsRules.CrawlDelay).
			Int("disallow_patterns", len(result.RobotsRules.DisallowPatterns)).
			Msg("Found sitemaps and robots rules for domain")
	} else {
		log.Debug().
			Str("domain", domain).
			Int("crawl_delay", result.RobotsRules.CrawlDelay).
			Int("disallow_patterns", len(result.RobotsRules.DisallowPatterns)).
			Msg("No sitemaps found but got robots rules for domain")
	}

	return result, nil
}

// DiscoverSitemaps is a backward-compatible wrapper that only returns sitemaps
func (c *Crawler) DiscoverSitemaps(ctx context.Context, domain string) ([]string, error) {
	result, err := c.DiscoverSitemapsAndRobots(ctx, domain)
	if err != nil {
		return nil, err
	}
	return result.Sitemaps, nil
}

// Create proper sitemap structs
type SitemapIndex struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Sitemap struct {
	XMLName xml.Name `xml:"sitemap"`
	Loc     string   `xml:"loc"`
}

type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []URL    `xml:"url"`
}

type URL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
}

// ParseSitemap extracts URLs from a sitemap
func (c *Crawler) ParseSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	var urls []string

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch sitemap: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(body)

	// Log the content for debugging
	log.Debug().
		Str("url", sitemapURL).
		Int("content_length", len(content)).
		Str("content_sample", content[:min(100, len(content))]).
		Msg("Sitemap content received")

	// Check if it's a sitemap index
	if strings.Contains(content, "<sitemapindex") {
		// Extract sitemap URLs
		sitemapURLs := extractURLsFromXML(content, "<sitemap>", "</sitemap>", "<loc>", "</loc>")

		// Process each sitemap in the index
		for _, childSitemapURL := range sitemapURLs {
			// Validate and normalise the child sitemap URL
			childSitemapURL = util.NormaliseURL(childSitemapURL)
			if childSitemapURL == "" {
				log.Warn().Str("url", childSitemapURL).Msg("Invalid child sitemap URL, skipping")
				continue
			}

			childURLs, err := c.ParseSitemap(ctx, childSitemapURL)
			if err != nil {
				log.Warn().Err(err).Str("url", childSitemapURL).Msg("Failed to parse child sitemap")
				continue
			}
			urls = append(urls, childURLs...)
		}
	} else {
		// It's a regular sitemap
		extractedURLs := extractURLsFromXML(content, "<url>", "</url>", "<loc>", "</loc>")

		// Validate and normalise all extracted URLs
		var validURLs []string
		for _, extractedURL := range extractedURLs {
			validURL := util.NormaliseURL(extractedURL)
			if validURL != "" {
				validURLs = append(validURLs, validURL)
			} else {
				log.Debug().Str("invalid_url", extractedURL).Msg("Skipping invalid URL from sitemap")
			}
		}

		log.Debug().
			Str("sitemap_url", sitemapURL).
			Int("url_count", len(validURLs)).
			Msg("Extracted valid URLs from regular sitemap")
		urls = append(urls, validURLs...)
	}

	log.Debug().
		Str("sitemap_url", sitemapURL).
		Int("total_url_count", len(urls)).
		Msg("Finished parsing sitemap")

	return urls, nil
}

// Helper function to extract URLs from XML content
func extractURLsFromXML(content, startTag, endTag, locStartTag, locEndTag string) []string {
	var urls []string

	// Find all instances of the outer tag
	startIdx := 0
	for {
		startTagIdx := strings.Index(content[startIdx:], startTag)
		if startTagIdx == -1 {
			break
		}

		startTagIdx += startIdx
		endTagIdx := strings.Index(content[startTagIdx:], endTag)
		if endTagIdx == -1 {
			break
		}

		endTagIdx += startTagIdx

		// Extract the section between tags
		section := content[startTagIdx : endTagIdx+len(endTag)]

		// Find the URL in the section
		locStartIdx := strings.Index(section, locStartTag)
		if locStartIdx != -1 {
			locEndIdx := strings.Index(section[locStartIdx:], locEndTag)
			if locEndIdx != -1 {
				locEndIdx += locStartIdx

				// Extract the URL
				url := section[locStartIdx+len(locStartTag) : locEndIdx]
				url = strings.TrimSpace(url)

				if url != "" {
					urls = append(urls, url)
				}
			}
		}

		startIdx = endTagIdx + len(endTag)
	}

	return urls
}

// min returns the smaller of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FilterURLs filters URLs based on include/exclude patterns
func (c *Crawler) FilterURLs(urls []string, includePaths, excludePaths []string) []string {
	if len(includePaths) == 0 && len(excludePaths) == 0 {
		return urls
	}

	var filtered []string

	for _, url := range urls {
		// If include patterns exist, URL must match at least one
		includeMatch := len(includePaths) == 0
		for _, pattern := range includePaths {
			if strings.Contains(url, pattern) {
				includeMatch = true
				break
			}
		}

		if !includeMatch {
			continue
		}

		// If URL matches any exclude pattern, skip it
		excludeMatch := false
		for _, pattern := range excludePaths {
			if strings.Contains(url, pattern) {
				excludeMatch = true
				break
			}
		}

		if !excludeMatch {
			filtered = append(filtered, url)
		}
	}

	return filtered
}
