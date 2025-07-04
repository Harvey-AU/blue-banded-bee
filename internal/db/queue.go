package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DbQueue is a PostgreSQL implementation of a job queue
type DbQueue struct {
	db *DB
}

// NewDbQueue creates a PostgreSQL job queue
func NewDbQueue(db *DB) *DbQueue {
	return &DbQueue{
		db: db,
	}
}

// Execute runs a database operation in a transaction
func (q *DbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	// Begin transaction
	tx, err := q.db.client.BeginTx(ctx, nil)
	if err != nil {
		sentry.CaptureException(err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Run the operation
	if err := fn(tx); err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		sentry.CaptureException(err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Task represents a task in the queue
type Task struct {
	ID          string
	JobID       string
	PageID      int
	Path        string
	Status      string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	RetryCount  int
	Error       string
	SourceType  string
	SourceURL   string

	// Result data
	StatusCode         int
	ResponseTime       int64
	CacheStatus        string
	ContentType        string
	SecondResponseTime int64
	SecondCacheStatus  string
	CacheCheckAttempts []byte // Stored as JSONB

	// Priority
	PriorityScore float64
}

// GetNextTask gets a pending task using row-level locking
func (q *DbQueue) GetNextTask(ctx context.Context, jobID string) (*Task, error) {
	var task Task

	err := q.Execute(ctx, func(tx *sql.Tx) error {
		// Query for a pending task with FOR UPDATE SKIP LOCKED
		// This allows concurrent workers to each get different tasks
		query := `
			SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score 
			FROM tasks 
			WHERE status = 'pending'
		`

		// Add job filter if specified
		args := []interface{}{}
		if jobID != "" {
			query += " AND job_id = $1"
			args = append(args, jobID)
		}

		// Add ordering and locking - prioritise by priority_score DESC, then created_at ASC
		query += `
			ORDER BY priority_score DESC, created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		`

		// Execute the query
		var row *sql.Row
		if len(args) > 0 {
			row = tx.QueryRowContext(ctx, query, args...)
		} else {
			row = tx.QueryRowContext(ctx, query)
		}

		err := row.Scan(
			&task.ID, &task.JobID, &task.PageID, &task.Path,
			&task.CreatedAt, &task.RetryCount, &task.SourceType, &task.SourceURL,
			&task.PriorityScore,
		)

		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		if err != nil {
			return fmt.Errorf("failed to query task: %w", err)
		}

		// Update the task status
		now := time.Now()
		_, err = tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'running', started_at = $1
			WHERE id = $2
		`, now, task.ID)

		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		task.Status = "running"
		task.StartedAt = now

		return nil
	})

	if err == sql.ErrNoRows {
		return nil, nil // No tasks available
	}
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// EnqueueURLs adds multiple URLs as tasks for a job
func (q *DbQueue) EnqueueURLs(ctx context.Context, jobID string, pages []Page, sourceType string, sourceURL string) error {
	if len(pages) == 0 {
		return nil
	}

	return q.Execute(ctx, func(tx *sql.Tx) error {
		// Get job's max_pages setting and current task counts
		var maxPages int
		var currentTaskCount int
		err := tx.QueryRowContext(ctx, `
			SELECT max_pages, 
				   COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status != 'skipped'), 0)
			FROM jobs WHERE id = $1
		`, jobID).Scan(&maxPages, &currentTaskCount)
		if err != nil {
			return fmt.Errorf("failed to get job max_pages and task count: %w", err)
		}

		// Count how many tasks will be pending vs skipped
		pendingCount := 0
		skippedCount := 0
		for _, page := range pages {
			if page.ID == 0 {
				continue
			}
			if maxPages == 0 || currentTaskCount+pendingCount < maxPages {
				pendingCount++
			} else {
				skippedCount++
			}
		}

		// Update job's total task count and skipped count
		_, err = tx.ExecContext(ctx, `
			UPDATE jobs
			SET total_tasks = total_tasks + $1,
				skipped_tasks = skipped_tasks + $2
			WHERE id = $3
		`, len(pages), skippedCount, jobID)
		if err != nil {
			return fmt.Errorf("failed to update job total tasks: %w", err)
		}

		// Prepare statement for batch insert
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at, retry_count,
				source_type, source_url, priority_score
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		// Insert each task with appropriate status
		now := time.Now()
		processedCount := 0
		for _, page := range pages {
			if page.ID == 0 {
				continue
			}

			// Determine status based on max_pages limit
			var status string
			if maxPages == 0 || currentTaskCount+processedCount < maxPages {
				status = "pending"
				processedCount++
			} else {
				status = "skipped"
			}

			taskID := uuid.New().String()
			_, err = stmt.ExecContext(ctx,
				taskID, jobID, page.ID, page.Path, status, now, 0, sourceType, sourceURL, page.Priority)

			if err != nil {
				return fmt.Errorf("failed to insert task: %w", err)
			}
		}

		return nil
	})
}

// CleanupStuckJobs finds and fixes jobs that are stuck in pending/running state
// despite having all their tasks completed
func (q *DbQueue) CleanupStuckJobs(ctx context.Context) error {
	span := sentry.StartSpan(ctx, "db.cleanup_stuck_jobs")
	defer span.Finish()

	// Define status constants for job states
	const (
		JobStatusCompleted = "completed"
		JobStatusPending   = "pending"
		JobStatusRunning   = "running"
	)

	result, err := q.db.client.ExecContext(ctx, `
		UPDATE jobs 
		SET status = $1, 
			completed_at = COALESCE(completed_at, $2),
			progress = 100.0
		WHERE (status = $3 OR status = $4)
		AND total_tasks > 0 
		AND total_tasks = completed_tasks + failed_tasks
	`, JobStatusCompleted, time.Now(), JobStatusPending, JobStatusRunning)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		sentry.CaptureException(err)
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

// UpdateTaskStatus updates a task's status and associated metadata in a single function
// This provides a unified way to handle various task state transitions
func (q *DbQueue) UpdateTaskStatus(ctx context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("cannot update nil task")
	}

	now := time.Now()

	// Set appropriate timestamps based on status if not already set
	if task.Status == "running" && task.StartedAt.IsZero() {
		task.StartedAt = now
	}
	if (task.Status == "completed" || task.Status == "failed") && task.CompletedAt.IsZero() {
		task.CompletedAt = now
	}

	// Update task in a transaction
	err := q.Execute(ctx, func(tx *sql.Tx) error {
		var err error

		// Use different update logic based on status
		switch task.Status {
		case "running":
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1, started_at = $2
				WHERE id = $3
			`, task.Status, task.StartedAt, task.ID)

		case "completed":
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1, completed_at = $2, status_code = $3, 
					response_time = $4, cache_status = $5, content_type = $6,
					second_response_time = $7, second_cache_status = $8,
					retry_count = $9, cache_check_attempts = $10
				WHERE id = $11
			`, task.Status, task.CompletedAt, task.StatusCode,
				task.ResponseTime, task.CacheStatus, task.ContentType,
				task.SecondResponseTime, task.SecondCacheStatus, task.RetryCount,
				task.CacheCheckAttempts, task.ID)

		case "failed":
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1, completed_at = $2, error = $3, retry_count = $4
				WHERE id = $5
			`, task.Status, task.CompletedAt, task.Error, task.RetryCount, task.ID)

		case "skipped":
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1
				WHERE id = $2
			`, task.Status, task.ID)

		default:
			// Generic status update
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1
				WHERE id = $2
			`, task.Status, task.ID)
		}

		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}
	
	return nil
}
