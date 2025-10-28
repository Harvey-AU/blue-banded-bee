package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DbQueue is a PostgreSQL implementation of a job queue
type DbQueue struct {
	db *DB
	// Mutex to prevent concurrent cleanup operations that cause prepared statement conflicts
	cleanupMutex        sync.Mutex
	poolWarnThreshold   float64
	poolRejectThreshold float64
	lastWarnLog         time.Time
	lastRejectLog       time.Time
}

// ErrPoolSaturated is returned when the database connection pool is saturated
var ErrPoolSaturated = errors.New("database connection pool saturated")

const (
	defaultPoolWarnThreshold   = 0.90
	defaultPoolRejectThreshold = 0.95
	poolLogCooldown            = 5 * time.Second
)

// NewDbQueue creates a PostgreSQL job queue
func NewDbQueue(db *DB) *DbQueue {
	warn := parseThresholdEnv("DB_POOL_WARN_THRESHOLD", defaultPoolWarnThreshold)
	reject := parseThresholdEnv("DB_POOL_REJECT_THRESHOLD", defaultPoolRejectThreshold)

	// Ensure thresholds are sane and warn <= reject
	if reject <= 0 || reject > 1 {
		reject = defaultPoolRejectThreshold
	}
	if warn <= 0 || warn >= reject {
		warn = reject - 0.05
		if warn <= 0 {
			warn = defaultPoolWarnThreshold
		}
	}

	return &DbQueue{
		db:                  db,
		poolWarnThreshold:   warn,
		poolRejectThreshold: reject,
	}
}

