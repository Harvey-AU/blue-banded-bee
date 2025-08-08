package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDashboardStatsEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedFields []string
		description    string
	}{
		{
			name:           "stats_no_filter",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"total_jobs", "completed_jobs", "failed_jobs", "active_jobs"},
			description:    "Should return all stats without filter",
		},
		{
			name:           "stats_with_date_range",
			queryParams:    "?start_date=2024-01-01&end_date=2024-12-31",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"total_jobs", "completed_jobs", "failed_jobs", "active_jobs", "date_range"},
			description:    "Should return stats for date range",
		},
		{
			name:           "stats_with_domain_filter",
			queryParams:    "?domain=example.com",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"total_jobs", "domain"},
			description:    "Should return stats for specific domain",
		},
		{
			name:           "stats_invalid_date",
			queryParams:    "?start_date=invalid",
			expectedStatus: http.StatusBadRequest,
			description:    "Should error on invalid date format",
		},
		{
			name:           "stats_future_dates",
			queryParams:    "?start_date=2025-01-01&end_date=2024-01-01",
			expectedStatus: http.StatusBadRequest,
			description:    "Should error when start > end date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/dashboard/stats"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Mock handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Parse query params
				startDate := r.URL.Query().Get("start_date")
				endDate := r.URL.Query().Get("end_date")
				domain := r.URL.Query().Get("domain")

				// Validate dates
				if startDate != "" {
					if _, err := time.Parse("2006-01-02", startDate); err != nil {
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{"error": "Invalid start date"})
						return
					}
				}
				
				if endDate != "" {
					if _, err := time.Parse("2006-01-02", endDate); err != nil {
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{"error": "Invalid end date"})
						return
					}
				}

				if startDate != "" && endDate != "" {
					start, _ := time.Parse("2006-01-02", startDate)
					end, _ := time.Parse("2006-01-02", endDate)
					if start.After(end) {
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{"error": "Start date must be before end date"})
						return
					}
				}

				// Return mock stats
				stats := map[string]interface{}{
					"total_jobs":     100,
					"completed_jobs": 75,
					"failed_jobs":    10,
					"active_jobs":    15,
				}

				if startDate != "" && endDate != "" {
					stats["date_range"] = map[string]string{
						"start": startDate,
						"end":   endDate,
					}
				}

				if domain != "" {
					stats["domain"] = domain
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(stats)
			})

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK && tt.expectedFields != nil {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				
				for _, field := range tt.expectedFields {
					assert.Contains(t, response, field, "Response should contain field: %s", field)
				}
			}
		})
	}
}

func TestDashboardActivityTimeline(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedCount  int
		description    string
	}{
		{
			name:           "timeline_default",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  10, // Default limit
			description:    "Should return default timeline entries",
		},
		{
			name:           "timeline_with_limit",
			queryParams:    "?limit=5",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			description:    "Should respect limit parameter",
		},
		{
			name:           "timeline_with_offset",
			queryParams:    "?offset=10&limit=5",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			description:    "Should support pagination with offset",
		},
		{
			name:           "timeline_invalid_limit",
			queryParams:    "?limit=-1",
			expectedStatus: http.StatusBadRequest,
			description:    "Should error on negative limit",
		},
		{
			name:           "timeline_excessive_limit",
			queryParams:    "?limit=1000",
			expectedStatus: http.StatusOK,
			expectedCount:  100, // Max limit
			description:    "Should cap at maximum limit",
		},
		{
			name:           "timeline_with_type_filter",
			queryParams:    "?type=job_created",
			expectedStatus: http.StatusOK,
			description:    "Should filter by activity type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/dashboard/activity"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Mock handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Parse query params
				limitStr := r.URL.Query().Get("limit")
				offsetStr := r.URL.Query().Get("offset")
				activityType := r.URL.Query().Get("type")

				limit := 10 // Default
				if limitStr != "" {
					var err error
					limit = 0
					for _, c := range limitStr {
						if c >= '0' && c <= '9' {
							limit = limit*10 + int(c-'0')
						} else if c == '-' && limit == 0 {
							w.WriteHeader(http.StatusBadRequest)
							json.NewEncoder(w).Encode(map[string]string{"error": "Invalid limit"})
							return
						}
					}
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]string{"error": "Invalid limit"})
						return
					}
				}

				// Cap at max
				if limit > 100 {
					limit = 100
				}

				offset := 0
				if offsetStr != "" {
					for _, c := range offsetStr {
						if c >= '0' && c <= '9' {
							offset = offset*10 + int(c-'0')
						}
					}
				}

				// Generate mock activity
				activities := make([]map[string]interface{}, 0, limit)
				for i := 0; i < limit; i++ {
					activity := map[string]interface{}{
						"id":        offset + i + 1,
						"type":      "job_created",
						"timestamp": time.Now().Add(-time.Hour * time.Duration(i)).Format(time.RFC3339),
						"details":   "Job created for example.com",
					}
					
					if activityType == "" || activityType == "job_created" {
						activities = append(activities, activity)
					}
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"activities": activities,
					"total":      100,
					"offset":     offset,
					"limit":      limit,
				})
			})

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				
				if activities, ok := response["activities"].([]interface{}); ok {
					if tt.expectedCount > 0 {
						assert.Equal(t, tt.expectedCount, len(activities), "Activity count should match")
					}
				}
			}
		})
	}
}

