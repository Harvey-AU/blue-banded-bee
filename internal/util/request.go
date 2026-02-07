package util

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// RequestMeta contains metadata extracted from an HTTP request.
// Useful for audit logging and contextual email content.
type RequestMeta struct {
	IP        string
	UserAgent string
	Browser   string
	OS        string
	Device    string // e.g. "Chrome on macOS"
	Timestamp time.Time
}

// ExtractRequestMeta extracts client metadata from an HTTP request.
func ExtractRequestMeta(r *http.Request) *RequestMeta {
	m := &RequestMeta{
		IP:        GetClientIP(r),
		UserAgent: r.UserAgent(),
		Timestamp: time.Now(),
	}
	m.Browser, m.OS = parseUserAgent(m.UserAgent)
	if m.Browser != "" && m.OS != "" {
		m.Device = fmt.Sprintf("%s on %s", m.Browser, m.OS)
	} else if m.Browser != "" {
		m.Device = m.Browser
	} else if m.OS != "" {
		m.Device = m.OS
	}
	return m
}

// FormattedTimestamp returns the timestamp in a human-readable format.
func (m *RequestMeta) FormattedTimestamp() string {
	return m.Timestamp.UTC().Format("January 2, 2006")
}

// GetClientIP extracts the client IP address from a request,
// respecting proxy headers (X-Forwarded-For, X-Real-IP).
func GetClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// parseUserAgent extracts browser and OS from a user agent string.
func parseUserAgent(ua string) (browser, os string) {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "", ""
	}

	browser = parseBrowser(ua)
	os = parseOS(ua)
	return browser, os
}

func parseBrowser(ua string) string {
	// Order matters — check specific browsers before generic engines
	switch {
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge/"):
		return "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "Brave"):
		return "Brave"
	case strings.Contains(ua, "Vivaldi"):
		return "Vivaldi"
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium"):
		return "Chrome"
	case strings.Contains(ua, "Firefox/"):
		return "Firefox"
	case strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome"):
		return "Safari"
	default:
		return ""
	}
}

func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		return "iOS"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS"):
		return "macOS"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	default:
		return ""
	}
}
