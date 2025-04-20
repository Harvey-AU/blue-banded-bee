package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Initialize random seed
func init() {
	// As of Go 1.20, rand.Seed is deprecated and no longer needed
	// The global random number generator is now automatically seeded
	// For specific seeded sequences, use rand.New(rand.NewSource(seed))
}

// Initialize database schema for jobs and tasks
func InitSchema(db *sql.DB) error {
	// Create jobs table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			domain TEXT NOT NULL,
			status TEXT NOT NULL,
			progress REAL NOT NULL,
			total_tasks INTEGER NOT NULL,
			completed_tasks INTEGER NOT NULL,
			failed_tasks INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			concurrency INTEGER NOT NULL,
			find_links BOOLEAN NOT NULL,
			include_paths TEXT,
			exclude_paths TEXT,
			error_message TEXT,
			required_workers INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	// Create tasks table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			page_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			depth INTEGER NOT NULL,
			path TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			retry_count INTEGER NOT NULL,
			error TEXT,
			status_code INTEGER,
			response_time INTEGER,
			cache_status TEXT,
			content_type TEXT,
			source_type TEXT NOT NULL,
			source_url TEXT,
			FOREIGN KEY (job_id) REFERENCES jobs(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// Create indexes
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job_id ON tasks(job_id)`)
	if err != nil {
		return fmt.Errorf("failed to create task index: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`)
	if err != nil {
		return fmt.Errorf("failed to create task status index: %w", err)
	}

	return nil
}

// serialize helper function
func serialize(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize data")
		return "{}"
	}
	return string(data)
}

/**
 * Database Retry Logic
 *
 * SQLite databases can experience "database is locked" errors during concurrent access,
 * especially when multiple processes/threads try to write simultaneously. This is particularly
 * common in high-concurrency scenarios like our job queue system.
 *
 * The retryDB function implements exponential backoff with jitter to gracefully handle
 * these transient errors:
 *
 * 1. It attempts the database operation
 * 2. If the operation fails with a "database is locked" or "busy" error, it waits
 *    and retries with increasing backoff periods
 * 3. A small random jitter is added to prevent retry storms
 * 4. It gives up after a maximum number of retries
 *
 * This mechanism significantly improves reliability when multiple workers are
 * simultaneously accessing the database.
 */

// retryDB executes a database operation with exponential backoff retry
func retryDB(operation func() error) error {
	var lastErr error
	retries := 5
	backoff := 200 * time.Millisecond

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff * time.Duration(1<<uint(attempt-1)))
		}
		err := operation()
		if err == nil {
			return nil
		}

		// Check if error is a database lock error
		if strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "busy") {
			// Calculate backoff with jitter
			backoff := backoff * time.Duration(1<<uint(attempt))
			jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
			sleepTime := backoff + jitter

			log.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Dur("backoff", sleepTime).
				Msg("Database locked, retrying operation")

			time.Sleep(sleepTime)
			continue
		}

		// Not a retryable error
		lastErr = err
	}

	return lastErr
}

// CreateJob inserts a new job into the database
func CreateJob(db *sql.DB, options *JobOptions) (*Job, error) {
	job := &Job{
		ID:              uuid.New().String(),
		Domain:          options.Domain,
		Status:          JobStatusPending,
		Progress:        0,
		TotalTasks:      0,
		CompletedTasks:  0,
		FailedTasks:     0,
		CreatedAt:       time.Now(),
		Concurrency:     options.Concurrency,
		FindLinks:       options.FindLinks,
		MaxDepth:        options.MaxDepth,
		IncludePaths:    options.IncludePaths,
		ExcludePaths:    options.ExcludePaths,
		RequiredWorkers: options.RequiredWorkers,
	}

	err := retryDB(func() error {
		_, err := db.Exec(
			`INSERT INTO jobs (
				id, domain, status, progress, total_tasks, completed_tasks, failed_tasks,
				created_at, concurrency, find_links, include_paths, exclude_paths,
				required_workers
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			job.ID, job.Domain, string(job.Status), job.Progress,
			job.TotalTasks, job.CompletedTasks, job.FailedTasks,
			job.CreatedAt, job.Concurrency, job.FindLinks,
			serialize(job.IncludePaths), serialize(job.ExcludePaths),
			job.RequiredWorkers,
		)
		return err
	})

	return job, err
}

// CreateTask inserts a new task into the database
func CreateTask(ctx context.Context, db *sql.DB, task *Task) error {
	span := sentry.StartSpan(ctx, "jobs.create_task")
	defer span.Finish()

	span.SetTag("job_id", task.JobID)
	span.SetTag("page_id", fmt.Sprintf("%d", task.PageID))
	span.SetTag("path", task.Path)

	err := retryDB(func() error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO tasks (
				id, job_id, page_id, status, depth, path, created_at, started_at, completed_at,
				retry_count, error, source_type, source_url
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`,
			task.ID, task.JobID, task.PageID, task.Status, task.Depth, task.Path, task.CreatedAt,
			task.StartedAt, task.CompletedAt, task.RetryCount, task.Error, task.SourceType, task.SourceURL,
		)
		return err
	})

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return fmt.Errorf("failed to insert task: %w", err)
	}

	return nil
}

