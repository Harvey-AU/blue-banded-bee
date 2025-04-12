package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// DiscoverSitemaps attempts to find sitemaps for a domain by checking common locations
func (c *Crawler) DiscoverSitemaps(ctx context.Context, domain string) ([]string, error) {
	// Create a client with shorter timeout and redirect handling
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Check robots.txt first as it's the most authoritative source
	robotsURL := "https://" + domain + "/robots.txt"
	var foundSitemaps []string

	// Try robots.txt first
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err == nil {
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			robotsTxt, err := io.ReadAll(resp.Body)
			if err == nil {
				lines := strings.Split(string(robotsTxt), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
						sitemapURL := strings.TrimSpace(line[8:])
						// Validate sitemap URL
						if _, err := url.Parse(sitemapURL); err == nil {
							foundSitemaps = append(foundSitemaps, sitemapURL)
						}
					}
				}
			}
		}
	}

	// If no sitemaps found in robots.txt, check common locations
	if len(foundSitemaps) == 0 {
		commonPaths := []string{
			"https://" + domain + "/sitemap.xml",
			"https://" + domain + "/sitemap_index.xml",
		}

		// Check common locations concurrently with a timeout
		for _, sitemapURL := range commonPaths {
			req, err := http.NewRequestWithContext(ctx, "HEAD", sitemapURL, nil)
			if err != nil {
				continue
			}

			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					foundSitemaps = append(foundSitemaps, sitemapURL)
				}
			}
		}
	}

	// Deduplicate sitemaps
	seen := make(map[string]bool)
	var uniqueSitemaps []string
	for _, sitemap := range foundSitemaps {
		if !seen[sitemap] {
			seen[sitemap] = true
			uniqueSitemaps = append(uniqueSitemaps, sitemap)
		}
	}

	return uniqueSitemaps, nil
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

	// Check if it's a sitemap index
	if strings.Contains(content, "<sitemapindex") {
		// Extract sitemap URLs
		sitemapURLs := extractURLsFromXML(content, "<sitemap>", "</sitemap>", "<loc>", "</loc>")

		// Process each sitemap in the index
		for _, childSitemapURL := range sitemapURLs {
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
		urls = append(urls, extractedURLs...)
	}

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

// Add a validateURL helper:
func validateURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Validate scheme and host
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid URL scheme: %s", parsed.Scheme)
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("missing host in URL")
	}

	return rawURL, nil
}

// Then use it for each URL before adding to the results