func parseThresholdEnv(key string, fallback float64) float64 {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// Execute runs a database operation in a transaction
func (q *DbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	// Add timeout to context if none exists
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	if err := q.ensurePoolCapacity(); err != nil {
		return err
	}

	// Begin transaction
	tx, err := q.db.client.BeginTx(ctx, nil)
	if err != nil {
		sentry.CaptureException(err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback is safe to call even after commit
	}()

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

// ExecuteMaintenance runs a low-impact transaction that bypasses pool saturation guards.
// This is intended for housekeeping tasks that must always run, even when the pool is busy.
func (q *DbQueue) ExecuteMaintenance(ctx context.Context, fn func(*sql.Tx) error) error {
	if q == nil || q.db == nil || q.db.client == nil {
		return fmt.Errorf("maintenance transaction requires an initialised database connection")
	}

	// Keep maintenance units short-lived to minimise pool impact.
	// Allow 65s to accommodate recovery batches processing large backlogs.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 65*time.Second)
		defer cancel()
	}

	tx, err := q.db.client.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		sentry.CaptureException(err)
		return fmt.Errorf("failed to begin maintenance transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Apply a statement timeout so maintenance never blocks the pool indefinitely.
	// Set to 60s to allow recovery batches time to process large backlogs.
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '60s'`); err != nil {
		log.Warn().Err(err).Msg("Failed to set maintenance statement timeout")
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		sentry.CaptureException(err)
		return fmt.Errorf("failed to commit maintenance transaction: %w", err)
	}

	return nil
}

func (q *DbQueue) ensurePoolCapacity() error {
	if q == nil || q.db == nil || q.db.client == nil {
		return nil
	}

	stats := q.db.client.Stats()
	maxOpen := stats.MaxOpenConnections
	if maxOpen == 0 && q.db.config != nil {
		maxOpen = q.db.config.MaxOpenConns
	}
	if maxOpen <= 0 {
		return nil
	}

	usage := float64(stats.InUse) / float64(maxOpen)

	if usage >= q.poolRejectThreshold {
		if time.Since(q.lastRejectLog) > poolLogCooldown {
			log.Warn().
				Int("in_use", stats.InUse).
				Int("max_open", maxOpen).
				Float64("usage", usage).
				Msg("DB pool saturated: rejecting request")
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(sentry.LevelWarning)
				scope.SetTag("event_type", "db_pool")
				scope.SetTag("state", "reject")
				scope.SetContext("db_pool", map[string]interface{}{
					"in_use":     stats.InUse,
					"max_open":   maxOpen,
					"idle":       stats.Idle,
					"wait_count": stats.WaitCount,
					"usage":      usage,
				})
				sentry.CaptureMessage("DB pool saturated")
			})
			q.lastRejectLog = time.Now()
		}
		return ErrPoolSaturated
	}

	if usage >= q.poolWarnThreshold && time.Since(q.lastWarnLog) > poolLogCooldown {
		log.Warn().
			Int("in_use", stats.InUse).
			Int("max_open", maxOpen).
			Float64("usage", usage).
			Msg("DB pool nearing capacity")
		// Note: Not sending to Sentry to avoid noise - only capture actual rejections
		q.lastWarnLog = time.Now()
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
	StatusCode          int
	ResponseTime        int64
	CacheStatus         string
	ContentType         string
	ContentLength       int64
	Headers             []byte // Stored as JSONB
	RedirectURL         string
	DNSLookupTime       int64
	TCPConnectionTime   int64
	TLSHandshakeTime    int64
	TTFB                int64
	ContentTransferTime int64

	// Second request data
	SecondResponseTime        int64
	SecondCacheStatus         string
	SecondContentLength       int64
	SecondHeaders             []byte // Stored as JSONB
	SecondDNSLookupTime       int64
	SecondTCPConnectionTime   int64
	SecondTLSHandshakeTime    int64
	SecondTTFB                int64
	SecondContentTransferTime int64
	CacheCheckAttempts        []byte // Stored as JSONB

	// Priority
	PriorityScore float64
}

// GetNextTask gets a pending task using row-level locking
// Uses FOR UPDATE SKIP LOCKED to prevent lock contention between workers
// Combines SELECT and UPDATE in a CTE for atomic claiming
func (q *DbQueue) GetNextTask(ctx context.Context, jobID string) (*Task, error) {
	var task Task
	now := time.Now().UTC()

	err := q.Execute(ctx, func(tx *sql.Tx) error {
		// Use CTE to select and update in a single atomic query
		// This reduces transaction time and minimises lock holding
		// Also enforces per-job concurrency limits by checking running_tasks < concurrency
		// Only locks the specific job row for the task we claim (not all eligible jobs)
		query := `
			WITH next_task AS (
				-- Claim a task and check job concurrency in one step
				SELECT t.id, t.job_id, t.page_id, t.path, t.created_at, t.retry_count,
				       t.source_type, t.source_url, t.priority_score
				FROM tasks t
				INNER JOIN jobs j ON t.job_id = j.id
				WHERE t.status = 'pending'
				AND j.status = 'running'
				-- Support legacy jobs with NULL or 0 concurrency (unlimited)
				AND (j.concurrency IS NULL OR j.concurrency = 0 OR j.running_tasks < j.concurrency)
		`

		// Add job filter if specified
		args := []interface{}{now}
		if jobID != "" {
			query += " AND t.job_id = $2"
			args = append(args, jobID)
		}

		query += `
				ORDER BY t.priority_score DESC, t.created_at ASC
				LIMIT 1
				-- Lock both the task and its job row (only for this specific task)
				FOR UPDATE OF t, j SKIP LOCKED
			),
			task_update AS (
				UPDATE tasks
				SET status = 'running', started_at = $1
				FROM next_task
				WHERE tasks.id = next_task.id
				RETURNING tasks.id, tasks.job_id, tasks.page_id, tasks.path,
				          tasks.created_at, tasks.retry_count, tasks.source_type,
				          tasks.source_url, tasks.priority_score
			)
			-- Atomically increment running_tasks for the job
			UPDATE jobs
			SET running_tasks = running_tasks + 1
			FROM task_update
			WHERE jobs.id = task_update.job_id
			RETURNING task_update.id, task_update.job_id, task_update.page_id, task_update.path,
			          task_update.created_at, task_update.retry_count, task_update.source_type,
			          task_update.source_url, task_update.priority_score
		`

		// Execute the combined query
		row := tx.QueryRowContext(ctx, query, args...)

		err := row.Scan(
			&task.ID, &task.JobID, &task.PageID, &task.Path,
			&task.CreatedAt, &task.RetryCount, &task.SourceType, &task.SourceURL,
			&task.PriorityScore,
		)

		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		if err != nil {
			return fmt.Errorf("failed to claim task: %w", err)
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
		uniquePages := make([]Page, 0, len(pages))
		seen := make(map[int]int, len(pages))
		for _, page := range pages {
			if page.ID == 0 {
				continue
			}

			if idx, ok := seen[page.ID]; ok {
				if page.Priority > uniquePages[idx].Priority {
					uniquePages[idx].Priority = page.Priority
				}
				continue
			}

			seen[page.ID] = len(uniquePages)
			uniquePages = append(uniquePages, page)
		}

		if len(uniquePages) == 0 {
			return nil
		}

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
		for range uniquePages {
			if maxPages == 0 || currentTaskCount+pendingCount < maxPages {
				pendingCount++
			} else {
				skippedCount++
			}
		}

		// Use direct query instead of prepared statement for Supabase pooler compatibility
		insertQuery := `
			INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at, retry_count,
				source_type, source_url, priority_score
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (job_id, page_id) DO NOTHING
		`

		// Insert each task with appropriate status
		now := time.Now().UTC()
		processedCount := 0
		for _, page := range uniquePages {
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
			_, err = tx.ExecContext(ctx, insertQuery,
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
	// Serialize cleanup operations to prevent prepared statement conflicts
	q.cleanupMutex.Lock()
	defer q.cleanupMutex.Unlock()

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
	`, JobStatusCompleted, time.Now().UTC(), JobStatusPending, JobStatusRunning)

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

	now := time.Now().UTC()

	// Set appropriate timestamps based on status if not already set
	if task.Status == "running" && task.StartedAt.IsZero() {
		task.StartedAt = now
	}
	if (task.Status == "completed" || task.Status == "failed") && task.CompletedAt.IsZero() {
		task.CompletedAt = now
	}

	// Update task in a transaction
	// Also adjust running_tasks counter for the job when status changes
	err := q.Execute(ctx, func(tx *sql.Tx) error {
		var err error
		var jobID string

		// Use different update logic based on status
		switch task.Status {
		case "running":
			// Increment running_tasks when manually setting to running (rare, but handle it)
			err = tx.QueryRowContext(ctx, `
				WITH task_update AS (
					UPDATE tasks
					SET status = $1, started_at = $2
					WHERE id = $3
					RETURNING job_id
				)
				UPDATE jobs
				SET running_tasks = running_tasks + 1
				FROM task_update
				WHERE jobs.id = task_update.job_id
				RETURNING task_update.job_id
			`, task.Status, task.StartedAt, task.ID).Scan(&jobID)

		case "completed":
			// Ensure JSONB fields are never nil and are valid JSON
			headers := task.Headers
			if len(headers) == 0 {
				headers = []byte("{}")
			}
			secondHeaders := task.SecondHeaders
			if len(secondHeaders) == 0 {
				secondHeaders = []byte("{}")
			}
			cacheCheckAttempts := task.CacheCheckAttempts
			if len(cacheCheckAttempts) == 0 {
				cacheCheckAttempts = []byte("[]")
			}

			// Log the actual values being passed for debugging
			log.Debug().
				Str("task_id", task.ID).
				Str("headers", string(headers)).
				Str("second_headers", string(secondHeaders)).
				Str("cache_check_attempts", string(cacheCheckAttempts)).
				Msg("Updating task with JSONB fields")

			// Update task fields only (running_tasks decremented separately via DecrementRunningTasks)
			err = tx.QueryRowContext(ctx, `
				UPDATE tasks
				SET status = $1, completed_at = $2, status_code = $3,
					response_time = $4, cache_status = $5, content_type = $6,
					content_length = $7, headers = $8::jsonb, redirect_url = $9,
					dns_lookup_time = $10, tcp_connection_time = $11, tls_handshake_time = $12,
					ttfb = $13, content_transfer_time = $14,
					second_response_time = $15, second_cache_status = $16,
					second_content_length = $17, second_headers = $18::jsonb,
					second_dns_lookup_time = $19, second_tcp_connection_time = $20,
					second_tls_handshake_time = $21, second_ttfb = $22,
					second_content_transfer_time = $23,
					retry_count = $24, cache_check_attempts = $25::jsonb
				WHERE id = $26
				RETURNING job_id
			`, task.Status, task.CompletedAt, task.StatusCode,
				task.ResponseTime, task.CacheStatus, task.ContentType,
				task.ContentLength, string(headers), task.RedirectURL,
				task.DNSLookupTime, task.TCPConnectionTime, task.TLSHandshakeTime,
				task.TTFB, task.ContentTransferTime,
				task.SecondResponseTime, task.SecondCacheStatus,
				task.SecondContentLength, string(secondHeaders),
				task.SecondDNSLookupTime, task.SecondTCPConnectionTime,
				task.SecondTLSHandshakeTime, task.SecondTTFB,
				task.SecondContentTransferTime,
				task.RetryCount, string(cacheCheckAttempts), task.ID).Scan(&jobID)

		case "failed":
			// Update task fields only (running_tasks decremented separately via DecrementRunningTasks)
			err = tx.QueryRowContext(ctx, `
				UPDATE tasks
				SET status = $1, completed_at = $2, error = $3, retry_count = $4
				WHERE id = $5
				RETURNING job_id
			`, task.Status, task.CompletedAt, task.Error, task.RetryCount, task.ID).Scan(&jobID)

		case "skipped":
			// Update task fields only (running_tasks decremented separately via DecrementRunningTasks)
			err = tx.QueryRowContext(ctx, `
				UPDATE tasks
				SET status = $1
				WHERE id = $2
				RETURNING job_id
			`, task.Status, task.ID).Scan(&jobID)

		case "pending":
			// Update task fields only (running_tasks decremented separately via DecrementRunningTasks)
			// The task will re-increment when claimed again
			err = tx.QueryRowContext(ctx, `
				UPDATE tasks
				SET status = $1, retry_count = $2, started_at = $3
				WHERE id = $4
				RETURNING job_id
			`, task.Status, task.RetryCount, task.StartedAt, task.ID).Scan(&jobID)

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

// DecrementRunningTasks immediately decrements the running_tasks counter for a job.
// This is called when a task completes to free up concurrency slots without waiting for batch flush.
// The actual task field updates are still handled by the batch manager for efficiency.
func (q *DbQueue) DecrementRunningTasks(ctx context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("jobID cannot be empty")
	}

	log.Debug().Str("job_id", jobID).Msg("DecrementRunningTasks called")

	query := `
		UPDATE jobs
		SET running_tasks = GREATEST(0, running_tasks - 1)
		WHERE id = $1
	`

	result, err := q.db.client.ExecContext(ctx, query, jobID)
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("DecrementRunningTasks database error")
		return fmt.Errorf("failed to decrement running_tasks for job %s: %w", jobID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("DecrementRunningTasks failed to get rows affected")
	} else {
		log.Debug().Str("job_id", jobID).Int64("rows_affected", rowsAffected).Msg("DecrementRunningTasks executed")
	}

	return nil
}
