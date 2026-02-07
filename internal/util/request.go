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
	City      string // From Cloudflare cf-ipcity header
	Region    string // From Cloudflare cf-region header
	Country   string // From Cloudflare cf-ipcountry header (ISO code)
	Timezone  string // From Cloudflare cf-timezone header
	Location  string // Formatted e.g. "Melbourne, Victoria, Australia"
	Timestamp time.Time
}

// ExtractRequestMeta extracts client metadata from an HTTP request.
// Location fields (City, Region, Country, Timezone) require the Cloudflare
// "Add visitor location headers" managed transform to be enabled.
func ExtractRequestMeta(r *http.Request) *RequestMeta {
	m := &RequestMeta{
		IP:        GetClientIP(r),
		UserAgent: r.UserAgent(),
		City:      r.Header.Get("cf-ipcity"),
		Region:    r.Header.Get("cf-region"),
		Country:   countryName(r.Header.Get("cf-ipcountry")),
		Timezone:  r.Header.Get("cf-timezone"),
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
	m.Location = buildLocation(m.City, m.Region, m.Country)
	return m
}

// buildLocation formats city, region, and country into a readable string.
// Deduplicates when values overlap (e.g. "Singapore, Singapore, Singapore" → "Singapore").
func buildLocation(city, region, country string) string {
	parts := make([]string, 0, 3)
	if city != "" {
		parts = append(parts, city)
	}
	if region != "" && region != city && region != country {
		parts = append(parts, region)
	}
	if country != "" && country != city {
		parts = append(parts, country)
	}
	return strings.Join(parts, ", ")
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

// countryName converts an ISO 3166-1 alpha-2 country code to a full name.
// Returns the code unchanged if not found in the lookup table.
func countryName(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return ""
	}
	if name, ok := countryNames[code]; ok {
		return name
	}
	return code
}

//nolint:gocyclo // static lookup table
var countryNames = map[string]string{
	"AF": "Afghanistan", "AL": "Albania", "DZ": "Algeria", "AR": "Argentina",
	"AT": "Austria", "AU": "Australia", "BD": "Bangladesh", "BE": "Belgium",
	"BR": "Brazil", "BG": "Bulgaria", "CA": "Canada", "CL": "Chile",
	"CN": "China", "CO": "Colombia", "HR": "Croatia", "CZ": "Czechia",
	"DK": "Denmark", "EG": "Egypt", "EE": "Estonia", "FI": "Finland",
	"FR": "France", "DE": "Germany", "GR": "Greece", "HK": "Hong Kong",
	"HU": "Hungary", "IN": "India", "ID": "Indonesia", "IE": "Ireland",
	"IL": "Israel", "IT": "Italy", "JP": "Japan", "KE": "Kenya",
	"KR": "South Korea", "LV": "Latvia", "LT": "Lithuania", "MY": "Malaysia",
	"MX": "Mexico", "NL": "Netherlands", "NZ": "New Zealand", "NG": "Nigeria",
	"NO": "Norway", "PK": "Pakistan", "PE": "Peru", "PH": "Philippines",
	"PL": "Poland", "PT": "Portugal", "RO": "Romania", "RU": "Russia",
	"SA": "Saudi Arabia", "SG": "Singapore", "SK": "Slovakia", "ZA": "South Africa",
	"ES": "Spain", "SE": "Sweden", "CH": "Switzerland", "TW": "Taiwan",
	"TH": "Thailand", "TR": "Türkiye", "UA": "Ukraine", "AE": "United Arab Emirates",
	"GB": "United Kingdom", "US": "United States", "VN": "Vietnam",
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