// GetJob retrieves a job by ID
func GetJob(ctx context.Context, db *sql.DB, jobID string) (*Job, error) {
	span := sentry.StartSpan(ctx, "jobs.get_job")
	defer span.Finish()

	span.SetTag("job_id", jobID)

	var job Job
	var includePaths, excludePaths []byte
	var startedAt, completedAt sql.NullTime
	var errorMessage sql.NullString

	err := retryDB(func() error {
		err := db.QueryRowContext(ctx, `
			SELECT 
				id, domain, status, progress, total_tasks, completed_tasks, failed_tasks,
				created_at, started_at, completed_at, concurrency, find_links,
				include_paths, exclude_paths, error_message, required_workers
			FROM jobs
			WHERE id = $1
		`, jobID).Scan(
			&job.ID, &job.Domain, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.CreatedAt, &startedAt, &completedAt, &job.Concurrency,
			&job.FindLinks, &includePaths, &excludePaths, &errorMessage, &job.RequiredWorkers,
		)
		return err
	})

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", jobID)
	} else if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Handle nullable fields
	if startedAt.Valid {
		job.StartedAt = startedAt.Time
	}

	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}

	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}

	// Parse arrays from JSON
	if len(includePaths) > 0 {
		err = json.Unmarshal(includePaths, &job.IncludePaths)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal include paths: %w", err)
		}
	}

	if len(excludePaths) > 0 {
		err = json.Unmarshal(excludePaths, &job.ExcludePaths)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal exclude paths: %w", err)
		}
	}

	return &job, nil
}

// NOTE: GetNextPendingTask function moved to worker.go to avoid duplication

// UpdateTaskStatus updates a task's status and result data
func UpdateTaskStatus(ctx context.Context, db *sql.DB, task *Task) error {
	// Only create spans for errors
	var span *sentry.Span
	defer func() {
		if span != nil {
			span.Finish()
		}
	}()

	now := time.Now()
	err := retryDB(func() error {
		var err error

		// Use constants for all status comparisons for consistency
		if task.Status == TaskStatusRunning {
			task.StartedAt = now
			_, err = db.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1, started_at = $2
				WHERE id = $3
			`, string(task.Status), task.StartedAt, task.ID)

		} else if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			task.CompletedAt = now
			_, err = db.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1, completed_at = $2, 
					error = $3, retry_count = $4
				WHERE id = $5
			`,
				string(task.Status), task.CompletedAt,
				task.Error, task.RetryCount, task.ID)

		} else if task.Status == TaskStatusSkipped {
			// Add explicit handling for skipped tasks
			_, err = db.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1
				WHERE id = $2
			`, string(task.Status), task.ID)
		} else {
			// Generic update for any other status
			_, err = db.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1
				WHERE id = $2
			`, string(task.Status), task.ID)
		}

		return err
	})

	if err != nil {
		span = sentry.StartSpan(ctx, "jobs.update_task_status.error")
		span.SetTag("task_id", task.ID)
		span.SetTag("status", string(task.Status))
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
	}
	return err
}

// ListJobs retrieves a list of jobs with pagination
func ListJobs(ctx context.Context, db *sql.DB, limit, offset int) ([]*Job, error) {
	span := sentry.StartSpan(ctx, "jobs.list_jobs")
	defer span.Finish()

	span.SetData("limit", limit)
	span.SetData("offset", offset)

	rows, err := db.QueryContext(ctx, `
		SELECT 
			id, domain, status, progress, total_tasks, completed_tasks, failed_tasks,
			created_at, started_at, completed_at, concurrency, find_links,
			include_paths, exclude_paths, error_message
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var job Job
		var includePaths, excludePaths []byte
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID, &job.Domain, &job.Status, &job.Progress, &job.TotalTasks, &job.CompletedTasks,
			&job.FailedTasks, &job.CreatedAt, &startedAt, &completedAt, &job.Concurrency,
			&job.FindLinks, &includePaths, &excludePaths, &job.ErrorMessage,
		)

		if err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Parse arrays from JSON
		if len(includePaths) > 0 {
			err = json.Unmarshal(includePaths, &job.IncludePaths)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal include paths: %w", err)
			}
		}

		if len(excludePaths) > 0 {
			err = json.Unmarshal(excludePaths, &job.ExcludePaths)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal exclude paths: %w", err)
			}
		}

		// Handle nullable times
		if startedAt.Valid {
			job.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = completedAt.Time
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return nil, fmt.Errorf("error iterating jobs: %w", err)
	}

	return jobs, nil
}
