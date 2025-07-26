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