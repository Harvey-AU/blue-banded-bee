package crawler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCrawlRequest(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		targetURL      string
		expectError    bool
		expectedScheme string
		expectedHost   string
	}{
		{
			name:           "valid_https_url",
			ctx:            context.Background(),
			targetURL:      "https://example.com",
			expectError:    false,
			expectedScheme: "https",
			expectedHost:   "example.com",
		},
		{
			name:           "valid_http_url",
			ctx:            context.Background(),
			targetURL:      "http://test.com",
			expectError:    false,
			expectedScheme: "http",
			expectedHost:   "test.com",
		},
		{
			name:           "valid_url_with_path",
			ctx:            context.Background(),
			targetURL:      "https://example.com/page?param=value",
			expectError:    false,
			expectedScheme: "https",
			expectedHost:   "example.com",
		},
		{
			name:        "invalid_url_format",
			ctx:         context.Background(),
			targetURL:   "not-a-url",
			expectError: true,
		},
		{
			name:        "url_missing_scheme",
			ctx:         context.Background(),
			targetURL:   "example.com",
			expectError: true,
		},
		{
			name:        "url_missing_host",
			ctx:         context.Background(),
			targetURL:   "https://",
			expectError: true,
		},
		{
			name:        "empty_url",
			ctx:         context.Background(),
			targetURL:   "",
			expectError: true,
		},
		{
			name:        "malformed_url",
			ctx:         context.Background(),
			targetURL:   "ht!tp://bad-url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip SSRF check for these URL format tests (DNS may not resolve)
			parsedURL, err := validateCrawlRequest(tt.ctx, tt.targetURL, true)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, parsedURL, "Should not return parsed URL on error")
			} else {
				assert.NoError(t, err)
				require.NotNil(t, parsedURL, "Should return parsed URL on success")
				assert.Equal(t, tt.expectedScheme, parsedURL.Scheme)
				assert.Equal(t, tt.expectedHost, parsedURL.Host)
			}
		})
	}
}

func TestValidateCrawlRequestContextCancellation(t *testing.T) {
	// Test context cancellation handling
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	parsedURL, err := validateCrawlRequest(cancelledCtx, "https://example.com", true)

	assert.Error(t, err)
	assert.Nil(t, parsedURL)
	assert.Equal(t, context.Canceled, err)
}

func TestValidateCrawlRequestErrorMessages(t *testing.T) {
	// Test that error messages are helpful and descriptive
	tests := []struct {
		name          string
		url           string
		errorContains string
	}{
		{
			name:          "invalid_format_error",
			url:           "not-a-url",
			errorContains: "invalid URL format",
		},
		{
			name:          "missing_scheme_error",
			url:           "example.com",
			errorContains: "invalid URL format",
		},
		{
			name:          "missing_host_error",
			url:           "https://",
			errorContains: "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateCrawlRequest(context.Background(), tt.url, true)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestValidateCrawlRequestURLEdgeCases(t *testing.T) {
	// Test edge cases for URL validation (skip SSRF for format-only tests)
	edgeCases := []struct {
		name      string
		url       string
		shouldErr bool
	}{
		{"url_with_port", "https://example.com:8080", false},
		{"url_with_auth", "https://user:pass@example.com", false},
		{"url_with_fragment", "https://example.com/page#section", false},
		{"url_with_query", "https://example.com/search?q=test&limit=10", false},
		{"ftp_scheme", "ftp://example.com", false},            // Valid URL format, just not HTTP
		{"javascript_url", "javascript:alert('xss')", true},   // Invalid - no host
		{"data_url", "data:text/plain;base64,SGVsbG8=", true}, // No host
		{"scheme_only", "https:", true},
		{"host_only", "//example.com", true}, // No scheme
		{"path_only", "/page", true},         // No scheme or host
		{"query_only", "?param=value", true}, // No scheme or host
		{"unicode_domain", "https://тест.com", false},
		{"punycode_domain", "https://xn--e1afmkfd.xn--p1ai", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedURL, err := validateCrawlRequest(context.Background(), tc.url, true)

			if tc.shouldErr {
				assert.Error(t, err)
				assert.Nil(t, parsedURL)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parsedURL)
			}
		})
	}
}
