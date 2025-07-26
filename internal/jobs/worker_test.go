package jobs

import (
	"errors"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "network error",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "server error 500",
			err:      errors.New("internal server error"),
			expected: true,
		},
		{
			name:     "server error 503",
			err:      errors.New("service unavailable"),
			expected: true,
		},
		{
			name:     "403 forbidden - not retryable",
			err:      errors.New("403 forbidden"),
			expected: false,
		},
		{
			name:     "429 too many requests - not retryable",
			err:      errors.New("429 too many requests"),
			expected: false,
		},
		{
			name:     "rate limit error - not retryable",
			err:      errors.New("rate limit exceeded"),
			expected: false,
		},
		{
			name:     "random error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsBlockingError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "403 forbidden",
			err:      errors.New("403 forbidden"),
			expected: true,
		},
		{
			name:     "status code 403",
			err:      errors.New("non-success status code: 403"),
			expected: true,
		},
		{
			name:     "429 too many requests",
			err:      errors.New("429 too many requests"),
			expected: true,
		},
		{
			name:     "rate limit error",
			err:      errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "timeout error - not blocking",
			err:      errors.New("request timeout"),
			expected: false,
		},
		{
			name:     "500 error - not blocking",
			err:      errors.New("internal server error"),
			expected: false,
		},
		{
			name:     "network error - not blocking",
			err:      errors.New("connection refused"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBlockingError(tt.err)
			if result != tt.expected {
				t.Errorf("isBlockingError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}
