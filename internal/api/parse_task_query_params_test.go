package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTaskQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected TaskQueryParams
	}{
		{
			name: "default_values",
			url:  "/v1/jobs/123/tasks",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "custom_limit_and_offset",
			url:  "/v1/jobs/123/tasks?limit=100&offset=20",
			expected: TaskQueryParams{
				Limit:   100,
				Offset:  20,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "with_status_filter",
			url:  "/v1/jobs/123/tasks?status=completed",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "completed",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "sort_by_path_ascending",
			url:  "/v1/jobs/123/tasks?sort=path",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "p.path ASC",
			},
		},
		{
			name: "sort_by_status_descending",
			url:  "/v1/jobs/123/tasks?sort=-status",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.status DESC",
			},
		},
		{
			name: "sort_by_response_time",
			url:  "/v1/jobs/123/tasks?sort=response_time",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.response_time ASC NULLS LAST",
			},
		},
		{
			name: "sort_by_cache_status",
			url:  "/v1/jobs/123/tasks?sort=-cache_status",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.cache_status DESC NULLS LAST",
			},
		},
		{
			name: "sort_by_second_response_time",
			url:  "/v1/jobs/123/tasks?sort=second_response_time",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.second_response_time ASC NULLS LAST",
			},
		},
		{
			name: "sort_by_status_code",
			url:  "/v1/jobs/123/tasks?sort=-status_code",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.status_code DESC NULLS LAST",
			},
		},
		{
			name: "sort_by_created_at_explicit",
			url:  "/v1/jobs/123/tasks?sort=created_at",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at ASC",
			},
		},
		{
			name: "invalid_sort_column_fallback",
			url:  "/v1/jobs/123/tasks?sort=invalid_column",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "limit_at_max_boundary",
			url:  "/v1/jobs/123/tasks?limit=200",
			expected: TaskQueryParams{
				Limit:   200,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "limit_over_max_clamped",
			url:  "/v1/jobs/123/tasks?limit=500",
			expected: TaskQueryParams{
				Limit:   50, // Should clamp to default since 500 > 200
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "zero_limit_ignored",
			url:  "/v1/jobs/123/tasks?limit=0",
			expected: TaskQueryParams{
				Limit:   50, // Should use default for zero limit
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "negative_limit_ignored",
			url:  "/v1/jobs/123/tasks?limit=-10",
			expected: TaskQueryParams{
				Limit:   50, // Should use default for negative limit
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "negative_offset_ignored",
			url:  "/v1/jobs/123/tasks?offset=-10",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0, // Should ignore negative offset
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "invalid_limit_ignored",
			url:  "/v1/jobs/123/tasks?limit=abc",
			expected: TaskQueryParams{
				Limit:   50, // Should use default for invalid limit
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "invalid_offset_ignored",
			url:  "/v1/jobs/123/tasks?offset=xyz",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0, // Should use default for invalid offset
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "all_params_combined",
			url:  "/v1/jobs/123/tasks?limit=25&offset=10&status=pending&sort=-created_at",
			expected: TaskQueryParams{
				Limit:   25,
				Offset:  10,
				Status:  "pending",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "url_encoded_status",
			url:  "/v1/jobs/123/tasks?status=in%20progress",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "in progress",
				OrderBy: "t.created_at DESC",
			},
		},
		{
			name: "empty_sort_parameter",
			url:  "/v1/jobs/123/tasks?sort=",
			expected: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			result := parseTaskQueryParams(req)
			
			assert.Equal(t, tt.expected.Limit, result.Limit, "Limit mismatch")
			assert.Equal(t, tt.expected.Offset, result.Offset, "Offset mismatch")
			assert.Equal(t, tt.expected.Status, result.Status, "Status mismatch")
			assert.Equal(t, tt.expected.OrderBy, result.OrderBy, "OrderBy mismatch")
		})
	}
}

func TestParseTaskQueryParamsEdgeCases(t *testing.T) {
	t.Run("multiple_sort_params_uses_first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?sort=path&sort=status", nil)
		result := parseTaskQueryParams(req)
		assert.Equal(t, "p.path ASC", result.OrderBy)
	})

	t.Run("multiple_limit_params_uses_first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?limit=10&limit=20", nil)
		result := parseTaskQueryParams(req)
		assert.Equal(t, 10, result.Limit)
	})

	t.Run("case_sensitive_sort_columns", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?sort=PATH", nil)
		result := parseTaskQueryParams(req)
		// Should fallback to default since PATH != path
		assert.Equal(t, "t.created_at DESC", result.OrderBy)
	})
}