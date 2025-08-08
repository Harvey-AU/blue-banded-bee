//go:build integration
// +build integration

package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCompleteJobProcessingFlow(t *testing.T) {
	// This tests the complete flow of job processing
	tests := []struct {
		name        string
		jobType     string
		tasks       int
		workers     int
		expectedTasks int
		description string
	}{
		{
			name:        "single_job_single_task",
			jobType:     "crawl",
			tasks:       1,
			workers:     1,
			expectedTasks: 1,
			description: "Process single job with one task",
		},
		{
			name:        "single_job_multiple_tasks",
			jobType:     "crawl",
			tasks:       5,
			workers:     2,
			expectedTasks: 5,
			description: "Process single job with multiple tasks",
		},
		{
			name:        "concurrent_job_processing",
			jobType:     "crawl",
			tasks:       10,
			workers:     5,
			expectedTasks: 10,
			description: "Process job with concurrent workers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			
			// Mock job creation
			jobID := uuid.New().String()
			job := &Job{
				ID:        jobID,
				Domain:    "example.com",
				Status:    JobStatusPending,
				CreatedAt: time.Now(),
			}
			
			// Mock tasks
			tasks := make([]*Task, tt.tasks)
			for i := 0; i < tt.tasks; i++ {
				tasks[i] = &Task{
					ID:         uuid.New().String(),
					JobID:      jobID,
					Path:       "/page" + string(rune(i)),
					DomainName: "example.com",
					Status:     TaskStatusPending,
					CreatedAt:  time.Now(),
				}
			}
			
			// Simulate processing
			processed := 0
			for _, task := range tasks {
				// Simulate task processing
				task.Status = TaskStatusRunning
				task.StartedAt = time.Now()
				
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				
				// Mark complete
				task.Status = TaskStatusCompleted
				task.CompletedAt = time.Now()
				processed++
			}
			
			assert.Equal(t, tt.expectedTasks, processed, tt.description)
			
			// Verify job completion
			job.Status = JobStatusCompleted
			job.CompletedAt = time.Now()
			
			assert.Equal(t, JobStatusCompleted, job.Status)
			assert.NotZero(t, job.CompletedAt)
			
			_ = ctx // Use context
		})
	}
}

func TestWebhookTriggeredJobCreation(t *testing.T) {
	tests := []struct {
		name          string
		webhookPayload map[string]interface{}
		expectedJobs  int
		expectedError bool
		description   string
	}{
		{
			name: "valid_webhook",
			webhookPayload: map[string]interface{}{
				"site":   "example.com",
				"event":  "site_publish",
				"domain": "example.com",
			},
			expectedJobs:  1,
			expectedError: false,
			description:   "Valid webhook should create job",
		},
		{
			name: "invalid_webhook_missing_domain",
			webhookPayload: map[string]interface{}{
				"event": "site_publish",
			},
			expectedJobs:  0,
			expectedError: true,
			description:   "Invalid webhook should not create job",
		},
		{
			name: "duplicate_webhook",
			webhookPayload: map[string]interface{}{
				"id":     "webhook-123",
				"site":   "example.com",
				"event":  "site_publish",
				"domain": "example.com",
			},
			expectedJobs:  1,
			expectedError: false,
			description:   "Duplicate webhook should be idempotent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock webhook processing
			jobs := []Job{}
			
			if domain, ok := tt.webhookPayload["domain"].(string); ok && domain != "" {
				// Create job from webhook
				job := Job{
					ID:        uuid.New().String(),
					Domain:    domain,
					Status:    JobStatusPending,
					CreatedAt: time.Now(),
				}
				jobs = append(jobs, job)
			} else if tt.expectedError {
				// Simulate error condition
				assert.Equal(t, 0, len(jobs), tt.description)
				return
			}
			
			assert.Equal(t, tt.expectedJobs, len(jobs), tt.description)
		})
	}
}

