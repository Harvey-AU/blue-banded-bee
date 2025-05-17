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

// normalizeDomain removes http/https prefix and www. from domain
func normalizeDomain(domain string) string {
	// Remove http:// or https:// prefix if present
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	
	// Remove www. prefix if present
	domain = strings.TrimPrefix(domain, "www.")
	
	// Remove trailing slash if present
	domain = strings.TrimSuffix(domain, "/")
	
	return domain
}

// DiscoverSitemaps attempts to find sitemaps for a domain by checking common locations
func (c *Crawler) DiscoverSitemaps(ctx context.Context, domain string) ([]string, error) {
	// Normalize the domain first to handle different input formats
	normalizedDomain := normalizeDomain(domain)
	log.Debug().
		Str("original_domain", domain).
		Str("normalized_domain", normalizedDomain).
		Msg("Starting sitemap discovery with normalized domain")
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
	robotsURL := "https://" + normalizedDomain + "/robots.txt"
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

	// Log if sitemaps were found in robots.txt
	if len(foundSitemaps) > 0 {
		log.Debug().
			Strs("sitemaps", foundSitemaps).
			Msg("Sitemaps found in robots.txt")
	} else {
		log.Debug().Msg("No sitemaps found in robots.txt")
	}

	// If no sitemaps found in robots.txt, check common locations
	if len(foundSitemaps) == 0 {
		commonPaths := []string{
			"https://" + normalizedDomain + "/sitemap.xml",
			"https://" + normalizedDomain + "/sitemap_index.xml",
		}

		// Check common locations concurrently with a timeout
		for _, sitemapURL := range commonPaths {
			log.Debug().Str("checking_sitemap_url", sitemapURL).Msg("Checking common sitemap location")
			req, err := http.NewRequestWithContext(ctx, "HEAD", sitemapURL, nil)
			if err != nil {
				log.Debug().Err(err).Str("url", sitemapURL).Msg("Error creating request for sitemap")
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Debug().Err(err).Str("url", sitemapURL).Msg("Error fetching sitemap")
				continue
			}
			
			resp.Body.Close()
			log.Debug().Str("url", sitemapURL).Int("status", resp.StatusCode).Msg("Sitemap check response")
			if resp.StatusCode == http.StatusOK {
				foundSitemaps = append(foundSitemaps, sitemapURL)
				log.Debug().Str("url", sitemapURL).Msg("Found sitemap at common location")
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

	// Log final result
	if len(uniqueSitemaps) > 0 {
		log.Debug().
			Strs("sitemaps", uniqueSitemaps).
			Int("count", len(uniqueSitemaps)).
			Msg("Found sitemaps for domain")
	} else {
		log.Debug().
			Str("domain", domain).
			Msg("No sitemaps found for domain")
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
			// Validate and normalize the child sitemap URL
			childSitemapURL = validateURL(childSitemapURL)
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
		
		// Validate and normalize all extracted URLs
		var validURLs []string
		for _, extractedURL := range extractedURLs {
			validURL := validateURL(extractedURL)
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

// validateURL ensures a URL is properly formatted with a scheme
func validateURL(rawURL string) string {
	// Clean up the URL by trimming spaces
	rawURL = strings.TrimSpace(rawURL)
	
	// Skip empty URLs
	if rawURL == "" {
		return ""
	}
	
	// Check if URL already has a scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		// Add https:// prefix if missing
		rawURL = "https://" + rawURL
	}
	
	// Validate URL format
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		log.Debug().Str("url", rawURL).Err(err).Msg("Invalid URL format")
		return ""
	}
	
	// Ensure no duplicate schemes (like https://http://example.com)
	hostPart := parsedURL.Host
	if strings.Contains(hostPart, "://") {
		log.Debug().Str("url", rawURL).Msg("URL contains embedded scheme in host part, fixing")
		// Extract the domain part after the embedded scheme
		parts := strings.SplitN(hostPart, "://", 2)
		if len(parts) == 2 {
			parsedURL.Host = parts[1]
			rawURL = parsedURL.String()
		}
	}
	
	return rawURL
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
