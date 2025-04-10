package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// DiscoverSitemaps attempts to find sitemaps for a domain by checking common locations
func (c *Crawler) DiscoverSitemaps(ctx context.Context, domain string) ([]string, error) {
	urls := []string{
		"https://" + domain + "/sitemap.xml",
		"https://" + domain + "/sitemap_index.xml",
		"https://www." + domain + "/sitemap.xml",
		"https://www." + domain + "/sitemap_index.xml",
	}

	var foundSitemaps []string
	client := &http.Client{Timeout: 10 * time.Second}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
		if err != nil {
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			foundSitemaps = append(foundSitemaps, url)
		}
	}

	if len(foundSitemaps) == 0 {
		// Try finding sitemaps in robots.txt
		robotsURL := "https://" + domain + "/robots.txt"
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
							foundSitemaps = append(foundSitemaps, sitemapURL)
						}
					}
				}
			}
		}
	}

	return foundSitemaps, nil
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
		section := content[startTagIdx:endTagIdx+len(endTag)]
		
		// Find the URL in the section
		locStartIdx := strings.Index(section, locStartTag)
		if locStartIdx != -1 {
			locEndIdx := strings.Index(section[locStartIdx:], locEndTag)
			if locEndIdx != -1 {
				locEndIdx += locStartIdx
				
				// Extract the URL
				url := section[locStartIdx+len(locStartTag):locEndIdx]
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
