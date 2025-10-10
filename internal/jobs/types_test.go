package jobs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobStatus(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		valid  bool
	}{
		{"pending", JobStatusPending, true},
		{"initializing", JobStatusInitialising, true},
		{"running", JobStatusRunning, true},
		{"paused", JobStatusPaused, true},
		{"completed", JobStatusCompleted, true},
		{"failed", JobStatusFailed, true},
		{"cancelled", JobStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test string conversion
			assert.Equal(t, tt.name, string(tt.status))

			// Test that status is not empty
			assert.NotEmpty(t, tt.status)
		})
	}
}

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		name   string
		status TaskStatus
		valid  bool
	}{
		{"pending", TaskStatusPending, true},
		{"running", TaskStatusRunning, true},
		{"completed", TaskStatusCompleted, true},
		{"failed", TaskStatusFailed, true},
		{"skipped", TaskStatusSkipped, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test string conversion
			assert.Equal(t, tt.name, string(tt.status))

			// Test that status is not empty
			assert.NotEmpty(t, tt.status)
		})
	}
}

func TestConstants(t *testing.T) {
	// Test timeout constants
	assert.Equal(t, 3*time.Minute, TaskStaleTimeout)
	assert.Equal(t, 5, MaxTaskRetries)

	// Ensure they're reasonable values
	assert.Greater(t, TaskStaleTimeout, time.Duration(0))
	assert.Greater(t, MaxTaskRetries, 0)
}

func TestJob(t *testing.T) {
	now := time.Now()
	userID := "user-123"
	orgID := "org-456"
	sourceType := "api"
	sourceDetail := "dashboard"
	sourceInfo := "{\"test\": true}"
	durationSec := 120
	avgTime := 2.5

	job := &Job{
		ID:                    "job-123",
		Domain:                "example.com",
		UserID:                &userID,
		OrganisationID:        &orgID,
		Status:                JobStatusRunning,
		Progress:              50.5,
		TotalTasks:            100,
		CompletedTasks:        50,
		FailedTasks:           0,
		SkippedTasks:          0,
		FoundTasks:            50,
		SitemapTasks:          30,
		CreatedAt:             now,
		StartedAt:             now.Add(time.Second),
		CompletedAt:           now.Add(time.Hour),
		Concurrency:           10,
		FindLinks:             true,
		MaxPages:              1000,
		IncludePaths:          []string{"/blog", "/docs"},
		ExcludePaths:          []string{"/admin", "/private"},
		RequiredWorkers:       5,
		SourceType:            &sourceType,
		SourceDetail:          &sourceDetail,
		SourceInfo:            &sourceInfo,
		ErrorMessage:          "",
		DurationSeconds:       &durationSec,
		AvgTimePerTaskSeconds: &avgTime,
	}

	// Test JSON serialization
	data, err := json.Marshal(job)
	require.NoError(t, err)

	var decoded Job
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Test core fields
	assert.Equal(t, job.ID, decoded.ID)
	assert.Equal(t, job.Domain, decoded.Domain)
	assert.Equal(t, job.Status, decoded.Status)
	assert.Equal(t, job.Progress, decoded.Progress)
	assert.Equal(t, job.TotalTasks, decoded.TotalTasks)
	assert.Equal(t, job.CompletedTasks, decoded.CompletedTasks)
	assert.Equal(t, job.FindLinks, decoded.FindLinks)
	assert.Equal(t, job.MaxPages, decoded.MaxPages)

	// Test optional fields
	if job.UserID != nil {
		require.NotNil(t, decoded.UserID)
		assert.Equal(t, *job.UserID, *decoded.UserID)
	}
	if job.OrganisationID != nil {
		require.NotNil(t, decoded.OrganisationID)
		assert.Equal(t, *job.OrganisationID, *decoded.OrganisationID)
	}

	// Test arrays
	assert.Equal(t, job.IncludePaths, decoded.IncludePaths)
	assert.Equal(t, job.ExcludePaths, decoded.ExcludePaths)
}

