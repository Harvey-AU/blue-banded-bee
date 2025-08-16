package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTaskQuery(t *testing.T) {
	tests := []struct {
		name            string
		jobID           string
		params          TaskQueryParams
		expectedArgsLen int
		expectStatus    bool
	}{
		{
			name:  "basic_query_no_filters",
			jobID: "job-123",
			params: TaskQueryParams{
				Limit:   50,
				Offset:  0,
				Status:  "",
				OrderBy: "t.created_at DESC",
			},
			expectedArgsLen: 3, // jobID, limit, offset
			expectStatus:    false,
		},
		{
			name:  "query_with_status_filter",
			jobID: "job-456",
			params: TaskQueryParams{
				Limit:   25,
				Offset:  10,
				Status:  "completed",
				OrderBy: "t.status ASC",
			},
			expectedArgsLen: 4, // jobID, status, limit, offset
			expectStatus:    true,
		},
		{
			name:  "query_with_custom_ordering",
			jobID: "job-789",
			params: TaskQueryParams{
				Limit:   100,
				Offset:  50,
				Status:  "",
				OrderBy: "p.path ASC",
			},
			expectedArgsLen: 3, // jobID, limit, offset
			expectStatus:    false,
		},
		{
			name:  "query_with_response_time_ordering",
			jobID: "job-abc",
			params: TaskQueryParams{
				Limit:   75,
				Offset:  25,
				Status:  "pending",
				OrderBy: "t.response_time DESC NULLS LAST",
			},
			expectedArgsLen: 4, // jobID, status, limit, offset
			expectStatus:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTaskQuery(tt.jobID, tt.params)

			// Verify structure
			assert.NotEmpty(t, result.SelectQuery, "SelectQuery should not be empty")
			assert.NotEmpty(t, result.CountQuery, "CountQuery should not be empty")
			assert.NotEmpty(t, result.Args, "Args should not be empty")

			// Verify argument count
			assert.Len(t, result.Args, tt.expectedArgsLen, "Args length mismatch")

			// Verify first argument is always jobID
			assert.Equal(t, tt.jobID, result.Args[0], "First argument should be jobID")

			// Verify status filter handling
			if tt.expectStatus {
				assert.Contains(t, result.SelectQuery, "AND t.status = $2", "SelectQuery should contain status filter")
				assert.Contains(t, result.CountQuery, "AND t.status = $2", "CountQuery should contain status filter")
				assert.Equal(t, tt.params.Status, result.Args[1], "Second argument should be status")
				assert.Equal(t, tt.params.Limit, result.Args[2], "Third argument should be limit")
				assert.Equal(t, tt.params.Offset, result.Args[3], "Fourth argument should be offset")
			} else {
				assert.NotContains(t, result.SelectQuery, "AND t.status", "SelectQuery should not contain status filter")
				assert.NotContains(t, result.CountQuery, "AND t.status", "CountQuery should not contain status filter")
				assert.Equal(t, tt.params.Limit, result.Args[1], "Second argument should be limit")
				assert.Equal(t, tt.params.Offset, result.Args[2], "Third argument should be offset")
			}

			// Verify ordering is included
			assert.Contains(t, result.SelectQuery, "ORDER BY "+tt.params.OrderBy, "SelectQuery should contain ordering")

			// Verify LIMIT and OFFSET are included
			assert.Contains(t, result.SelectQuery, "LIMIT", "SelectQuery should contain LIMIT")
			assert.Contains(t, result.SelectQuery, "OFFSET", "SelectQuery should contain OFFSET")

			// Verify basic structure of queries
			assert.Contains(t, result.SelectQuery, "SELECT t.id, t.job_id", "SelectQuery should select task fields")
			assert.Contains(t, result.SelectQuery, "FROM tasks t", "SelectQuery should use tasks table")
			assert.Contains(t, result.SelectQuery, "JOIN pages p", "SelectQuery should join pages")
			assert.Contains(t, result.SelectQuery, "WHERE t.job_id = $1", "SelectQuery should filter by job_id")

			assert.Contains(t, result.CountQuery, "SELECT COUNT(*)", "CountQuery should count rows")
			assert.Contains(t, result.CountQuery, "FROM tasks t", "CountQuery should use tasks table")
			assert.Contains(t, result.CountQuery, "WHERE t.job_id = $1", "CountQuery should filter by job_id")
		})
	}
}

