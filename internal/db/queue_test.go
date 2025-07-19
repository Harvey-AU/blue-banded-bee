package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueURLs_DuplicateHandling(t *testing.T) {
	// Skip if no database URL is provided
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// Setup test database
	db, err := InitFromEnv()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	queue := db.Queue()

	// Create test domain and job
	domainID, err := queue.GetOrCreateDomain(ctx, "test.com")
	require.NoError(t, err)

	jobID := "test-job-" + time.Now().Format("20060102150405")
	err = queue.CreateJob(ctx, &Job{
		ID:        jobID,
		DomainID:  domainID,
		Status:    "running",
		MaxPages:  100,
		FindLinks: true,
	})
	require.NoError(t, err)

	// Create test pages
	pages := []Page{
		{Path: "/page1", Priority: 1.0},
		{Path: "/page2", Priority: 0.9},
		{Path: "/page3", Priority: 0.8},
	}

	// Get page IDs
	for i := range pages {
		pageID, err := queue.GetOrCreatePage(ctx, domainID, pages[i].Path)
		require.NoError(t, err)
		pages[i].ID = pageID
	}

	// Test 1: First enqueue should succeed
	err = queue.EnqueueURLs(ctx, jobID, pages, "test", "http://test.com")
	assert.NoError(t, err, "First enqueue should succeed")

	// Test 2: Second enqueue of same pages should NOT fail (duplicates ignored)
	err = queue.EnqueueURLs(ctx, jobID, pages, "test", "http://test.com")
	assert.NoError(t, err, "Duplicate enqueue should not return error due to ON CONFLICT DO NOTHING")

	// Test 3: Verify only one task per page was created
	var taskCount int
	err = db.client.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = $1
	`, jobID).Scan(&taskCount)
	require.NoError(t, err)
	assert.Equal(t, 3, taskCount, "Should have exactly 3 tasks, not duplicates")

	// Test 4: Enqueue with some new and some duplicate pages
	mixedPages := []Page{
		pages[0], // duplicate
		{Path: "/page4", Priority: 0.7}, // new
		pages[1], // duplicate
	}
	
	// Get page ID for new page
	pageID, err := queue.GetOrCreatePage(ctx, domainID, "/page4")
	require.NoError(t, err)
	mixedPages[1].ID = pageID

	err = queue.EnqueueURLs(ctx, jobID, mixedPages, "test", "http://test.com")
	assert.NoError(t, err, "Mixed enqueue should succeed")

	// Verify we now have 4 tasks total (3 original + 1 new)
	err = db.client.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = $1
	`, jobID).Scan(&taskCount)
	require.NoError(t, err)
	assert.Equal(t, 4, taskCount, "Should have 4 tasks total after adding one new page")

	// Cleanup
	_, err = db.client.ExecContext(ctx, `DELETE FROM tasks WHERE job_id = $1`, jobID)
	require.NoError(t, err)
	_, err = db.client.ExecContext(ctx, `DELETE FROM jobs WHERE id = $1`, jobID)
	require.NoError(t, err)
}