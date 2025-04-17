package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TaskQueue manages task processing using PostgreSQL's row-level locking
type TaskQueue struct {
	db *sql.DB
}

// NewTaskQueue creates a new PostgreSQL-based task queue
func NewTaskQueue(db *sql.DB) *TaskQueue {
	return &TaskQueue{db: db}
}

// Task represents a crawl task
type Task struct {
	ID          string
	JobID       string
	URL         string
	Status      string
	Depth       int
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	RetryCount  int
	Error       string
	SourceType  string
	SourceURL   string

	// Result data
	StatusCode   int
	ResponseTime int64
	CacheStatus  string
	ContentType  string
}

// GetNextTask acquires the next pending task using row-level locking
// This uses SELECT FOR UPDATE SKIP LOCKED for efficient concurrent processing
func (q *TaskQueue) GetNextTask(ctx context.Context) (*Task, error) {
	// Start transaction
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Try to get and lock an available task
	var task Task
	err = tx.QueryRowContext(ctx, `
		SELECT id, job_id, url, depth, created_at, retry_count, source_type, source_url 
		FROM tasks 
		WHERE status = 'pending'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`).Scan(
		&task.ID, &task.JobID, &task.URL, &task.Depth,
		&task.CreatedAt, &task.RetryCount, &task.SourceType, &task.SourceURL,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No tasks available
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query next task: %w", err)
	}

	// Mark task as running
	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = 'running', started_at = $1
		WHERE id = $2
	`, now, task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	task.Status = "running"
	task.StartedAt = now

	return &task, nil
}

// CompleteTask marks a task as completed
func (q *TaskQueue) CompleteTask(ctx context.Context, task *Task) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE tasks
		SET 
			status = 'completed', 
			completed_at = $1,
			status_code = $2,
			response_time = $3,
			cache_status = $4,
			content_type = $5,
			error = $6
		WHERE id = $7
	`, time.Now(), task.StatusCode, task.ResponseTime,
		task.CacheStatus, task.ContentType, task.Error, task.ID)

	if err != nil {
		return fmt.Errorf("failed to mark task as completed: %w", err)
	}

	// Update job progress
	return q.updateJobProgress(ctx, task.JobID)
}

// FailTask marks a task as failed
func (q *TaskQueue) FailTask(ctx context.Context, task *Task, err error) error {
	_, dbErr := q.db.ExecContext(ctx, `
		UPDATE tasks
		SET 
			status = 'failed', 
			completed_at = $1,
			error = $2
		WHERE id = $3
	`, time.Now(), err.Error(), task.ID)

	if dbErr != nil {
		return fmt.Errorf("failed to mark task as failed: %w", dbErr)
	}

	// Update job progress
	return q.updateJobProgress(ctx, task.JobID)
}

// updateJobProgress updates a job's progress based on task completion
func (q *TaskQueue) updateJobProgress(ctx context.Context, jobID string) error {
	// Start a transaction
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get task counts
	var total, completed, failed int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM tasks 
		WHERE job_id = $1
	`, jobID).Scan(&total)
	if err != nil {
		return fmt.Errorf("failed to get total tasks: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM tasks 
		WHERE job_id = $1 AND status = 'completed'
	`, jobID).Scan(&completed)
	if err != nil {
		return fmt.Errorf("failed to get completed tasks: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM tasks 
		WHERE job_id = $1 AND status = 'failed'
	`, jobID).Scan(&failed)
	if err != nil {
		return fmt.Errorf("failed to get failed tasks: %w", err)
	}

	// Calculate progress
	progress := 0.0
	if total > 0 {
		progress = float64(completed+failed) / float64(total) * 100.0
	}

	// Update job
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs
		SET 
			progress = $1,
			completed_tasks = $2,
			failed_tasks = $3,
			status = CASE 
				WHEN $1 >= 100.0 THEN 'completed'
				ELSE status
			END,
			completed_at = CASE 
				WHEN $1 >= 100.0 THEN NOW()
				ELSE completed_at
			END
		WHERE id = $4
	`, progress, completed, failed, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	return tx.Commit()
}

// EnqueueTasks adds multiple tasks for a job
func (q *TaskQueue) EnqueueTasks(ctx context.Context, jobID string, urls []string, sourceType string, sourceURL string, depth int) error {
	if len(urls) == 0 {
		return nil
	}

	// Use a transaction for the entire batch
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// First update the total task count
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs
		SET total_tasks = total_tasks + $1
		WHERE id = $2
	`, len(urls), jobID)
	if err != nil {
		return fmt.Errorf("failed to update job total tasks: %w", err)
	}

	// Prepare batch insert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tasks (
			id, job_id, url, status, depth, created_at, retry_count,
			source_type, source_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert all tasks
	now := time.Now()
	for _, url := range urls {
		taskID := uuid.New().String()
		_, err = stmt.ExecContext(ctx,
			taskID, jobID, url, "pending", depth, now, 0, sourceType, sourceURL)
		if err != nil {
			return fmt.Errorf("failed to insert task: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("job_id", jobID).
		Int("task_count", len(urls)).
		Msg("Successfully enqueued tasks")

	return nil
}
