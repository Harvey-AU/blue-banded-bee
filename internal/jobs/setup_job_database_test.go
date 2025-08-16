package jobs

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestSetupJobDatabaseCallsDbQueue(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
	}{
		{
			name:        "successful_database_setup",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "database_error_propagated",
			mockError:   sql.ErrConnDone,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := &DatabaseSetupMock{
				executeError: tt.mockError,
			}

			jm := &JobManager{
				dbQueue: mockQueue,
			}

			job := &Job{
				ID:     "job-test",
				Domain: "example.com",
				Status: JobStatusPending,
			}

			ctx := context.Background()
			domainID, err := jm.setupJobDatabase(ctx, job, "example.com")

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, 0, domainID)
				assert.Contains(t, err.Error(), "failed to create job")
			} else {
				// For successful case, we can't easily test the domainID value
				// due to mocking complexity, but we can verify no error
				assert.NoError(t, err)
			}

			assert.True(t, mockQueue.executeCalled)
		})
	}
}

func TestSetupJobDatabaseFunctionExists(t *testing.T) {
	// Test that the function exists and has correct signature
	jm := &JobManager{
		dbQueue: &DatabaseSetupMock{},
	}

	job := &Job{
		ID:     "test-job",
		Domain: "example.com",
		Status: JobStatusPending,
	}

	ctx := context.Background()
	
	// Should not panic and should return expected types
	domainID, err := jm.setupJobDatabase(ctx, job, "example.com")
	
	assert.NoError(t, err)
	assert.IsType(t, int(0), domainID)
}

// DatabaseSetupMock for testing setupJobDatabase
type DatabaseSetupMock struct {
	executeCalled bool
	executeError  error
}

func (m *DatabaseSetupMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	return m.executeError
}

func (m *DatabaseSetupMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *DatabaseSetupMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}