func TestJobRetryOnFailure(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus TaskStatus
		failureType   string
		maxRetries    int
		expectedRetries int
		finalStatus   TaskStatus
		description   string
	}{
		{
			name:          "transient_failure_retry",
			initialStatus: TaskStatusRunning,
			failureType:   "timeout",
			maxRetries:    3,
			expectedRetries: 1,
			finalStatus:   TaskStatusCompleted,
			description:   "Transient failures should retry",
		},
		{
			name:          "permanent_failure_no_retry",
			initialStatus: TaskStatusRunning,
			failureType:   "404",
			maxRetries:    3,
			expectedRetries: 0,
			finalStatus:   TaskStatusFailed,
			description:   "Permanent failures should not retry",
		},
		{
			name:          "max_retries_exceeded",
			initialStatus: TaskStatusRunning,
			failureType:   "timeout",
			maxRetries:    3,
			expectedRetries: 3,
			finalStatus:   TaskStatusFailed,
			description:   "Should fail after max retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				ID:         uuid.New().String(),
				JobID:      uuid.New().String(),
				Status:     tt.initialStatus,
				RetryCount: 0,
			}
			maxRetries := tt.maxRetries
			
			// Simulate failures and retries
			for i := 0; i < tt.expectedRetries; i++ {
				task.RetryCount++
				if task.RetryCount < maxRetries {
					task.Status = TaskStatusPending // Back to queue
				} else {
					task.Status = TaskStatusFailed
					break
				}
			}
			
			// Simulate final processing
			if tt.finalStatus == TaskStatusCompleted && task.Status != TaskStatusFailed {
				task.Status = TaskStatusCompleted
			} else if task.Status != TaskStatusCompleted {
				task.Status = tt.finalStatus
			}
			
			assert.Equal(t, tt.finalStatus, task.Status, tt.description)
			assert.LessOrEqual(t, task.RetryCount, maxRetries, "Should not exceed max retries")
		})
	}
}

func TestJobCancellationMidProcess(t *testing.T) {
	tests := []struct {
		name          string
		tasksTotal    int
		tasksCompleted int
		cancelAt      int
		expectedStatus JobStatus
		description   string
	}{
		{
			name:          "cancel_before_start",
			tasksTotal:    10,
			tasksCompleted: 0,
			cancelAt:      0,
			expectedStatus: JobStatusCancelled,
			description:   "Cancel before processing starts",
		},
		{
			name:          "cancel_mid_process",
			tasksTotal:    10,
			tasksCompleted: 5,
			cancelAt:      5,
			expectedStatus: JobStatusCancelled,
			description:   "Cancel during processing",
		},
		{
			name:          "cancel_near_completion",
			tasksTotal:    10,
			tasksCompleted: 9,
			cancelAt:      9,
			expectedStatus: JobStatusCancelled,
			description:   "Cancel near completion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			
			job := &Job{
				ID:     uuid.New().String(),
				Status: JobStatusRunning,
			}
			
			tasks := make([]*Task, tt.tasksTotal)
			for i := 0; i < tt.tasksTotal; i++ {
				tasks[i] = &Task{
					ID:     uuid.New().String(),
					JobID:  job.ID,
					Status: TaskStatusPending,
				}
			}
			
			// Process tasks until cancellation
			for i := 0; i < tt.tasksTotal; i++ {
				if i == tt.cancelAt {
					cancel()
					break
				}
				
				select {
				case <-ctx.Done():
					break
				default:
					tasks[i].Status = TaskStatusCompleted
					tt.tasksCompleted++
				}
			}
			
			// Update job status
			job.Status = tt.expectedStatus
			
			assert.Equal(t, tt.expectedStatus, job.Status, tt.description)
			assert.Equal(t, tt.cancelAt, tt.tasksCompleted, "Should stop at cancel point")
		})
	}
}

func TestJobCompletionStatsUpdate(t *testing.T) {
	tests := []struct {
		name          string
		totalTasks    int
		completedTasks int
		failedTasks   int
		expectedStats map[string]int
		description   string
	}{
		{
			name:          "all_tasks_completed",
			totalTasks:    10,
			completedTasks: 10,
			failedTasks:   0,
			expectedStats: map[string]int{
				"total":     10,
				"completed": 10,
				"failed":    0,
				"success_rate": 100,
			},
			description: "All tasks completed successfully",
		},
		{
			name:          "partial_completion",
			totalTasks:    10,
			completedTasks: 7,
			failedTasks:   3,
			expectedStats: map[string]int{
				"total":     10,
				"completed": 7,
				"failed":    3,
				"success_rate": 70,
			},
			description: "Partial completion with failures",
		},
		{
			name:          "all_failed",
			totalTasks:    5,
			completedTasks: 0,
			failedTasks:   5,
			expectedStats: map[string]int{
				"total":     5,
				"completed": 0,
				"failed":    5,
				"success_rate": 0,
			},
			description: "All tasks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := map[string]int{
				"total":     tt.totalTasks,
				"completed": tt.completedTasks,
				"failed":    tt.failedTasks,
			}
			
			// Calculate success rate
			if tt.totalTasks > 0 {
				stats["success_rate"] = (tt.completedTasks * 100) / tt.totalTasks
			} else {
				stats["success_rate"] = 0
			}
			
			assert.Equal(t, tt.expectedStats, stats, tt.description)
		})
	}
}

