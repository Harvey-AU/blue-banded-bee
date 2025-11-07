//go:build integration

package jobs

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestCleanup(t *testing.T) *db.DB {
	t.Helper()
	testutil.LoadTestEnv(t)

	// Skip if no DATABASE_URL is set
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	database, err := db.InitFromEnv()
	require.NoError(t, err, "Failed to connect to test database")

	// Register cleanup to close database - this will run LAST (LIFO order)
	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// TestCleanupOrphanedTasksFromFailedJobs verifies that orphaned tasks from failed jobs
// are cleaned up and marked as failed with appropriate error messages
func TestCleanupOrphanedTasksFromFailedJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	database := setupTestCleanup(t)
	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Cleanup function
	t.Cleanup(func() {
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM domains WHERE name LIKE 'cleanup-test-%'")
	})

	// Create test domain
	var domainID int
	err := sqlDB.QueryRowContext(ctx, `
		INSERT INTO domains (name)
		VALUES ($1)
		RETURNING id
	`, "cleanup-test-domain.com").Scan(&domainID)
	require.NoError(t, err)

	// Create a failed job
	jobID := "cleanup-test-failed-job"
	_, err = sqlDB.ExecContext(ctx, `
		INSERT INTO jobs (
			id, domain_id, status, progress, total_tasks, completed_tasks,
			failed_tasks, skipped_tasks, created_at, concurrency, find_links,
			max_pages, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`, jobID, domainID, JobStatusFailed, 10.0, 100, 10, 0, 0,
		time.Now().UTC().Add(-1*time.Hour), 5, true, 1000,
		"Job timed out: no task progress for 30 minutes")
	require.NoError(t, err)

	// Create test page
	var pageID int
	err = sqlDB.QueryRowContext(ctx, `
		INSERT INTO pages (domain_id, path)
		VALUES ($1, $2)
		RETURNING id
	`, domainID, "/test-page").Scan(&pageID)
	require.NoError(t, err)

	// Create orphaned tasks in various statuses
	tasks := []struct {
		id     string
		status string
	}{
		{"cleanup-test-task-pending-1", string(TaskStatusPending)},
		{"cleanup-test-task-pending-2", string(TaskStatusPending)},
		{"cleanup-test-task-waiting-1", string(TaskStatusWaiting)},
		{"cleanup-test-task-waiting-2", string(TaskStatusWaiting)},
		{"cleanup-test-task-completed", string(TaskStatusCompleted)}, // Should not be touched
	}

	for _, task := range tasks {
		_, err = sqlDB.ExecContext(ctx, `
			INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at,
				retry_count, source_type
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8
			)
		`, task.id, jobID, pageID, "/test", task.status,
			time.Now().UTC().Add(-30*time.Minute), 0, "sitemap")
		require.NoError(t, err)
	}

	// Create worker pool
	wp := &WorkerPool{
		db:              sqlDB,
		dbQueue:         dbQueue,
		cleanupInterval: time.Minute,
	}

	// Run cleanup
	err = wp.cleanupOrphanedTasks(ctx)
	require.NoError(t, err)

	// Verify pending and waiting tasks are now failed
	for _, task := range tasks[:4] { // First 4 are pending/waiting
		var status, errorMsg string
		var completedAt sql.NullTime

		err = sqlDB.QueryRowContext(ctx, `
			SELECT status, COALESCE(error, ''), completed_at
			FROM tasks
			WHERE id = $1
		`, task.id).Scan(&status, &errorMsg, &completedAt)
		require.NoError(t, err)

		assert.Equal(t, string(TaskStatusFailed), status, "Task %s should be failed", task.id)
		assert.Contains(t, errorMsg, "Job timed out", "Task %s should have job error message", task.id)
		assert.True(t, completedAt.Valid, "Task %s should have completed_at set", task.id)
	}

	// Verify completed task is untouched
	var completedTaskStatus string
	err = sqlDB.QueryRowContext(ctx, `
		SELECT status FROM tasks WHERE id = $1
	`, tasks[4].id).Scan(&completedTaskStatus)
	require.NoError(t, err)
	assert.Equal(t, string(TaskStatusCompleted), completedTaskStatus, "Completed task should not be modified")

	// Verify job counters were updated by trigger
	var failedTasks int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT failed_tasks FROM jobs WHERE id = $1
	`, jobID).Scan(&failedTasks)
	require.NoError(t, err)
	assert.Equal(t, 4, failedTasks, "Job should have 4 failed tasks (trigger should update counter)")
}

// TestCleanupIgnoresCancelledJobs verifies that cancelled jobs are NOT processed
// by cleanup (they're handled by CancelJob which marks tasks as 'skipped')
func TestCleanupIgnoresCancelledJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	database := setupTestCleanup(t)
	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Cleanup function
	t.Cleanup(func() {
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM domains WHERE name LIKE 'cleanup-test-%'")
	})

	// Create test domain
	var domainID int
	err := sqlDB.QueryRowContext(ctx, `
		INSERT INTO domains (name)
		VALUES ($1)
		RETURNING id
	`, "cleanup-test-cancelled-domain.com").Scan(&domainID)
	require.NoError(t, err)

	// Create a cancelled job
	jobID := "cleanup-test-cancelled-job"
	_, err = sqlDB.ExecContext(ctx, `
		INSERT INTO jobs (
			id, domain_id, status, progress, total_tasks, completed_tasks,
			failed_tasks, skipped_tasks, created_at, completed_at, concurrency,
			find_links, max_pages
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`, jobID, domainID, JobStatusCancelled, 50.0, 100, 50, 0, 0,
		time.Now().UTC().Add(-1*time.Hour), time.Now().UTC(), 5, true, 1000)
	require.NoError(t, err)

	// Create test page
	var pageID int
	err = sqlDB.QueryRowContext(ctx, `
		INSERT INTO pages (domain_id, path)
		VALUES ($1, $2)
		RETURNING id
	`, domainID, "/test-page").Scan(&pageID)
	require.NoError(t, err)

	// Create pending tasks that should NOT be touched by cleanup
	taskID := "cleanup-test-task-pending-cancelled"
	_, err = sqlDB.ExecContext(ctx, `
		INSERT INTO tasks (
			id, job_id, page_id, path, status, created_at,
			retry_count, source_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`, taskID, jobID, pageID, "/test", string(TaskStatusPending),
		time.Now().UTC().Add(-30*time.Minute), 0, "sitemap")
	require.NoError(t, err)

	// Create worker pool
	wp := &WorkerPool{
		db:              sqlDB,
		dbQueue:         dbQueue,
		cleanupInterval: time.Minute,
	}

	// Run cleanup
	err = wp.cleanupOrphanedTasks(ctx)
	require.NoError(t, err)

	// Verify task is still pending (not touched by cleanup)
	var status string
	err = sqlDB.QueryRowContext(ctx, `
		SELECT status FROM tasks WHERE id = $1
	`, taskID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(TaskStatusPending), status, "Task from cancelled job should not be touched by cleanup")
}

// TestCleanupProcessesOneJobAtATime verifies that cleanup processes jobs incrementally
// to avoid timeout with large task counts
func TestCleanupProcessesOneJobAtATime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	database := setupTestCleanup(t)
	ctx := context.Background()
	sqlDB := database.GetDB()
	dbQueue := db.NewDbQueue(database)

	// Cleanup function
	t.Cleanup(func() {
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM tasks WHERE job_id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM jobs WHERE id LIKE 'cleanup-test-%'")
		_, _ = sqlDB.ExecContext(ctx, "DELETE FROM domains WHERE name LIKE 'cleanup-test-%'")
	})

	// Create test domain
	var domainID int
	err := sqlDB.QueryRowContext(ctx, `
		INSERT INTO domains (name)
		VALUES ($1)
		RETURNING id
	`, "cleanup-test-batch-domain.com").Scan(&domainID)
	require.NoError(t, err)

	// Create two failed jobs
	job1ID := "cleanup-test-failed-job-1"
	job2ID := "cleanup-test-failed-job-2"

	for _, jobID := range []string{job1ID, job2ID} {
		_, err = sqlDB.ExecContext(ctx, `
			INSERT INTO jobs (
				id, domain_id, status, progress, total_tasks, completed_tasks,
				failed_tasks, skipped_tasks, created_at, concurrency, find_links,
				max_pages, error_message
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
			)
		`, jobID, domainID, JobStatusFailed, 0.0, 10, 0, 0, 0,
			time.Now().UTC().Add(-1*time.Hour), 5, true, 1000,
			"Job timed out")
		require.NoError(t, err)
	}

	// Create test page
	var pageID int
	err = sqlDB.QueryRowContext(ctx, `
		INSERT INTO pages (domain_id, path)
		VALUES ($1, $2)
		RETURNING id
	`, domainID, "/test-page").Scan(&pageID)
	require.NoError(t, err)

	// Create pending tasks for both jobs
	for i, jobID := range []string{job1ID, job2ID} {
		taskID := "cleanup-test-task-" + string(rune('a'+i))
		_, err = sqlDB.ExecContext(ctx, `
			INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at,
				retry_count, source_type
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8
			)
		`, taskID, jobID, pageID, "/test", string(TaskStatusPending),
			time.Now().UTC().Add(-30*time.Minute), 0, "sitemap")
		require.NoError(t, err)
	}

	// Create worker pool
	wp := &WorkerPool{
		db:              sqlDB,
		dbQueue:         dbQueue,
		cleanupInterval: time.Minute,
	}

	// Run cleanup first time - should process only one job
	err = wp.cleanupOrphanedTasks(ctx)
	require.NoError(t, err)

	// Count how many tasks are now failed
	var failedCount int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks
		WHERE job_id IN ($1, $2) AND status = $3
	`, job1ID, job2ID, string(TaskStatusFailed)).Scan(&failedCount)
	require.NoError(t, err)

	// Should have processed exactly 1 task (from 1 job)
	assert.Equal(t, 1, failedCount, "First cleanup should process tasks from one job only")

	// Run cleanup second time - should process the other job
	err = wp.cleanupOrphanedTasks(ctx)
	require.NoError(t, err)

	// Now both tasks should be failed
	err = sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks
		WHERE job_id IN ($1, $2) AND status = $3
	`, job1ID, job2ID, string(TaskStatusFailed)).Scan(&failedCount)
	require.NoError(t, err)

	assert.Equal(t, 2, failedCount, "Second cleanup should process remaining job")
}
