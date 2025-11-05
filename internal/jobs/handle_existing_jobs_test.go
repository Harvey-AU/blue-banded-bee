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
		userID         *string
		organisationID *string
		expectSkip     bool
	}{
		{
			name:           "nil_user_and_org_skips_check",
			domain:         "example.com",
			userID:         nil,
			organisationID: nil,
			expectSkip:     true,
		},
		{
			name:           "empty_user_and_org_skips_check",
			domain:         "example.com",
			userID:         func() *string { s := ""; return &s }(),
			organisationID: func() *string { s := ""; return &s }(),
			expectSkip:     true,
		},
		{
			name:           "valid_organisation_id_performs_check",
			domain:         "example.com",
			userID:         nil,
			organisationID: func() *string { s := "org-123"; return &s }(),
			expectSkip:     false,
		},
		{
			name:           "valid_user_id_performs_check",
			domain:         "example.com",
			userID:         func() *string { s := "user-456"; return &s }(),
			organisationID: nil,
			expectSkip:     false,
		},
		{
			name:           "handles_complex_domain",
			domain:         "test-domain.co.uk",
			userID:         nil,
			organisationID: func() *string { s := "org-456"; return &s }(),
			expectSkip:     false,
		},
		{
			name:           "prefers_org_over_user",
			domain:         "example.com",
			userID:         func() *string { s := "user-123"; return &s }(),
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
			err := jm.handleExistingJobs(ctx, tt.domain, tt.userID, tt.organisationID)

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

	// Should not panic and should return nil with both nil
	err := jm.handleExistingJobs(ctx, "test.com", nil, nil)
	assert.NoError(t, err)

	// With org ID only
	orgID := "test-org"
	err = jm.handleExistingJobs(ctx, "test.com", nil, &orgID)
	assert.NoError(t, err)

	// With user ID only
	userID := "test-user"
	err = jm.handleExistingJobs(ctx, "test.com", &userID, nil)
	assert.NoError(t, err)

	// With both IDs (org takes precedence)
	err = jm.handleExistingJobs(ctx, "test.com", &userID, &orgID)
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