func TestBuildTaskQueryParameterPlaceholders(t *testing.T) {
	t.Run("no_status_filter_placeholders", func(t *testing.T) {
		params := TaskQueryParams{
			Limit:   50,
			Offset:  10,
			Status:  "",
			OrderBy: "t.created_at DESC",
		}
		
		result := buildTaskQuery("job-123", params)
		
		// Should have $1 (jobID), $2 (limit), $3 (offset)
		assert.Contains(t, result.SelectQuery, "$1", "Should contain $1 placeholder")
		assert.Contains(t, result.SelectQuery, "$2", "Should contain $2 placeholder")
		assert.Contains(t, result.SelectQuery, "$3", "Should contain $3 placeholder")
		assert.NotContains(t, result.SelectQuery, "$4", "Should not contain $4 placeholder")
		
		// Verify args match placeholders
		require.Len(t, result.Args, 3)
		assert.Equal(t, "job-123", result.Args[0])
		assert.Equal(t, 50, result.Args[1])
		assert.Equal(t, 10, result.Args[2])
	})

	t.Run("with_status_filter_placeholders", func(t *testing.T) {
		params := TaskQueryParams{
			Limit:   25,
			Offset:  5,
			Status:  "completed",
			OrderBy: "t.status ASC",
		}
		
		result := buildTaskQuery("job-456", params)
		
		// Should have $1 (jobID), $2 (status), $3 (limit), $4 (offset)
		assert.Contains(t, result.SelectQuery, "$1", "Should contain $1 placeholder")
		assert.Contains(t, result.SelectQuery, "$2", "Should contain $2 placeholder")
		assert.Contains(t, result.SelectQuery, "$3", "Should contain $3 placeholder")
		assert.Contains(t, result.SelectQuery, "$4", "Should contain $4 placeholder")
		assert.NotContains(t, result.SelectQuery, "$5", "Should not contain $5 placeholder")
		
		// Verify args match placeholders
		require.Len(t, result.Args, 4)
		assert.Equal(t, "job-456", result.Args[0])
		assert.Equal(t, "completed", result.Args[1])
		assert.Equal(t, 25, result.Args[2])
		assert.Equal(t, 5, result.Args[3])
	})
}

func TestBuildTaskQuerySQLInjectionSafety(t *testing.T) {
	// Test that the function properly handles potentially dangerous inputs
	// via parameterized queries
	
	t.Run("job_id_with_sql_injection_attempt", func(t *testing.T) {
		maliciousJobID := "'; DROP TABLE tasks; --"
		params := TaskQueryParams{
			Limit:   50,
			Offset:  0,
			Status:  "",
			OrderBy: "t.created_at DESC",
		}
		
		result := buildTaskQuery(maliciousJobID, params)
		
		// The jobID should be safely parameterized as $1
		assert.Contains(t, result.SelectQuery, "WHERE t.job_id = $1")
		assert.Equal(t, maliciousJobID, result.Args[0])
		
		// Query structure should remain intact
		assert.Contains(t, result.SelectQuery, "SELECT t.id")
		assert.Contains(t, result.SelectQuery, "FROM tasks t")
	})
	
	t.Run("status_with_sql_injection_attempt", func(t *testing.T) {
		maliciousStatus := "'; DELETE FROM tasks WHERE 1=1; --"
		params := TaskQueryParams{
			Limit:   50,
			Offset:  0,
			Status:  maliciousStatus,
			OrderBy: "t.created_at DESC",
		}
		
		result := buildTaskQuery("job-123", params)
		
		// The status should be safely parameterized as $2
		assert.Contains(t, result.SelectQuery, "AND t.status = $2")
		assert.Equal(t, maliciousStatus, result.Args[1])
		
		// Query structure should remain intact
		assert.Contains(t, result.SelectQuery, "SELECT t.id")
		assert.NotContains(t, result.SelectQuery, "DELETE")
	})
	
	t.Run("orderby_is_not_parameterized_but_controlled", func(t *testing.T) {
		// OrderBy is built from controlled input via parseTaskQueryParams
		// so it should be safe, but let's verify the function handles it properly
		params := TaskQueryParams{
			Limit:   50,
			Offset:  0,
			Status:  "",
			OrderBy: "t.created_at DESC",
		}
		
		result := buildTaskQuery("job-123", params)
		
		// OrderBy should be directly inserted (not parameterized) because
		// it comes from parseTaskQueryParams which validates the values
		assert.Contains(t, result.SelectQuery, "ORDER BY t.created_at DESC")
	})
}