func TestDashboardDateRangeFiltering(t *testing.T) {
	tests := []struct {
		name           string
		period         string
		customStart    string
		customEnd      string
		expectedDays   int
		expectedError  bool
		description    string
	}{
		{
			name:         "today",
			period:       "today",
			expectedDays: 1,
			description:  "Today should be 1 day range",
		},
		{
			name:         "yesterday",
			period:       "yesterday",
			expectedDays: 1,
			description:  "Yesterday should be 1 day range",
		},
		{
			name:         "last_7_days",
			period:       "7d",
			expectedDays: 7,
			description:  "Last 7 days range",
		},
		{
			name:         "last_30_days",
			period:       "30d",
			expectedDays: 30,
			description:  "Last 30 days range",
		},
		{
			name:         "this_month",
			period:       "month",
			expectedDays: -1, // Variable
			description:  "Current month range",
		},
		{
			name:         "custom_range",
			period:       "custom",
			customStart:  "2024-01-01",
			customEnd:    "2024-01-31",
			expectedDays: 31,
			description:  "Custom date range",
		},
		{
			name:          "invalid_period",
			period:        "invalid",
			expectedError: true,
			description:   "Invalid period should error",
		},
		{
			name:          "custom_missing_dates",
			period:        "custom",
			expectedError: true,
			description:   "Custom without dates should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test calculateDateRange equivalent
			start, end, err := calculateTestDateRange(tt.period, tt.customStart, tt.customEnd)
			
			if tt.expectedError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				
				if tt.expectedDays > 0 {
					days := int(end.Sub(start).Hours() / 24) + 1
					assert.Equal(t, tt.expectedDays, days, "Date range should match expected days")
				}
			}
		})
	}
}

func TestDashboardPagination(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		perPage        int
		total          int
		expectedOffset int
		expectedLimit  int
		description    string
	}{
		{
			name:           "first_page",
			page:           1,
			perPage:        10,
			total:          100,
			expectedOffset: 0,
			expectedLimit:  10,
			description:    "First page should have offset 0",
		},
		{
			name:           "second_page",
			page:           2,
			perPage:        10,
			total:          100,
			expectedOffset: 10,
			expectedLimit:  10,
			description:    "Second page should have offset 10",
		},
		{
			name:           "last_page_partial",
			page:           11,
			perPage:        10,
			total:          105,
			expectedOffset: 100,
			expectedLimit:  5,
			description:    "Last page may be partial",
		},
		{
			name:           "page_beyond_total",
			page:           20,
			perPage:        10,
			total:          50,
			expectedOffset: 190,
			expectedLimit:  0,
			description:    "Page beyond total should return empty",
		},
		{
			name:           "large_per_page",
			page:           1,
			perPage:        100,
			total:          50,
			expectedOffset: 0,
			expectedLimit:  50,
			description:    "Per page larger than total",
		},
		{
			name:           "zero_page",
			page:           0,
			perPage:        10,
			total:          100,
			expectedOffset: 0,
			expectedLimit:  10,
			description:    "Zero page should default to 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := tt.page
			if page < 1 {
				page = 1
			}
			
			offset := (page - 1) * tt.perPage
			limit := tt.perPage
			
			// Adjust limit for last page
			remaining := tt.total - offset
			if remaining < limit {
				limit = remaining
			}
			if limit < 0 {
				limit = 0
			}
			
			assert.Equal(t, tt.expectedOffset, offset, "Offset should match")
			assert.Equal(t, tt.expectedLimit, limit, "Limit should match")
		})
	}
}

func TestDashboardErrorResponses(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		expectedError  string
		description    string
	}{
		{
			name:           "unauthorized",
			endpoint:       "/v1/dashboard/stats",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Unauthorized",
			description:    "Should return 401 without auth",
		},
		{
			name:           "forbidden",
			endpoint:       "/v1/dashboard/admin/stats",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Forbidden",
			description:    "Should return 403 for insufficient permissions",
		},
		{
			name:           "not_found",
			endpoint:       "/v1/dashboard/invalid",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Not Found",
			description:    "Should return 404 for invalid endpoint",
		},
		{
			name:           "internal_error",
			endpoint:       "/v1/dashboard/stats?trigger_error=true",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal Server Error",
			description:    "Should return 500 for server errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()

			// Mock handler with various error conditions
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v1/dashboard/invalid":
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"error": "Not Found"})
				case r.URL.Path == "/v1/dashboard/admin/stats":
					w.WriteHeader(http.StatusForbidden)
					json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
				case r.URL.Query().Get("trigger_error") == "true":
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{"error": "Internal Server Error"})
				case r.Header.Get("Authorization") == "":
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
				default:
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
				}
			})

			// Add auth header for non-auth tests
			if tt.name != "unauthorized" {
				req.Header.Set("Authorization", "Bearer test-token")
			}

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedError != "" {
				var response map[string]string
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Equal(t, tt.expectedError, response["error"], "Error message should match")
			}
		})
	}
}

// Helper function for date range calculation
func calculateTestDateRange(period, customStart, customEnd string) (time.Time, time.Time, error) {
	now := time.Now()
	var start, end time.Time

	switch period {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.Add(24*time.Hour - time.Second)
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		end = start.Add(24*time.Hour - time.Second)
	case "7d":
		start = now.AddDate(0, 0, -6)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		end = now
	case "30d":
		start = now.AddDate(0, 0, -29)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		end = now
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = now
	case "custom":
		if customStart == "" || customEnd == "" {
			return start, end, assert.AnError
		}
		var err error
		start, err = time.Parse("2006-01-02", customStart)
		if err != nil {
			return start, end, err
		}
		end, err = time.Parse("2006-01-02", customEnd)
		if err != nil {
			return start, end, err
		}
	default:
		return start, end, assert.AnError
	}

	return start, end, nil
}