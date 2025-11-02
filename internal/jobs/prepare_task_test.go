package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareTaskForProcessingWithCache(t *testing.T) {
	// Test task preparation when job info is cached
	now := time.Now()
	dbTask := &db.Task{
		ID:            "task-123",
		JobID:         "job-456",
		PageID:        789,
		Path:          "/test-page",
		Status:        "pending",
		CreatedAt:     now,
		RetryCount:    0,
		SourceType:    "manual",
		SourceURL:     "",
		PriorityScore: 0.8,
	}

	jobInfo := &JobInfo{
		DomainID:   42,
		DomainName: "example.com",
		FindLinks:  true,
		CrawlDelay: 2,
		RobotsRules: &crawler.RobotsRules{
			CrawlDelay: 2,
		},
	}

	wp := &WorkerPool{
		jobInfoCache: map[string]*JobInfo{
			"job-456": jobInfo,
		},
	}

	ctx := context.Background()
	result, err := wp.prepareTaskForProcessing(ctx, dbTask)

	assert.NoError(t, err)
	require.NotNil(t, result)

	// Verify conversion
	assert.Equal(t, dbTask.ID, result.ID)
	assert.Equal(t, dbTask.JobID, result.JobID)
	assert.Equal(t, dbTask.PageID, result.PageID)
	assert.Equal(t, dbTask.Path, result.Path)
	assert.Equal(t, TaskStatus(dbTask.Status), result.Status)
	assert.Equal(t, dbTask.PriorityScore, result.PriorityScore)

	// Verify job info enrichment including DomainID
	assert.Equal(t, jobInfo.DomainID, result.DomainID)
	assert.Equal(t, jobInfo.DomainName, result.DomainName)
	assert.Equal(t, jobInfo.FindLinks, result.FindLinks)
	assert.Equal(t, jobInfo.CrawlDelay, result.CrawlDelay)
}

func TestPrepareTaskForProcessingCacheMiss(t *testing.T) {
	// Test task preparation when job info is not cached
	dbTask := &db.Task{
		ID:     "task-123",
		JobID:  "job-456",
		Status: "pending",
	}

	mockQueue := &TaskPrepMock{}

	wp := &WorkerPool{
		jobInfoCache:  map[string]*JobInfo{}, // Empty cache
		dbQueue:       mockQueue,
		domainLimiter: newDomainLimiter(mockQueue),
	}

	ctx := context.Background()
	result, err := wp.prepareTaskForProcessing(ctx, dbTask)

	assert.NoError(t, err)
	require.NotNil(t, result)

	// Should have attempted database query
	assert.True(t, mockQueue.executeCalled)

	// Basic conversion should work even if enrichment fails
	assert.Equal(t, dbTask.ID, result.ID)
	assert.Equal(t, dbTask.JobID, result.JobID)
}

func TestPrepareTaskForProcessingDatabaseError(t *testing.T) {
	// Test error handling when database fallback fails
	dbTask := &db.Task{
		ID:     "task-123",
		JobID:  "job-456",
		Status: "pending",
	}

	mockQueue := &TaskPrepMock{
		executeError: sql.ErrConnDone,
	}

	wp := &WorkerPool{
		jobInfoCache: map[string]*JobInfo{}, // Empty cache to trigger fallback
		dbQueue:      mockQueue,
	}

	ctx := context.Background()
	result, err := wp.prepareTaskForProcessing(ctx, dbTask)

	// Should still return a task even if enrichment fails
	assert.NoError(t, err)
	require.NotNil(t, result)

	// Basic conversion should work
	assert.Equal(t, dbTask.ID, result.ID)
	assert.Equal(t, dbTask.JobID, result.JobID)

	// Enrichment fields should be empty/zero due to database error
	assert.Equal(t, 0, result.DomainID)
	assert.Empty(t, result.DomainName)
	assert.False(t, result.FindLinks)
	assert.Equal(t, 0, result.CrawlDelay)
}

func TestPrepareTaskForProcessingFieldMapping(t *testing.T) {
	// Test comprehensive field mapping
	now := time.Now()
	startTime := now.Add(-time.Minute)

	dbTask := &db.Task{
		ID:            "task-abc",
		JobID:         "job-def",
		PageID:        123,
		Path:          "/complex/path?param=value",
		Status:        "running",
		CreatedAt:     now,
		StartedAt:     startTime,
		RetryCount:    2,
		SourceType:    "sitemap",
		SourceURL:     "https://example.com/sitemap.xml",
		PriorityScore: 0.95,
	}

	wp := &WorkerPool{
		jobInfoCache: map[string]*JobInfo{
			"job-def": {
				DomainID:   999,
				DomainName: "complex.example.com",
				FindLinks:  true,
				CrawlDelay: 5,
			},
		},
	}

	ctx := context.Background()
	result, err := wp.prepareTaskForProcessing(ctx, dbTask)

	assert.NoError(t, err)
	require.NotNil(t, result)

	// Verify all fields are mapped correctly
	assert.Equal(t, "task-abc", result.ID)
	assert.Equal(t, "job-def", result.JobID)
	assert.Equal(t, 123, result.PageID)
	assert.Equal(t, "/complex/path?param=value", result.Path)
	assert.Equal(t, TaskStatus("running"), result.Status)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, startTime, result.StartedAt)
	assert.Equal(t, 2, result.RetryCount)
	assert.Equal(t, "sitemap", result.SourceType)
	assert.Equal(t, "https://example.com/sitemap.xml", result.SourceURL)
	assert.Equal(t, 0.95, result.PriorityScore)

	// Verify enrichment including DomainID
	assert.Equal(t, 999, result.DomainID)
	assert.Equal(t, "complex.example.com", result.DomainName)
	assert.True(t, result.FindLinks)
	assert.Equal(t, 5, result.CrawlDelay)
}

// TaskPrepMock for testing task preparation
type TaskPrepMock struct {
	executeCalled bool
	executeError  error
	domainName    string
	findLinks     bool
	crawlDelay    sql.NullInt64
}

func (m *TaskPrepMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	if m.executeError != nil {
		return m.executeError
	}

	// Simulate database query success
	return nil
}

func (m *TaskPrepMock) ExecuteMaintenance(ctx context.Context, fn func(*sql.Tx) error) error {
	return m.Execute(ctx, fn)
}

func (m *TaskPrepMock) GetNextTask(ctx context.Context, jobID string) (*db.Task, error) {
	return nil, nil
}

func (m *TaskPrepMock) UpdateTaskStatus(ctx context.Context, task *db.Task) error {
	return nil
}

func (m *TaskPrepMock) DecrementRunningTasks(ctx context.Context, jobID string) error {
	return nil
}

func (m *TaskPrepMock) SetConcurrencyOverride(fn db.ConcurrencyOverrideFunc) {}

func (m *TaskPrepMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *TaskPrepMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}
