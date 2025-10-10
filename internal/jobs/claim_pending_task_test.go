package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestClaimPendingTaskWithActiveJobs(t *testing.T) {
	tests := []struct {
		name        string
		activeJobs  map[string]bool
		mockTask    *db.Task
		mockError   error
		expectTask  bool
		expectError bool
	}{
		{
			name:        "no_active_jobs_returns_no_rows",
			activeJobs:  map[string]bool{},
			expectTask:  false,
			expectError: true, // sql.ErrNoRows
		},
		{
			name: "active_job_with_available_task",
			activeJobs: map[string]bool{
				"job-123": true,
			},
			mockTask: &db.Task{
				ID:            "task-456",
				JobID:         "job-123",
				PageID:        789,
				Path:          "/test-page",
				Status:        "pending",
				PriorityScore: 0.5,
				CreatedAt:     time.Now(),
			},
			expectTask:  true,
			expectError: false,
		},
		{
			name: "active_job_with_no_tasks",
			activeJobs: map[string]bool{
				"job-123": true,
			},
			mockError:   sql.ErrNoRows,
			expectTask:  false,
			expectError: true,
		},
		{
			name: "multiple_jobs_first_has_task",
			activeJobs: map[string]bool{
				"job-123": true,
				"job-456": true,
			},
			mockTask: &db.Task{
				ID:     "task-789",
				JobID:  "job-123",
				Status: "pending",
			},
			expectTask:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DbQueue
			mockQueue := &ClaimTaskMock{
				returnTask:  tt.mockTask,
				returnError: tt.mockError,
			}

			wp := &WorkerPool{
				jobs:    tt.activeJobs,
				dbQueue: mockQueue,
			}

			ctx := context.Background()
			task, err := wp.claimPendingTask(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				if tt.expectTask {
					assert.NotNil(t, task)
					assert.Equal(t, tt.mockTask.ID, task.ID)
				} else {
					assert.Nil(t, task)
				}
			}

			// Verify database was queried if there were active jobs
			if len(tt.activeJobs) > 0 {
				assert.True(t, mockQueue.getNextTaskCalled)
			}
		})
	}
}

func TestClaimPendingTaskJobIteration(t *testing.T) {
	// Test that function iterates through all active jobs correctly
	mockQueue := &ClaimTaskMock{
		returnError: sql.ErrNoRows, // No tasks in any job
	}

	activeJobs := map[string]bool{
		"job-1": true,
		"job-2": true,
		"job-3": true,
	}

	wp := &WorkerPool{
		jobs:    activeJobs,
		dbQueue: mockQueue,
	}

	ctx := context.Background()
	task, err := wp.claimPendingTask(ctx)

	// Should try all jobs and return ErrNoRows
	assert.Error(t, err)
	assert.Equal(t, sql.ErrNoRows, err)
	assert.Nil(t, task)
	assert.True(t, mockQueue.getNextTaskCalled)
}

func TestClaimPendingTaskDatabaseError(t *testing.T) {
	// Test handling of database errors
	mockQueue := &ClaimTaskMock{
		returnError: sql.ErrConnDone,
	}

	wp := &WorkerPool{
		jobs:    map[string]bool{"job-123": true},
		dbQueue: mockQueue,
	}

	ctx := context.Background()
	task, err := wp.claimPendingTask(ctx)

	assert.Error(t, err)
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, task)
}

// ClaimTaskMock for testing task claiming
type ClaimTaskMock struct {
	getNextTaskCalled bool
	returnTask        *db.Task
	returnError       error
}

func (m *ClaimTaskMock) GetNextTask(ctx context.Context, jobID string) (*db.Task, error) {
	m.getNextTaskCalled = true
	return m.returnTask, m.returnError
}

func (m *ClaimTaskMock) UpdateTaskStatus(ctx context.Context, task *db.Task) error {
	return nil
}

func (m *ClaimTaskMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	return nil
}

func (m *ClaimTaskMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *ClaimTaskMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}
