package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJob(t *testing.T) {
	// Skip in CI environment
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping database test in CI environment")
	}

	// 1. Connect to test database
	database, err := db.InitFromEnv()
	require.NoError(t, err, "Failed to connect to test database")
	defer database.Close()

	// 2. Create test data using a simpler approach
	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)
	jm := NewJobManager(sqlDB, dbQueue, nil, nil)
	
	// Create a job using CreateJob which handles all required fields
	options := &JobOptions{
		Domain:      "test.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  true,
	}
	
	createdJob, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create test job")
	require.NotNil(t, createdJob)
	
	// 3. Execute GetJob function
	job, err := jm.GetJob(ctx, createdJob.ID)

	// 4. Assert results
	require.NoError(t, err, "GetJob should not return error")
	assert.Equal(t, createdJob.ID, job.ID)
	assert.Equal(t, "test.example.com", job.Domain)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 5, job.Concurrency)
	assert.True(t, job.FindLinks)

	// Cleanup
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", job.ID)
	require.NoError(t, err, "Failed to cleanup test job")
}

func TestCreateJob(t *testing.T) {
	// Skip in CI environment
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping database test in CI environment")
	}

	// Connect to test database
	database, err := db.InitFromEnv()
	require.NoError(t, err, "Failed to connect to test database")
	defer database.Close()

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)
	
	// For integration test, we'll use nil crawler since we're not testing sitemap functionality
	// and nil worker pool since we'll handle cancellation differently
	jm := NewJobManager(sqlDB, dbQueue, nil, nil)
	
	// Test case 1: Create a new job successfully
	options := &JobOptions{
		Domain:      "test-create.example.com",
		Concurrency: 3,
		FindLinks:   false,
		MaxPages:    50,
		UseSitemap:  false, // Don't trigger sitemap processing
	}
	
	// Create the job
	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "CreateJob should not return error")
	require.NotNil(t, job, "Job should not be nil")
	
	// Verify job properties
	assert.NotEmpty(t, job.ID, "Job ID should be set")
	assert.Equal(t, "test-create.example.com", job.Domain)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, 3, job.Concurrency)
	assert.False(t, job.FindLinks)
	assert.Equal(t, 50, job.MaxPages)
	
	// Verify job exists in database
	var count int
	err = sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE id = $1", job.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Job should exist in database")
	
	// Cleanup - need to delete tasks first due to foreign key constraints
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id IN (SELECT id FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%')")
	require.NoError(t, err)
}

func TestCancelJob(t *testing.T) {
	// Skip in CI environment
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping database test in CI environment")
	}

	// Connect to test database
	database, err := db.InitFromEnv()
	require.NoError(t, err, "Failed to connect to test database")
	defer database.Close()

	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)
	
	// Create JobManager with minimal worker pool for RemoveJob functionality
	wp := &WorkerPool{
		jobs:     make(map[string]bool),
		stopCh:   make(chan struct{}),
		notifyCh: make(chan struct{}, 1),
	}
	jm := NewJobManager(sqlDB, dbQueue, nil, wp)
	
	// Test case 1: Cancel a pending job
	options := &JobOptions{
		Domain:      "test-cancel.example.com",
		Concurrency: 5,
		FindLinks:   true,
		MaxPages:    100,
		UseSitemap:  false, // Don't trigger sitemap processing
	}
	
	// Create a job
	job, err := jm.CreateJob(ctx, options)
	require.NoError(t, err, "Failed to create job")
	require.NotNil(t, job)
	
	// Cancel the job
	err = jm.CancelJob(ctx, job.ID)
	require.NoError(t, err, "CancelJob should not return error")
	
	// Verify job status is cancelled
	cancelledJob, err := jm.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCancelled, cancelledJob.Status)
	assert.False(t, cancelledJob.CompletedAt.IsZero(), "CompletedAt should be set")
	
	// Test case 2: Try to cancel an already cancelled job
	err = jm.CancelJob(ctx, job.ID)
	assert.Error(t, err, "Should error when cancelling already cancelled job")
	assert.Contains(t, err.Error(), "job cannot be canceled")
	
	// Test case 3: Try to cancel a non-existent job
	err = jm.CancelJob(ctx, "non-existent-id")
	assert.Error(t, err, "Should error when cancelling non-existent job")
	
	// Cleanup
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id IN (SELECT id FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%'))")
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE domain_id IN (SELECT id FROM domains WHERE name LIKE 'test-%')")
	require.NoError(t, err)
}