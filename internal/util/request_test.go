package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Forwarded-For single IP",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
			remoteAddr: "127.0.0.1:1234",
			expected:   "203.0.113.50",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50, 70.41.3.18, 150.172.238.178"},
			remoteAddr: "127.0.0.1:1234",
			expected:   "203.0.113.50",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "198.51.100.178"},
			remoteAddr: "127.0.0.1:1234",
			expected:   "198.51.100.178",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50", "X-Real-IP": "198.51.100.178"},
			remoteAddr: "127.0.0.1:1234",
			expected:   "203.0.113.50",
		},
		{
			name:       "falls back to RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:5678",
			expected:   "192.168.1.1",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, GetClientIP(r))
		})
	}
}

func TestParseBrowser(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected string
	}{
		{
			name:     "Chrome on macOS",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			expected: "Chrome",
		},
		{
			name:     "Firefox on Windows",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			expected: "Firefox",
		},
		{
			name:     "Safari on macOS",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
			expected: "Safari",
		},
		{
			name:     "Edge on Windows",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			expected: "Edge",
		},
		{
			name:     "Opera",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
			expected: "Opera",
		},
		{
			name:     "empty",
			ua:       "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseBrowser(tt.ua))
		})
	}
}

func TestParseOS(t *testing.T) {
	tests := []struct {
		name     string
		ua       string
		expected string
	}{
		{
			name:     "macOS",
			ua:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			expected: "macOS",
		},
		{
			name:     "Windows",
			ua:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			expected: "Windows",
		},
		{
			name:     "Linux",
			ua:       "Mozilla/5.0 (X11; Linux x86_64)",
			expected: "Linux",
		},
		{
			name:     "iOS iPhone",
			ua:       "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X)",
			expected: "iOS",
		},
		{
			name:     "iOS iPad",
			ua:       "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X)",
			expected: "iOS",
		},
		{
			name:     "Android",
			ua:       "Mozilla/5.0 (Linux; Android 14; Pixel 8)",
			expected: "Android",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseOS(tt.ua))
		})
	}
}

func TestExtractRequestMeta(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/organisations/invites", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	r.Header.Set("X-Forwarded-For", "203.0.113.50")

	meta := ExtractRequestMeta(r)

	assert.Equal(t, "203.0.113.50", meta.IP)
	assert.Equal(t, "Chrome", meta.Browser)
	assert.Equal(t, "macOS", meta.OS)
	assert.Equal(t, "Chrome on macOS", meta.Device)
	assert.NotEmpty(t, meta.FormattedTimestamp())
}
