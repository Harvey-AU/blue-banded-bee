package util

import (
	"fmt"
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

// ValidateDomain checks if a domain string is a valid domain format.
// Returns an error describing why the domain is invalid, or nil if valid.
func ValidateDomain(domain string) error {
	// Normalise first
	domain = NormaliseDomain(domain)

	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Must contain at least one dot (for TLD)
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("domain must contain a TLD (e.g., .com, .co.uk)")
	}

	// Split into parts and validate each
	parts := strings.Split(domain, ".")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("domain contains empty segment")
		}

		// Check for valid characters (alphanumeric and hyphens)
		for _, c := range part {
			isLower := c >= 'a' && c <= 'z'
			isUpper := c >= 'A' && c <= 'Z'
			isDigit := c >= '0' && c <= '9'
			isHyphen := c == '-'
			if !isLower && !isUpper && !isDigit && !isHyphen {
				return fmt.Errorf("domain contains invalid character: %c", c)
			}
		}

		// Cannot start or end with hyphen
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("domain segment cannot start or end with hyphen")
		}
	}

	// TLD must be at least 2 characters
	tld := parts[len(parts)-1]
	if len(tld) < 2 {
		return fmt.Errorf("TLD must be at least 2 characters")
	}

	// Block localhost and common internal hostnames
	lowerDomain := strings.ToLower(domain)
	blockedDomains := []string{"localhost", "localhost.localdomain", "local", "internal"}
	for _, blocked := range blockedDomains {
		if lowerDomain == blocked || strings.HasSuffix(lowerDomain, "."+blocked) {
			return fmt.Errorf("domain %q is not allowed", domain)
		}
	}

	return nil
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

// normaliseHostPort removes default ports (80 for HTTP, 443 for HTTPS) from host.
func normaliseHostPort(host, scheme string) string {
	if scheme == "http" && strings.HasSuffix(host, ":80") {
		return strings.TrimSuffix(host, ":80")
	}
	if scheme == "https" && strings.HasSuffix(host, ":443") {
		return strings.TrimSuffix(host, ":443")
	}
	return host
}

// IsSignificantRedirect checks if a redirect URL is meaningfully different from the original.
// Only the host and path are compared; query parameters and fragments are ignored.
// Returns false for trivial redirects like:
//   - HTTP to HTTPS on same domain/path
//   - www to non-www (or vice versa) on same path
//   - Trailing slash differences
//   - Default port differences (e.g., :443 for HTTPS, :80 for HTTP)
//
// Returns true for redirects to different domains or different paths.
func IsSignificantRedirect(originalURL, redirectURL string) bool {
	if redirectURL == "" {
		return false
	}

	// Parse both URLs
	origParsed, origErr := url.Parse(originalURL)
	redirParsed, redirErr := url.Parse(redirectURL)

	if origErr != nil || redirErr != nil {
		// If we can't parse, assume it's significant
		return true
	}

	// Normalise hosts (remove www prefix, lowercase, strip default ports)
	origHost := normaliseHostPort(origParsed.Host, origParsed.Scheme)
	origHost = strings.ToLower(strings.TrimPrefix(origHost, "www."))
	redirHost := normaliseHostPort(redirParsed.Host, redirParsed.Scheme)
	redirHost = strings.ToLower(strings.TrimPrefix(redirHost, "www."))

	// Different domain = significant
	if origHost != redirHost {
		return true
	}

	// Normalise paths (ensure leading slash, remove trailing slash for comparison)
	origPath := origParsed.Path
	redirPath := redirParsed.Path

	if origPath == "" {
		origPath = "/"
	}
	if redirPath == "" {
		redirPath = "/"
	}

	// Remove trailing slashes for comparison (but "/" stays as "/")
	if len(origPath) > 1 {
		origPath = strings.TrimSuffix(origPath, "/")
	}
	if len(redirPath) > 1 {
		redirPath = strings.TrimSuffix(redirPath, "/")
	}

	// Different path = significant
	if origPath != redirPath {
		return true
	}

	// Same domain and path - not significant (likely HTTP→HTTPS or www→non-www)
	return false
}