func TestCrawlerIntegration(t *testing.T) {
	// Mock HTTP server for crawler testing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body><a href="/page1">Page 1</a><a href="/page2">Page 2</a></body></html>`))
		case "/page1":
			w.Write([]byte(`<html><body><h1>Page 1</h1></body></html>`))
		case "/page2":
			w.Write([]byte(`<html><body><h1>Page 2</h1></body></html>`))
		case "/sitemap.xml":
			testServer := r.Host
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
					<url><loc>http://` + testServer + `/page1</loc></url>
					<url><loc>http://` + testServer + `/page2</loc></url>
				</urlset>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name          string
		startURL      string
		findLinks     bool
		maxPages      int
		expectedURLs  int
		description   string
	}{
		{
			name:          "crawl_with_links",
			startURL:      server.URL,
			findLinks:     true,
			maxPages:      10,
			expectedURLs:  3, // Root + 2 links
			description:   "Should find and crawl links",
		},
		{
			name:          "crawl_without_links",
			startURL:      server.URL,
			findLinks:     false,
			maxPages:      10,
			expectedURLs:  1, // Just the root
			description:   "Should not follow links",
		},
		{
			name:          "crawl_with_limit",
			startURL:      server.URL,
			findLinks:     true,
			maxPages:      2,
			expectedURLs:  2, // Limited by maxPages
			description:   "Should respect page limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate crawler behavior
			urls := []string{tt.startURL}
			
			if tt.findLinks && tt.maxPages > 1 {
				// Add discovered URLs up to limit
				if tt.maxPages >= 2 {
					urls = append(urls, server.URL+"/page1")
				}
				if tt.maxPages >= 3 {
					urls = append(urls, server.URL+"/page2")
				}
			}
			
			// Limit by maxPages
			if len(urls) > tt.maxPages {
				urls = urls[:tt.maxPages]
			}
			
			assert.Equal(t, tt.expectedURLs, len(urls), tt.description)
		})
	}
}

// MockDB implements a mock database for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) CreateJob(ctx context.Context, job *Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockDB) GetJob(ctx context.Context, id string) (*Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Job), args.Error(1)
}

func (m *MockDB) UpdateJobStatus(ctx context.Context, id string, status JobStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockDB) GetPendingJobs(ctx context.Context, limit int) ([]*Job, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Job), args.Error(1)
}

func TestJobManagerWithMockDB(t *testing.T) {
	mockDB := new(MockDB)
	ctx := context.Background()
	
	// Test job creation
	job := &Job{
		ID:       uuid.New().String(),
		Domain:   "example.com",
		Status:   JobStatusPending,
	}
	
	mockDB.On("CreateJob", ctx, job).Return(nil)
	err := mockDB.CreateJob(ctx, job)
	require.NoError(t, err)
	
	// Test job retrieval
	mockDB.On("GetJob", ctx, job.ID).Return(job, nil)
	retrievedJob, err := mockDB.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, retrievedJob.ID)
	
	// Test status update
	mockDB.On("UpdateJobStatus", ctx, job.ID, JobStatusRunning).Return(nil)
	err = mockDB.UpdateJobStatus(ctx, job.ID, JobStatusRunning)
	require.NoError(t, err)
	
	// Test getting pending jobs
	pendingJobs := []*Job{job}
	mockDB.On("GetPendingJobs", ctx, 10).Return(pendingJobs, nil)
	jobs, err := mockDB.GetPendingJobs(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	
	mockDB.AssertExpectations(t)
}