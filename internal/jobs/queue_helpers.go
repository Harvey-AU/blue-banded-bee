package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/common"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog/log"
)

// Use interface instead of concrete type to avoid import cycle
type queueProvider interface {
	GetQueue() *common.DbQueue
}

// dbInstance needs to be set at application startup
var dbInstance queueProvider

// SetDBInstance sets the global DB instance
func SetDBInstance(instance queueProvider) {
	dbInstance = instance
}

// ExecuteInQueue runs a database operation in the global queue
func ExecuteInQueue(ctx context.Context, fn func(*sql.Tx) error) error {
	if dbInstance == nil {
		return fmt.Errorf("database instance not initialized")
	}

	// Add retry logic specifically for connection issues
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := dbInstance.GetQueue().Execute(ctx, fn)
		if err == nil {
			return nil
		}

		// Check for connection errors
		if strings.Contains(err.Error(), "stream is closed") ||
			strings.Contains(err.Error(), "driver: bad connection") {
			if attempt < maxRetries-1 {
				log.Warn().
					Err(err).
					Int("attempt", attempt+1).
					Msg("Database connection error, retrying...")
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
		}
		return err
	}
	return fmt.Errorf("max retries exceeded")
}

// Only implement transaction versions of critical functions

// UpdateTaskStatusTx updates a task's status within a transaction
func UpdateTaskStatusTx(ctx context.Context, tx *sql.Tx, task *Task) error {
	span := sentry.StartSpan(ctx, "jobs.update_task_status_tx")
	defer span.Finish()

	span.SetTag("task_id", task.ID)
	span.SetTag("task_status", string(task.Status))

	now := time.Now()

	// Set timestamps based on status
	if task.Status == TaskStatusRunning && task.StartedAt.IsZero() {
		task.StartedAt = now
	}
	if (task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed) && task.CompletedAt.IsZero() {
		task.CompletedAt = now
	}

	// Update task status
	_, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = ?,
			started_at = ?,
			completed_at = ?,
			retry_count = ?,
			error = ?
		WHERE id = ?
	`, task.Status, task.StartedAt, task.CompletedAt,
		task.RetryCount, task.Error, task.ID)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// GetNextPendingTaskTx gets and claims the next pending task for a job
func GetNextPendingTaskTx(ctx context.Context, tx *sql.Tx, jobID string) (*Task, error) {
	span := sentry.StartSpan(ctx, "jobs.get_next_pending_task_tx")

	defer span.Finish()

	span.SetTag("job_id", jobID)

	now := time.Now()

	// Get task ID to update first using a separate transaction-safe query
	var taskID string
	row := tx.QueryRowContext(ctx, `
		SELECT id FROM tasks 
		WHERE job_id = ? AND status = ? 
		LIMIT 1
	`, jobID, TaskStatusPending)

	err := row.Scan(&taskID)
	if err == sql.ErrNoRows {
		return nil, nil // No pending tasks
	}
	if err != nil {
		return nil, err
	}

	// Now update and get the specific task we identified
	_, err = tx.ExecContext(ctx, `
		UPDATE tasks 
		SET status = ?, started_at = ?
		WHERE id = ?
	`, TaskStatusRunning, now, taskID)

	if err != nil {
		return nil, err
	}

	// Get the complete task details
	row = tx.QueryRowContext(ctx, `
		SELECT 
			t.id, t.job_id, t.page_id, p.path, t.status, t.depth, t.created_at, t.started_at, t.completed_at,
			retry_count, error, source_type, source_url
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		WHERE t.id = ?
	`, taskID)

	task := &Task{}
	var startedAt, completedAt sql.NullTime
	var errorMsg, sourceURL sql.NullString

	err = row.Scan(
		&task.ID, &task.JobID, &task.PageID, &task.Path, &task.Status, &task.Depth, &task.CreatedAt,
		&startedAt, &completedAt, &task.RetryCount, &errorMsg,
		&task.SourceType, &sourceURL,
	)

	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.Time
	}

	if errorMsg.Valid {
		task.Error = errorMsg.String
	}
	if sourceURL.Valid {
		task.SourceURL = sourceURL.String
	}

	return task, nil
}

// batchInsertCrawlResults inserts multiple crawl results in a single transaction
func batchInsertCrawlResults(ctx context.Context, tx *sql.Tx, results []CrawlResultData) error {
	if len(results) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(results))
	valueArgs := make([]interface{}, 0, len(results)*8)

	for _, result := range results {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs,
			result.JobID, result.TaskID, result.URL, result.ResponseTime,
			result.StatusCode, result.Error, result.CacheStatus,
			result.ContentType)
	}

	query := fmt.Sprintf(`
		INSERT INTO crawl_results 
		(job_id, task_id, url, response_time, status_code, error, cache_status, content_type)
		VALUES %s
	`, strings.Join(valueStrings, ","))

	startTime := time.Now()
	result, err := tx.ExecContext(ctx, query, valueArgs...)
	duration := time.Since(startTime)

	log.Debug().
		Int("count", len(results)).
		Dur("duration_ms", duration).
		Msg("Batch inserted crawl results")

	if err != nil {
		log.Error().
			Err(err).
			Int("task_count", len(results)).
			Msg("Failed to batch insert crawl results")
		return fmt.Errorf("failed to batch insert crawl results: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Int64("rows_affected", rowsAffected).
		Msg("Job update result")

	return nil
}

// filterTasksByStatus returns tasks with a specific status
func filterTasksByStatus(tasks []*Task, status TaskStatus) []*Task {
	if len(tasks) == 0 {
		return nil
	}

	result := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Status == status {
			result = append(result, task)
		}
	}

	return result
}

// CleanupStuckJobs finds and fixes jobs that are stuck in pending/running state
// despite having all their tasks completed
func CleanupStuckJobs(ctx context.Context, db *sql.DB) error {
	span := sentry.StartSpan(ctx, "jobs.cleanup_stuck_jobs")
	defer span.Finish()

	result, err := db.ExecContext(ctx, `
		UPDATE jobs 
		SET status = ?, 
			completed_at = COALESCE(completed_at, ?),
			progress = 100.0
		WHERE (status = ? OR status = ?)
		AND total_tasks > 0 
		AND total_tasks = completed_tasks + failed_tasks
	`, JobStatusCompleted, time.Now(), JobStatusPending, JobStatusRunning)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to cleanup stuck jobs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Info().
			Int64("jobs_fixed", rowsAffected).
			Msg("Fixed stuck jobs")
	}

	return nil
}