func TestTask(t *testing.T) {
	now := time.Now()

	task := &Task{
		ID:            "task-123",
		JobID:         "job-456",
		PageID:        1,
		Path:          "/page",
		DomainName:    "example.com",
		Status:        TaskStatusCompleted,
		CreatedAt:     now,
		StartedAt:     now.Add(time.Second),
		CompletedAt:   now.Add(5 * time.Second),
		RetryCount:    2,
		StatusCode:    200,
		ResponseTime:  250,
		CacheStatus:   "HIT",
		ContentType:   "text/html",
		Error:         "",
		SourceType:    "sitemap",
		SourceURL:     "https://example.com/sitemap.xml",
		PriorityScore: 1.0,
	}

	// Test JSON serialization
	data, err := json.Marshal(task)
	require.NoError(t, err)

	var decoded Task
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Test core fields
	assert.Equal(t, task.ID, decoded.ID)
	assert.Equal(t, task.JobID, decoded.JobID)
	assert.Equal(t, task.PageID, decoded.PageID)
	assert.Equal(t, task.Path, decoded.Path)
	assert.Equal(t, task.DomainName, decoded.DomainName)
	assert.Equal(t, task.Status, decoded.Status)
	assert.Equal(t, task.RetryCount, decoded.RetryCount)

	// Test result fields
	assert.Equal(t, task.StatusCode, decoded.StatusCode)
	assert.Equal(t, task.ResponseTime, decoded.ResponseTime)
	assert.Equal(t, task.CacheStatus, decoded.CacheStatus)
	assert.Equal(t, task.ContentType, decoded.ContentType)

	// Test source fields
	assert.Equal(t, task.SourceType, decoded.SourceType)
	assert.Equal(t, task.SourceURL, decoded.SourceURL)
	assert.Equal(t, task.PriorityScore, decoded.PriorityScore)
}

func TestJobWithEmptyOptionalFields(t *testing.T) {
	job := &Job{
		ID:              "job-123",
		Domain:          "example.com",
		Status:          JobStatusPending,
		Progress:        0,
		TotalTasks:      0,
		CompletedTasks:  0,
		FailedTasks:     0,
		SkippedTasks:    0,
		FoundTasks:      0,
		SitemapTasks:    0,
		CreatedAt:       time.Now(),
		Concurrency:     1,
		FindLinks:       false,
		MaxPages:        100,
		RequiredWorkers: 1,
	}

	// Test JSON serialization with nil optional fields
	data, err := json.Marshal(job)
	require.NoError(t, err)

	var decoded Job
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Test that nil fields remain nil
	assert.Nil(t, decoded.UserID)
	assert.Nil(t, decoded.OrganisationID)
	assert.Nil(t, decoded.SourceType)
	assert.Nil(t, decoded.SourceDetail)
	assert.Nil(t, decoded.SourceInfo)
	assert.Nil(t, decoded.DurationSeconds)
	assert.Nil(t, decoded.AvgTimePerTaskSeconds)

	// Test that empty slices are handled correctly
	assert.Nil(t, decoded.IncludePaths)
	assert.Nil(t, decoded.ExcludePaths)
}

func TestTaskWithError(t *testing.T) {
	errorMsg := "Connection timeout"
	task := &Task{
		ID:         "task-123",
		JobID:      "job-456",
		PageID:     1,
		Path:       "/failed-page",
		DomainName: "example.com",
		Status:     TaskStatusFailed,
		CreatedAt:  time.Now(),
		StartedAt:  time.Now().Add(time.Second),
		Error:      errorMsg,
	}

	// Test JSON serialization with error
	data, err := json.Marshal(task)
	require.NoError(t, err)

	var decoded Task
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Test error field
	assert.Equal(t, errorMsg, decoded.Error)

	// Test that optional success fields are zero values
	assert.Equal(t, 0, decoded.StatusCode)
	assert.Equal(t, int64(0), decoded.ResponseTime)
	assert.Empty(t, decoded.CacheStatus)
	assert.Empty(t, decoded.ContentType)
}

func BenchmarkJobJSONMarshal(b *testing.B) {
	job := &Job{
		ID:             "job-123",
		Domain:         "example.com",
		Status:         JobStatusRunning,
		Progress:       50.5,
		TotalTasks:     100,
		CompletedTasks: 50,
		CreatedAt:      time.Now(),
		Concurrency:    10,
		FindLinks:      true,
		MaxPages:       1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(job)
	}
}

func BenchmarkTaskJSONMarshal(b *testing.B) {
	task := &Task{
		ID:         "task-123",
		JobID:      "job-456",
		PageID:     1,
		Path:       "/page",
		DomainName: "example.com",
		Status:     TaskStatusCompleted,
		CreatedAt:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(task)
	}
}
