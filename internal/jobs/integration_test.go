//go:build integration
// +build integration

package jobs

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerPoolWithRealDB tests the worker pool with a real database connection
func TestWorkerPoolWithRealDB(t *testing.T) {
	// Load test environment
	testutil.LoadTestEnv(t)
	
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to database
	config := &db.Config{
		DatabaseURL: databaseURL,
	}
	database, err := db.New(config)
	require.NoError(t, err)
	require.NotNil(t, database)
	defer database.Close()

	// Create a test job
	jobID, err := database.CreateJob(ctx, "test-domain.com", nil, nil, 5, true, 100, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, jobID)

	// Create test tasks
	for i := 0; i < 5; i++ {
		path := "/page" + strconv.Itoa(i)
		err = database.CreateTask(ctx, jobID, path, "test-domain.com", "manual", "")
		require.NoError(t, err)
	}

	// Verify tasks were created
	tasks, err := database.GetTasksByJobID(ctx, jobID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, tasks, 5)

	// Test GetNextTask with locking
	task1, err := database.GetNextTask(ctx, "worker-1")
	require.NoError(t, err)
	assert.NotNil(t, task1)
	assert.Equal(t, "running", task1.Status)
	assert.Equal(t, "worker-1", *task1.WorkerID)

	// Another worker should get a different task
	task2, err := database.GetNextTask(ctx, "worker-2")
	require.NoError(t, err)
	assert.NotNil(t, task2)
	assert.NotEqual(t, task1.ID, task2.ID)

	// Complete the first task
	err = database.UpdateTaskStatus(ctx, task1.ID, "completed", nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Fail the second task
	errorMsg := "test error"
	err = database.UpdateTaskStatus(ctx, task2.ID, "failed", &errorMsg, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Clean up
	err = database.UpdateJobStatus(ctx, jobID, "cancelled")
	require.NoError(t, err)
}

// TestRecoverStaleTasks tests recovery of stale tasks
func TestRecoverStaleTasks(t *testing.T) {
	testutil.LoadTestEnv(t)
	
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to database
	config := &db.Config{
		DatabaseURL: databaseURL,
	}
	database, err := db.New(config)
	require.NoError(t, err)
	defer database.Close()

	// Create a test job
	jobID, err := database.CreateJob(ctx, "stale-test.com", nil, nil, 2, false, 10, nil, nil)
	require.NoError(t, err)

	// Create tasks
	for i := 0; i < 3; i++ {
		err = database.CreateTask(ctx, jobID, "/page"+strconv.Itoa(i), "stale-test.com", "manual", "")
		require.NoError(t, err)
	}

	// Simulate tasks being picked up
	task1, err := database.GetNextTask(ctx, "worker-stale-1")
	require.NoError(t, err)
	require.NotNil(t, task1)

	task2, err := database.GetNextTask(ctx, "worker-stale-2")
	require.NoError(t, err)
	require.NotNil(t, task2)

	// Make tasks stale by updating their started_at to be old
	_, err = database.Exec(ctx, `
		UPDATE tasks 
		SET started_at = NOW() - INTERVAL '1 hour'
		WHERE id IN ($1, $2)
	`, task1.ID, task2.ID)
	require.NoError(t, err)

	// Recover stale tasks (tasks running for more than 5 minutes)
	recovered, err := database.RecoverStaleTasks(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, recovered)

	// Tasks should be available again
	task3, err := database.GetNextTask(ctx, "worker-new")
	require.NoError(t, err)
	assert.NotNil(t, task3)

	// Clean up
	err = database.UpdateJobStatus(ctx, jobID, "cancelled")
	require.NoError(t, err)
}

// TestTransactionRollback tests transaction rollback on error
func TestTransactionRollback(t *testing.T) {
	testutil.LoadTestEnv(t)
	
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to database
	config := &db.Config{
		DatabaseURL: databaseURL,
	}
	database, err := db.New(config)
	require.NoError(t, err)
	defer database.Close()

	// Start a transaction
	tx, err := database.Begin(ctx)
	require.NoError(t, err)

	// Create a job in the transaction
	var jobID string
	err = tx.QueryRow(ctx, `
		INSERT INTO jobs (domain, status, concurrency, find_links, max_pages, created_at)
		VALUES ($1, 'pending', 1, false, 10, NOW())
		RETURNING id
	`, "tx-test.com").Scan(&jobID)
	require.NoError(t, err)

	// Rollback the transaction
	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// Job should not exist
	var count int
	err = database.QueryRow(ctx, "SELECT COUNT(*) FROM jobs WHERE id = $1", jobID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Job should not exist after rollback")
}

// TestConcurrentTaskProcessing tests concurrent task processing doesn't cause conflicts
func TestConcurrentTaskProcessing(t *testing.T) {
	testutil.LoadTestEnv(t)
	
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to database
	config := &db.Config{
		DatabaseURL: databaseURL,
	}
	database, err := db.New(config)
	require.NoError(t, err)
	defer database.Close()

	// Create a job with many tasks
	jobID, err := database.CreateJob(ctx, "concurrent-test.com", nil, nil, 10, false, 50, nil, nil)
	require.NoError(t, err)

	// Create 20 tasks
	taskCount := 20
	for i := 0; i < taskCount; i++ {
		err = database.CreateTask(ctx, jobID, "/page"+strconv.Itoa(i), "concurrent-test.com", "manual", "")
		require.NoError(t, err)
	}

	// Simulate multiple workers grabbing tasks concurrently
	workers := 5
	tasksChan := make(chan string, taskCount)
	errorsChan := make(chan error, workers)

	for w := 0; w < workers; w++ {
		go func(workerID int) {
			workerName := "worker-" + strconv.Itoa(workerID)
			for {
				task, err := database.GetNextTask(ctx, workerName)
				if err != nil {
					errorsChan <- err
					return
				}
				if task == nil {
					// No more tasks
					errorsChan <- nil
					return
				}
				tasksChan <- task.ID
				
				// Simulate processing
				time.Sleep(10 * time.Millisecond)
				
				// Complete the task
				err = database.UpdateTaskStatus(ctx, task.ID, "completed", nil, nil, nil, nil, nil, nil)
				if err != nil {
					errorsChan <- err
					return
				}
			}
		}(w)
	}

	// Wait for all workers
	for i := 0; i < workers; i++ {
		err := <-errorsChan
		assert.NoError(t, err)
	}
	close(tasksChan)

	// Count processed tasks
	processedTasks := make(map[string]bool)
	for taskID := range tasksChan {
		processedTasks[taskID] = true
	}

	// All tasks should have been processed exactly once
	assert.Equal(t, taskCount, len(processedTasks), "All tasks should be processed exactly once")

	// Verify in database
	var completedCount int
	err = database.QueryRow(ctx, "SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status = 'completed'", jobID).Scan(&completedCount)
	require.NoError(t, err)
	assert.Equal(t, taskCount, completedCount)

	// Clean up
	err = database.UpdateJobStatus(ctx, jobID, "completed")
	require.NoError(t, err)
}