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

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid domains
		{name: "simple_domain", input: "example.com", wantError: false},
		{name: "subdomain", input: "www.example.com", wantError: false},
		{name: "deep_subdomain", input: "api.v1.example.com", wantError: false},
		{name: "with_https", input: "https://example.com", wantError: false},
		{name: "with_http", input: "http://example.com", wantError: false},
		{name: "co_uk_tld", input: "example.co.uk", wantError: false},
		{name: "hyphen_in_domain", input: "my-site.example.com", wantError: false},
		{name: "numbers_in_domain", input: "site123.example.com", wantError: false},

		// Invalid domains - using publicsuffix error messages
		{name: "no_tld", input: "asdfasdf", wantError: true, errorMsg: "invalid domain"},
		{name: "empty", input: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "just_tld", input: ".com", wantError: true, errorMsg: "empty label"},
		{name: "double_dot", input: "example..com", wantError: true, errorMsg: "empty label"},
		{name: "localhost", input: "localhost", wantError: true, errorMsg: "not allowed"},
		{name: "localhost_with_subdomain", input: "api.localhost", wantError: true, errorMsg: "not allowed"},
		{name: "internal", input: "internal", wantError: true, errorMsg: "not allowed"},
		{name: "just_suffix", input: "com", wantError: true, errorMsg: "invalid domain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.input)
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
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

func TestIsSignificantRedirect(t *testing.T) {
	tests := []struct {
		name        string
		originalURL string
		redirectURL string
		expected    bool
	}{
		// Not significant - same domain/path variations
		{
			name:        "empty_redirect",
			originalURL: "https://example.com/page",
			redirectURL: "",
			expected:    false,
		},
		{
			name:        "http_to_https_same_path",
			originalURL: "http://example.com/page",
			redirectURL: "https://example.com/page",
			expected:    false,
		},
		{
			name:        "www_to_non_www",
			originalURL: "https://www.example.com/page",
			redirectURL: "https://example.com/page",
			expected:    false,
		},
		{
			name:        "non_www_to_www",
			originalURL: "https://example.com/page",
			redirectURL: "https://www.example.com/page",
			expected:    false,
		},
		{
			name:        "trailing_slash_added",
			originalURL: "https://example.com/page",
			redirectURL: "https://example.com/page/",
			expected:    false,
		},
		{
			name:        "trailing_slash_removed",
			originalURL: "https://example.com/page/",
			redirectURL: "https://example.com/page",
			expected:    false,
		},
		{
			name:        "root_path_variations",
			originalURL: "https://example.com",
			redirectURL: "https://example.com/",
			expected:    false,
		},
		{
			name:        "case_insensitive_domain",
			originalURL: "https://EXAMPLE.COM/page",
			redirectURL: "https://example.com/page",
			expected:    false,
		},
		// Significant - different domain
		{
			name:        "different_domain",
			originalURL: "https://example.com/page",
			redirectURL: "https://other.com/page",
			expected:    true,
		},
		{
			name:        "subdomain_change",
			originalURL: "https://example.com/page",
			redirectURL: "https://blog.example.com/page",
			expected:    true,
		},
		{
			name:        "external_redirect",
			originalURL: "https://example.com/page",
			redirectURL: "https://google.com/",
			expected:    true,
		},
		// Significant - different path
		{
			name:        "different_path",
			originalURL: "https://example.com/old-page",
			redirectURL: "https://example.com/new-page",
			expected:    true,
		},
		{
			name:        "path_added",
			originalURL: "https://example.com/",
			redirectURL: "https://example.com/home",
			expected:    true,
		},
		{
			name:        "deeper_path",
			originalURL: "https://example.com/blog",
			redirectURL: "https://example.com/blog/post",
			expected:    true,
		},
		// Not significant - default port handling
		{
			name:        "https_default_port_to_no_port",
			originalURL: "https://example.com:443/page",
			redirectURL: "https://example.com/page",
			expected:    false,
		},
		{
			name:        "http_default_port_to_no_port",
			originalURL: "http://example.com:80/page",
			redirectURL: "http://example.com/page",
			expected:    false,
		},
		// Significant - non-default port
		{
			name:        "different_non_default_port",
			originalURL: "https://example.com:8080/page",
			redirectURL: "https://example.com/page",
			expected:    true,
		},
		// Not significant - query parameter changes (ignored)
		{
			name:        "query_parameter_change",
			originalURL: "https://example.com/page?a=1",
			redirectURL: "https://example.com/page?b=2",
			expected:    false,
		},
		// Not significant - fragment changes (ignored)
		{
			name:        "fragment_change",
			originalURL: "https://example.com/page#section1",
			redirectURL: "https://example.com/page#section2",
			expected:    false,
		},
		// Significant - malformed URL handling
		{
			name:        "malformed_original_url",
			originalURL: "://invalid",
			redirectURL: "https://example.com/page",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSignificantRedirect(tt.originalURL, tt.redirectURL)
			assert.Equal(t, tt.expected, result, "originalURL: %s, redirectURL: %s", tt.originalURL, tt.redirectURL)
		})
	}
}
