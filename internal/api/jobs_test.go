package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/auth"
	"github.com/Harvey-AU/blue-banded-bee/internal/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		hasAuth        bool
	}{
		{
			name:           "GET_without_auth",
			method:         http.MethodGet,
			expectedStatus: http.StatusUnauthorized,
			hasAuth:        false,
		},
		{
			name:           "POST_without_auth",
			method:         http.MethodPost,
			expectedStatus: http.StatusUnauthorized,
			hasAuth:        false,
		},
		{
			name:           "PATCH_not_allowed",
			method:         http.MethodPatch,
			expectedStatus: http.StatusMethodNotAllowed,
			hasAuth:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{}
			req := httptest.NewRequest(tt.method, "/v1/jobs", nil)
			
			if tt.hasAuth {
				ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
					UserID: "test-user",
					Email:  "test@example.com",
				})
				req = req.WithContext(ctx)
			}
			
			rec := httptest.NewRecorder()
			h.JobsHandler(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestJobHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		hasAuth        bool
	}{
		{
			name:           "empty_job_id",
			method:         http.MethodGet,
			path:           "/v1/jobs/",
			expectedStatus: http.StatusBadRequest,
			hasAuth:        true,
		},
		{
			name:           "tasks_subresource",
			method:         http.MethodGet,
			path:           "/v1/jobs/job-123/tasks",
			expectedStatus: http.StatusUnauthorized, // Will fail auth check in sub-handler
			hasAuth:        false,
		},
		{
			name:           "unknown_subresource",
			method:         http.MethodGet,
			path:           "/v1/jobs/job-123/unknown",
			expectedStatus: http.StatusNotFound,
			hasAuth:        true,
		},
		{
			name:           "GET_job",
			method:         http.MethodGet,
			path:           "/v1/jobs/job-123",
			expectedStatus: http.StatusUnauthorized,
			hasAuth:        false,
		},
		{
			name:           "PUT_job",
			method:         http.MethodPut,
			path:           "/v1/jobs/job-123",
			expectedStatus: http.StatusUnauthorized,
			hasAuth:        false,
		},
		{
			name:           "DELETE_job",
			method:         http.MethodDelete,
			path:           "/v1/jobs/job-123",
			expectedStatus: http.StatusUnauthorized,
			hasAuth:        false,
		},
		{
			name:           "PATCH_not_allowed",
			method:         http.MethodPatch,
			path:           "/v1/jobs/job-123",
			expectedStatus: http.StatusMethodNotAllowed,
			hasAuth:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{}
			req := httptest.NewRequest(tt.method, tt.path, nil)
			
			if tt.hasAuth {
				ctx := context.WithValue(req.Context(), auth.UserKey, &auth.UserClaims{
					UserID: "test-user",
					Email:  "test@example.com",
				})
				req = req.WithContext(ctx)
			}
			
			rec := httptest.NewRecorder()
			h.JobHandler(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestCreateJobRequest(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected CreateJobRequest
		hasError bool
	}{
		{
			name: "full_request",
			json: `{
				"domain": "example.com",
				"use_sitemap": true,
				"find_links": false,
				"concurrency": 10,
				"max_pages": 100,
				"source_type": "api",
				"source_detail": "dashboard",
				"source_info": "{\"test\": true}"
			}`,
			expected: CreateJobRequest{
				Domain: "example.com",
				UseSitemap: func() *bool { b := true; return &b }(),
				FindLinks: func() *bool { b := false; return &b }(),
				Concurrency: func() *int { i := 10; return &i }(),
				MaxPages: func() *int { i := 100; return &i }(),
				SourceType: func() *string { s := "api"; return &s }(),
				SourceDetail: func() *string { s := "dashboard"; return &s }(),
				SourceInfo: func() *string { s := "{\"test\": true}"; return &s }(),
			},
			hasError: false,
		},
		{
			name: "minimal_request",
			json: `{"domain": "example.com"}`,
			expected: CreateJobRequest{
				Domain: "example.com",
			},
			hasError: false,
		},
		{
			name:     "invalid_json",
			json:     `{invalid}`,
			expected: CreateJobRequest{},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateJobRequest
			err := json.Unmarshal([]byte(tt.json), &req)
			
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Domain, req.Domain)
				
				if tt.expected.UseSitemap != nil {
					assert.Equal(t, *tt.expected.UseSitemap, *req.UseSitemap)
				}
				if tt.expected.FindLinks != nil {
					assert.Equal(t, *tt.expected.FindLinks, *req.FindLinks)
				}
				if tt.expected.Concurrency != nil {
					assert.Equal(t, *tt.expected.Concurrency, *req.Concurrency)
				}
				if tt.expected.MaxPages != nil {
					assert.Equal(t, *tt.expected.MaxPages, *req.MaxPages)
				}
			}
		})
	}
}

