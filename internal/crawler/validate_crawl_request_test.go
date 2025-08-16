package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCrawlRequest(t *testing.T) {
	tests := []struct {
		name           string
		ctx            context.Context
		targetURL      string
		expectError    bool
		expectResult   bool
		expectedScheme string
		expectedHost   string
	}{
		{
			name:           "valid_https_url",
			ctx:            context.Background(),
			targetURL:      "https://example.com",
			expectError:    false,
			expectResult:   false,
			expectedScheme: "https",
			expectedHost:   "example.com",
		},
		{
			name:           "valid_http_url",
			ctx:            context.Background(),
			targetURL:      "http://test.com",
			expectError:    false,
			expectResult:   false,
			expectedScheme: "http",
			expectedHost:   "test.com",
		},
		{
			name:           "valid_url_with_path",
			ctx:            context.Background(),
			targetURL:      "https://example.com/page?param=value",
			expectError:    false,
			expectResult:   false,
			expectedScheme: "https",
			expectedHost:   "example.com",
		},
		{
			name:         "invalid_url_format",
			ctx:          context.Background(),
			targetURL:    "not-a-url",
			expectError:  true,
			expectResult: true, // Should return error result
		},
		{
			name:         "url_missing_scheme",
			ctx:          context.Background(),
			targetURL:    "example.com",
			expectError:  true,
			expectResult: true, // Should return error result with invalid format
		},
		{
			name:         "url_missing_host",
			ctx:          context.Background(),
			targetURL:    "https://",
			expectError:  true,
			expectResult: true, // Should return error result
		},
		{
			name:         "empty_url",
			ctx:          context.Background(),
			targetURL:    "",
			expectError:  true,
			expectResult: true, // Should return error result
		},
		{
			name:         "malformed_url",
			ctx:          context.Background(),
			targetURL:    "ht!tp://bad-url",
			expectError:  true,
			expectResult: true, // Should return error result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, result, err := validateCrawlRequest(tt.ctx, tt.targetURL)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectResult {
					require.NotNil(t, result, "Should return error result")
					assert.Equal(t, tt.targetURL, result.URL)
					assert.NotEmpty(t, result.Error)
					assert.NotZero(t, result.Timestamp)
				}
				assert.Nil(t, parsedURL, "Should not return parsed URL on error")
			} else {
				assert.NoError(t, err)
				assert.Nil(t, result, "Should not return error result on success")
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

	parsedURL, result, err := validateCrawlRequest(cancelledCtx, "https://example.com")

	assert.Error(t, err)
	assert.Nil(t, parsedURL)
	assert.Nil(t, result)
	assert.Equal(t, context.Canceled, err)
}

func TestValidateCrawlRequestTimestampAccuracy(t *testing.T) {
	// Test that error results have accurate timestamps
	beforeTime := time.Now().Unix()
	
	_, result, err := validateCrawlRequest(context.Background(), "invalid-url")
	
	afterTime := time.Now().Unix()

	assert.Error(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Timestamp, beforeTime)
	assert.LessOrEqual(t, result.Timestamp, afterTime)
}

func TestValidateCrawlRequestURLEdgeCases(t *testing.T) {
	// Test edge cases for URL validation
	edgeCases := []struct {
		name      string
		url       string
		shouldErr bool
	}{
		{"url_with_port", "https://example.com:8080", false},
		{"url_with_auth", "https://user:pass@example.com", false},
		{"url_with_fragment", "https://example.com/page#section", false},
		{"url_with_query", "https://example.com/search?q=test&limit=10", false},
		{"ftp_scheme", "ftp://example.com", false}, // Valid URL format, just not HTTP
		{"javascript_url", "javascript:alert('xss')", true}, // Invalid - no host
		{"data_url", "data:text/plain;base64,SGVsbG8=", true}, // No host
		{"scheme_only", "https:", true},
		{"host_only", "//example.com", true}, // No scheme
		{"path_only", "/page", true}, // No scheme or host
		{"query_only", "?param=value", true}, // No scheme or host
		{"unicode_domain", "https://тест.com", false},
		{"punycode_domain", "https://xn--e1afmkfd.xn--p1ai", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, result, err := validateCrawlRequest(context.Background(), tc.url)

			if tc.shouldErr {
				assert.Error(t, err)
				assert.NotNil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, result)
			}
		})
	}
}