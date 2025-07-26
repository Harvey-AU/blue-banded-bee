package util

import (
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// NormaliseDomain removes http/https prefix and www. from domain
func NormaliseDomain(domain string) string {
	// Remove http:// or https:// prefix if present
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")

	// Remove www. prefix if present
	domain = strings.TrimPrefix(domain, "www.")

	// Remove trailing slash if present
	domain = strings.TrimSuffix(domain, "/")

	return domain
}

// NormaliseURL ensures a URL has proper https:// scheme and validates format
func NormaliseURL(rawURL string) string {
	// Clean up the URL by trimming spaces
	rawURL = strings.TrimSpace(rawURL)

	// Skip empty URLs
	if rawURL == "" {
		return ""
	}

	// Convert http:// to https://
	if strings.HasPrefix(rawURL, "http://") {
		rawURL = strings.Replace(rawURL, "http://", "https://", 1)
	}

	// Add https:// prefix if missing
	if !strings.HasPrefix(rawURL, "https://") {
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

// ExtractPathFromURL extracts just the path component from a full URL
func ExtractPathFromURL(fullURL string) string {
	// Remove any protocol and domain to get just the path
	path := fullURL
	// Strip common prefixes
	path = strings.TrimPrefix(path, "http://")
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "www.")

	// Find the first slash after the domain name
	domainEnd := strings.Index(path, "/")
	if domainEnd != -1 {
		// Extract just the path part
		path = path[domainEnd:]
	} else {
		// If no path found, use root path
		path = "/"
	}

	return path
}

// ConstructURL builds a proper URL from domain and path components
func ConstructURL(domain, path string) string {
	// Normalise the domain
	normalisedDomain := NormaliseDomain(domain)

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Construct the full URL
	return "https://" + normalisedDomain + path
}
