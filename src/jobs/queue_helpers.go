package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/common"
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

// updateJobProgressTx updates a job's progress within a transaction
func updateJobProgressTx(ctx context.Context, tx *sql.Tx, jobID string) error {
	// Get current task counts
	var total, completed, failed int
	var recentURLs []string

	// Count tasks by status
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = ?
	`).Scan(&total)
	if err != nil {
		return err
	}

	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status = ?
	`, jobID, TaskStatusCompleted).Scan(&completed)
	if err != nil {
		return err
	}

	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status = ?
	`, jobID, TaskStatusFailed).Scan(&failed)
	if err != nil {
		return err
	}

	// Get recent URLs (last 5 completed tasks)
	rows, err := tx.QueryContext(ctx, `
		SELECT url FROM tasks 
		WHERE job_id = ? AND (status = ? OR status = ?)
		ORDER BY completed_at DESC LIMIT 5
	`, jobID, TaskStatusCompleted, TaskStatusFailed)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return err
		}
		recentURLs = append(recentURLs, url)
	}

	// Calculate progress (avoid division by zero)
	var progress float64
	if total > 0 {
		progress = float64(completed+failed) / float64(total) * 100
	}

	log.Debug().
		Str("job_id", jobID).
		Int("total", total).
		Int("completed", completed).
		Int("failed", failed).
		Msg("Job progress stats calculated")

	log.Debug().
		Str("job_id", jobID).
		Msg("Prepared job update data")

	// Update job status
	result, err := tx.ExecContext(ctx, `
		UPDATE jobs SET
			progress = ?,
			total_tasks = ?,
			completed_tasks = ?,
			failed_tasks = ?
		WHERE id = ?
	`, progress, total, completed, failed, jobID)

	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job")
		return err
	}

	// Check rows affected
	rowsAffected, _ := result.RowsAffected()
	log.Debug().
		Str("job_id", jobID).
		Int64("rows_affected", rowsAffected).
		Msg("Job update result")

	// If all tasks are done, update job status to completed
	if completed+failed == total && total > 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE jobs SET
				status = ?,
				completed_at = ?
			WHERE id = ? AND status = ?
		`, JobStatusCompleted, time.Now(), jobID, JobStatusRunning)
		if err != nil {
			return err
		}
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
			id, job_id, url, status, depth, created_at, started_at, completed_at,
			retry_count, error, source_type, source_url
		FROM tasks 
		WHERE id = ?
	`, taskID)

	task := &Task{}
	var startedAt, completedAt sql.NullTime
	var errorMsg, sourceURL sql.NullString

	err = row.Scan(
		&task.ID, &task.JobID, &task.URL, &task.Status, &task.Depth, &task.CreatedAt,
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

// updateJobCounter updates job counters during batch processing
func updateJobCounter(ctx context.Context, tx *sql.Tx, jobID string, completedCount, failedCount int) error {
	span := sentry.StartSpan(ctx, "jobs.update_job_counter")
	defer span.Finish()

	span.SetTag("job_id", jobID)
	span.SetData("completed_count", completedCount)
	span.SetData("failed_count", failedCount)

	if completedCount == 0 && failedCount == 0 {
		return nil
	}

	startTime := time.Now()

	// More efficient update query that avoids subqueries
	result, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET 
			completed_tasks = completed_tasks + ?,
			failed_tasks = failed_tasks + ?,
			progress = CAST(100.0 * (completed_tasks + ? + failed_tasks + ?) / 
					   CASE WHEN total_tasks = 0 THEN 1 ELSE total_tasks END AS FLOAT),
			status = CASE 
				WHEN (completed_tasks + ? + failed_tasks + ?) >= total_tasks AND total_tasks > 0 
				THEN ? ELSE status END,
			completed_at = CASE 
				WHEN (completed_tasks + ? + failed_tasks + ?) >= total_tasks AND total_tasks > 0 
				THEN ? ELSE completed_at END
		WHERE id = ?
	`,
		completedCount, failedCount,
		completedCount, failedCount,
		completedCount, failedCount, string(JobStatusCompleted),
		completedCount, failedCount, time.Now(),
		jobID)

	duration := time.Since(startTime)

	if err != nil {
		log.Error().
			Err(err).
			Str("job_id", jobID).
			Int("completed", completedCount).
			Int("failed", failedCount).
			Msg("Failed to update job counters")
		return fmt.Errorf("failed to update job counters: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	log.Debug().
		Str("job_id", jobID).
		Int("completed", completedCount).
		Int("failed", failedCount).
		Int64("rows_affected", rowsAffected).
		Dur("duration_ms", duration).
		Msg("Updated job counters")

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
