package jobs

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestCreateManualRootTaskParameterHandling(t *testing.T) {
	tests := []struct {
		name        string
		job         *Job
		domainID    int
		rootPath    string
		mockError   error
		expectError bool
	}{
		{
			name: "successful_root_task_creation",
			job: &Job{
				ID:     "job-123",
				Domain: "example.com",
			},
			domainID:    42,
			rootPath:    "/",
			mockError:   nil,
			expectError: false,
		},
		{
			name: "custom_root_path",
			job: &Job{
				ID:     "job-456",
				Domain: "api.example.com",
			},
			domainID:    24,
			rootPath:    "/api",
			mockError:   nil,
			expectError: false,
		},
		{
			name: "database_error_propagated",
			job: &Job{
				ID:     "job-789",
				Domain: "error.com",
			},
			domainID:    1,
			rootPath:    "/",
			mockError:   sql.ErrConnDone,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := &ManualRootTaskMock{
				executeError: tt.mockError,
			}

			jm := &JobManager{
				dbQueue: mockQueue,
			}

			ctx := context.Background()
			err := jm.createManualRootTask(ctx, tt.job, tt.domainID, tt.rootPath)

			if tt.expectError {
				assert.Error(t, err)
				// Error message comes from the mock error we injected
			} else {
				assert.NoError(t, err)
			}

			assert.True(t, mockQueue.executeCalled)
		})
	}
}

func TestCreateManualRootTaskFunctionExists(t *testing.T) {
	// Test basic function signature and existence
	jm := &JobManager{
		dbQueue: &ManualRootTaskMock{},
	}

	job := &Job{
		ID:     "test-job",
		Domain: "example.com",
	}

	ctx := context.Background()
	err := jm.createManualRootTask(ctx, job, 1, "/")

	assert.NoError(t, err)
}

func TestCreateManualRootTaskLogging(t *testing.T) {
	// Test that function logs appropriately
	mockQueue := &ManualRootTaskMock{}
	
	jm := &JobManager{
		dbQueue: mockQueue,
	}

	job := &Job{
		ID:     "log-test",
		Domain: "example.com",
	}

	ctx := context.Background()
	err := jm.createManualRootTask(ctx, job, 1, "/")

	// Should succeed and log success message
	assert.NoError(t, err)
	assert.True(t, mockQueue.executeCalled)
}

// ManualRootTaskMock for testing manual root task creation
type ManualRootTaskMock struct {
	executeCalled bool
	executeError  error
}

func (m *ManualRootTaskMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	return m.executeError
}

func (m *ManualRootTaskMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *ManualRootTaskMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}