package jobs

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestHandleExistingJobsParameterValidation(t *testing.T) {
	tests := []struct {
		name           string
		domain         string
		organisationID *string
		expectSkip     bool
	}{
		{
			name:           "nil_organisation_id_skips_check",
			domain:         "example.com",
			organisationID: nil,
			expectSkip:     true,
		},
		{
			name:           "empty_organisation_id_skips_check",
			domain:         "example.com",
			organisationID: func() *string { s := ""; return &s }(),
			expectSkip:     true,
		},
		{
			name:           "valid_organisation_id_performs_check",
			domain:         "example.com",
			organisationID: func() *string { s := "org-123"; return &s }(),
			expectSkip:     false,
		},
		{
			name:           "handles_complex_domain",
			domain:         "test-domain.co.uk",
			organisationID: func() *string { s := "org-456"; return &s }(),
			expectSkip:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create simple mock that tracks whether Execute was called
			mockQueue := &SimpleDbQueueMock{
				executeCalled: false,
			}

			jm := &JobManager{
				dbQueue: mockQueue,
			}

			ctx := context.Background()
			err := jm.handleExistingJobs(ctx, tt.domain, tt.organisationID)

			// Function should always return nil (errors are logged but not propagated)
			assert.NoError(t, err)

			if tt.expectSkip {
				assert.False(t, mockQueue.executeCalled, "Should skip database check")
			} else {
				assert.True(t, mockQueue.executeCalled, "Should perform database check")
			}
		})
	}
}

func TestHandleExistingJobsFunctionExists(t *testing.T) {
	// Simple test to verify the extracted function exists and can be called
	jm := &JobManager{
		dbQueue: &SimpleDbQueueMock{},
	}

	ctx := context.Background()

	// Should not panic and should return nil
	err := jm.handleExistingJobs(ctx, "test.com", nil)
	assert.NoError(t, err)

	orgID := "test-org"
	err = jm.handleExistingJobs(ctx, "test.com", &orgID)
	// Will fail on actual DB operation but should not panic
	assert.NoError(t, err)
}

// SimpleDbQueueMock tracks only what we need for this test
type SimpleDbQueueMock struct {
	executeCalled bool
}

func (m *SimpleDbQueueMock) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	m.executeCalled = true
	// Return sql.ErrNoRows to simulate "no existing jobs found"
	// The handleExistingJobs function handles this gracefully
	return nil
}

func (m *SimpleDbQueueMock) ExecuteMaintenance(ctx context.Context, fn func(*sql.Tx) error) error {
	return m.Execute(ctx, fn)
}

func (m *SimpleDbQueueMock) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	return nil
}

func (m *SimpleDbQueueMock) CleanupStuckJobs(ctx context.Context) error {
	return nil
}
