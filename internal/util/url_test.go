package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormaliseDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with_https",
			input:    "https://example.com",
			expected: "example.com",
		},
		{
			name:     "with_http",
			input:    "http://example.com",
			expected: "example.com",
		},
		{
			name:     "with_www",
			input:    "www.example.com",
			expected: "example.com",
		},
		{
			name:     "with_https_and_www",
			input:    "https://www.example.com",
			expected: "example.com",
		},
		{
			name:     "with_trailing_slash",
			input:    "example.com/",
			expected: "example.com",
		},
		{
			name:     "with_all_prefixes",
			input:    "https://www.example.com/",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			input:    "https://api.example.com",
			expected: "api.example.com",
		},
		{
			name:     "plain_domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "with_port",
			input:    "https://example.com:8080",
			expected: "example.com:8080",
		},
		{
			name:     "ip_address",
			input:    "http://192.168.1.1",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormaliseDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormaliseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain_domain",
			input:    "example.com",
			expected: "https://example.com",
		},
		{
			name:     "with_www",
			input:    "www.example.com",
			expected: "https://www.example.com",
		},
		{
			name:     "http_to_https",
			input:    "http://example.com",
			expected: "https://example.com",
		},
		{
			name:     "already_https",
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "with_path",
			input:    "example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "with_query",
			input:    "example.com/path?q=test",
			expected: "https://example.com/path?q=test",
		},
		{
			name:     "with_fragment",
			input:    "example.com#section",
			expected: "https://example.com#section",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace_only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "with_spaces",
			input:    "  example.com  ",
			expected: "https://example.com",
		},
		{
			name:     "with_port",
			input:    "example.com:8080",
			expected: "https://example.com:8080",
		},
		{
			name:     "subdomain",
			input:    "api.example.com",
			expected: "https://api.example.com",
		},
		{
			name:     "double_scheme_invalid",
			input:    "https://http://example.com",
			expected: "https://http://example.com", // Current behavior doesn't fix this
		},
		{
			name:     "ip_address",
			input:    "192.168.1.1",
			expected: "https://192.168.1.1",
		},
		{
			name:     "invalid_url",
			input:    "://invalid",
			expected: "https://://invalid", // Scheme gets added but remains invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormaliseURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPathFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full_url_with_path",
			input:    "https://example.com/path/to/page",
			expected: "/path/to/page",
		},
		{
			name:     "http_url_with_path",
			input:    "http://example.com/page",
			expected: "/page",
		},
		{
			name:     "url_with_www",
			input:    "https://www.example.com/test",
			expected: "/test",
		},
		{
			name:     "url_no_path",
			input:    "https://example.com",
			expected: "/",
		},
		{
			name:     "domain_only",
			input:    "example.com",
			expected: "/",
		},
		{
			name:     "path_with_query",
			input:    "https://example.com/search?q=test",
			expected: "/search?q=test",
		},
		{
			name:     "path_with_fragment",
			input:    "https://example.com/page#section",
			expected: "/page#section",
		},
		{
			name:     "subdomain_with_path",
			input:    "https://api.example.com/v1/users",
			expected: "/v1/users",
		},
		{
			name:     "just_path",
			input:    "/path/to/page",
			expected: "/path/to/page",
		},
		{
			name:     "root_path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "with_port",
			input:    "https://example.com:8080/api",
			expected: "/api",
		},
		{
			name:     "complex_path",
			input:    "https://example.com/path/to/page.html?param=value&other=123#anchor",
			expected: "/path/to/page.html?param=value&other=123#anchor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPathFromURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstructURL(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		path     string
		expected string
	}{
		{
			name:     "simple_domain_path",
			domain:   "example.com",
			path:     "/page",
			expected: "https://example.com/page",
		},
		{
			name:     "domain_with_https",
			domain:   "https://example.com",
			path:     "/test",
			expected: "https://example.com/test",
		},
		{
			name:     "domain_with_www",
			domain:   "www.example.com",
			path:     "/path",
			expected: "https://example.com/path",
		},
		{
			name:     "path_without_slash",
			domain:   "example.com",
			path:     "page",
			expected: "https://example.com/page",
		},
		{
			name:     "root_path",
			domain:   "example.com",
			path:     "/",
			expected: "https://example.com/",
		},
		{
			name:     "empty_path",
			domain:   "example.com",
			path:     "",
			expected: "https://example.com/",
		},
		{
			name:     "complex_path",
			domain:   "example.com",
			path:     "/path/to/page?q=test#section",
			expected: "https://example.com/path/to/page?q=test#section",
		},
		{
			name:     "subdomain",
			domain:   "api.example.com",
			path:     "/v1/users",
			expected: "https://api.example.com/v1/users",
		},
		{
			name:     "domain_with_trailing_slash",
			domain:   "example.com/",
			path:     "/page",
			expected: "https://example.com/page",
		},
		{
			name:     "domain_with_port",
			domain:   "example.com:8080",
			path:     "/api",
			expected: "https://example.com:8080/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstructURL(tt.domain, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkNormaliseDomain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NormaliseDomain("https://www.example.com/")
	}
}

func BenchmarkNormaliseURL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NormaliseURL("http://www.example.com/path?q=test")
	}
}

func BenchmarkExtractPathFromURL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ExtractPathFromURL("https://www.example.com/path/to/page?q=test#section")
	}
}

func BenchmarkConstructURL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ConstructURL("https://www.example.com", "/path/to/page")
	}
}