func TestJobResponse(t *testing.T) {
	now := time.Now()
	started := now.Add(-time.Hour)
	completed := now
	
	response := JobResponse{
		ID:             "job-123",
		Domain:         "example.com",
		Status:         "completed",
		TotalTasks:     100,
		CompletedTasks: 95,
		FailedTasks:    3,
		SkippedTasks:   2,
		Progress:       95.0,
		CreatedAt:      now.Format(time.RFC3339),
		StartedAt:      func() *string { s := started.Format(time.RFC3339); return &s }(),
		CompletedAt:    func() *string { s := completed.Format(time.RFC3339); return &s }(),
	}
	
	data, err := json.Marshal(response)
	require.NoError(t, err)
	
	var decoded JobResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	
	assert.Equal(t, response.ID, decoded.ID)
	assert.Equal(t, response.Domain, decoded.Domain)
	assert.Equal(t, response.Status, decoded.Status)
	assert.Equal(t, response.TotalTasks, decoded.TotalTasks)
	assert.Equal(t, response.CompletedTasks, decoded.CompletedTasks)
	assert.Equal(t, response.FailedTasks, decoded.FailedTasks)
	assert.Equal(t, response.SkippedTasks, decoded.SkippedTasks)
	assert.Equal(t, response.Progress, decoded.Progress)
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name: "x_real_ip",
			headers: map[string]string{
				"X-Real-IP": "1.2.3.4",
			},
			remoteAddr: "192.168.1.1:1234",
			expected:   "1.2.3.4",
		},
		{
			name: "x_forwarded_for_single",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			},
			remoteAddr: "192.168.1.1:1234",
			expected:   "1.2.3.4",
		},
		{
			name: "x_forwarded_for_multiple",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4, 5.6.7.8, 9.10.11.12",
			},
			remoteAddr: "192.168.1.1:1234",
			expected:   "1.2.3.4",
		},
		{
			name:       "remote_addr_with_port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:1234",
			expected:   "192.168.1.1",
		},
		{
			name:       "remote_addr_without_port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "ipv6_with_port",
			headers:    map[string]string{},
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name: "prefer_real_ip_over_forwarded",
			headers: map[string]string{
				"X-Real-IP":       "1.2.3.4",
				"X-Forwarded-For": "5.6.7.8",
			},
			remoteAddr: "192.168.1.1:1234",
			expected:   "1.2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			
			result := getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskResponse(t *testing.T) {
	now := time.Now()
	started := now.Add(-time.Second)
	completed := now
	
	task := TaskResponse{
		ID:          "task-123",
		JobID:       "job-456",
		Path:        "/page",
		URL:         "https://example.com/page",
		Status:      "completed",
		RetryCount:  0,
		ResponseTime: func() *int { i := 250; return &i }(),
		StatusCode:  func() *int { i := 200; return &i }(),
		CacheStatus: func() *string { s := "HIT"; return &s }(),
		ContentType: func() *string { s := "text/html"; return &s }(),
		Error:       nil,
		CreatedAt:   now.Format(time.RFC3339),
		StartedAt:   func() *string { s := started.Format(time.RFC3339); return &s }(),
		CompletedAt: func() *string { s := completed.Format(time.RFC3339); return &s }(),
	}
	
	data, err := json.Marshal(task)
	require.NoError(t, err)
	
	var decoded TaskResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	
	assert.Equal(t, task.ID, decoded.ID)
	assert.Equal(t, task.JobID, decoded.JobID)
	assert.Equal(t, task.URL, decoded.URL)
	assert.Equal(t, task.Status, decoded.Status)
	assert.Equal(t, task.RetryCount, decoded.RetryCount)
	assert.Equal(t, *task.ResponseTime, *decoded.ResponseTime)
	assert.Equal(t, *task.StatusCode, *decoded.StatusCode)
}


func TestFormattingStatusAndProgress(t *testing.T) {
	tests := []struct {
		name     string
		job      *jobs.Job
		expected float64
	}{
		{
			name: "no_tasks",
			job: &jobs.Job{
				TotalTasks: 0,
			},
			expected: 0.0,
		},
		{
			name: "all_completed",
			job: &jobs.Job{
				TotalTasks:     100,
				CompletedTasks: 100,
			},
			expected: 100.0,
		},
		{
			name: "partial_completion",
			job: &jobs.Job{
				TotalTasks:     100,
				CompletedTasks: 50,
				FailedTasks:    10,
				SkippedTasks:   5,
			},
			expected: 65.0,
		},
		{
			name: "with_decimals",
			job: &jobs.Job{
				TotalTasks:     333,
				CompletedTasks: 111,
			},
			expected: 33.33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := calculateProgress(tt.job.TotalTasks, tt.job.CompletedTasks, tt.job.FailedTasks, tt.job.SkippedTasks)
			assert.InDelta(t, tt.expected, progress, 0.01)
		})
	}
}

// Helper function to test progress calculation
func calculateProgress(total, completed, failed, skipped int) float64 {
	if total == 0 {
		return 0.0
	}
	processed := completed + failed + skipped
	return float64(processed) / float64(total) * 100
}

// Benchmark tests
func BenchmarkCreateJobRequestParsing(b *testing.B) {
	jsonData := []byte(`{
		"domain": "example.com",
		"use_sitemap": true,
		"find_links": false,
		"concurrency": 10,
		"max_pages": 100
	}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var req CreateJobRequest
		_ = json.Unmarshal(jsonData, &req)
	}
}

func BenchmarkJobResponseMarshalling(b *testing.B) {
	response := JobResponse{
		ID:             "job-123",
		Domain:         "example.com",
		Status:         "running",
		TotalTasks:     1000,
		CompletedTasks: 500,
		FailedTasks:    10,
		SkippedTasks:   5,
		Progress:       51.5,
		CreatedAt:      time.Now().Format(time.RFC3339),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(response)
		_ = data
	}
}

func BenchmarkGetClientIP(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8, 9.10.11.12")
	req.RemoteAddr = "192.168.1.1:1234"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getClientIP(req)
	}
}