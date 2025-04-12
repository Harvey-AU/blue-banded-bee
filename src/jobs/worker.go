package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WorkerPool manages a pool of workers that process crawl tasks
type WorkerPool struct {
	db               *sql.DB
	crawler          *crawler.Crawler
	numWorkers       int
	jobs             map[string]bool
	jobsMutex        sync.RWMutex
	stopCh           chan struct{}
	wg               sync.WaitGroup
	recoveryInterval time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(db *sql.DB, crawler *crawler.Crawler, numWorkers int) *WorkerPool {
	return &WorkerPool{
		db:               db,
		crawler:          crawler,
		numWorkers:       numWorkers,
		jobs:             make(map[string]bool),
		stopCh:           make(chan struct{}),
		recoveryInterval: 1 * time.Minute, // Check every minute
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.numWorkers).Msg("Starting worker pool")

	wp.wg.Add(wp.numWorkers)
	for i := 0; i < wp.numWorkers; i++ {
		go wp.worker(ctx, i)
	}

	// Start the recovery monitor
	wp.wg.Add(1)
	go wp.recoveryMonitor(ctx)
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	log.Info().Msg("Stopping worker pool")
	close(wp.stopCh)
	wp.wg.Wait()
	log.Info().Msg("Worker pool stopped")
}

// AddJob adds a job to be processed by the worker pool
func (wp *WorkerPool) AddJob(jobID string) {
	wp.jobsMutex.Lock()
	defer wp.jobsMutex.Unlock()
	wp.jobs[jobID] = true
	log.Info().Str("job_id", jobID).Msg("Added job to worker pool")
}

// RemoveJob removes a job from the worker pool
func (wp *WorkerPool) RemoveJob(jobID string) {
	wp.jobsMutex.Lock()
	defer wp.jobsMutex.Unlock()
	delete(wp.jobs, jobID)
	log.Info().Str("job_id", jobID).Msg("Removed job from worker pool")
}

// worker processes tasks from the database
func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	log.Info().Int("worker_id", id).Msg("Worker started")

	// Add at the start of the loop
	log.Info().Int("worker_id", id).Msg("Worker checking for tasks...")

	// Worker loop
	for {
		select {
		case <-wp.stopCh:
			log.Info().Int("worker_id", id).Msg("Worker stopping")
			return
		case <-time.After(100 * time.Millisecond): // Poll interval
			// Process available tasks
			if err := wp.processNextTask(ctx, id); err != nil {
				log.Error().Err(err).Int("worker_id", id).Msg("Error processing task")
			}
		}
	}
}

// processNextTask processes the next available task from any active job
func (wp *WorkerPool) processNextTask(ctx context.Context, workerID int) error {
	// Get active jobs
	wp.jobsMutex.RLock()
	activeJobs := make([]string, 0, len(wp.jobs))
	for jobID := range wp.jobs {
		activeJobs = append(activeJobs, jobID)
	}
	wp.jobsMutex.RUnlock()

	if len(activeJobs) == 0 {
		return nil // No active jobs
	}

	// Try to get a task from each active job
	for _, jobID := range activeJobs {
		// Use ExecuteInQueue to perform both operations in a single transaction
		var task *Task
		err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
			// Check job status within transaction
			row := tx.QueryRowContext(ctx, `
				SELECT status FROM jobs WHERE id = ?
			`, jobID)

			var status string
			if err := row.Scan(&status); err != nil {
				return err
			}

			if status != string(JobStatusRunning) && status != string(JobStatusPending) {
				// Mark for removal outside transaction
				return sql.ErrNoRows // Using this as signal
			}

			// If job is active, get task within same transaction
			var err error
			task, err = GetNextPendingTaskTx(ctx, tx, jobID)
			return err
		})

		if err == sql.ErrNoRows {
			// Job is not active
			wp.RemoveJob(jobID)
			continue
		} else if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to process job")
			continue
		}

		if task == nil {
			continue // No pending tasks
		}

		// Process the task
		if err := wp.processTask(ctx, task, workerID); err != nil {
			log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to process task")

			// Update task as failed
			task.Status = TaskStatusFailed
			task.Error = err.Error()
			if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task status")
			}
		}

		return nil // Processed a task
	}

	return nil // No tasks to process
}

// processTask processes a single task using the crawler
func (wp *WorkerPool) processTask(ctx context.Context, task *Task, workerID int) error {
	// Only create spans for errors, not for every task
	var span *sentry.Span
	defer func() {
		if span != nil {
			span.Finish()
		}
	}()

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", task.ID).
		Str("url", task.URL).
		Msg("Processing task")

	// Crawl the URL (no DB operation)
	result, err := wp.crawler.WarmURL(ctx, task.URL)
	if err != nil {
		// Only create span on error
		span = sentry.StartSpan(ctx, "worker.process_task.error")
		span.SetTag("worker_id", fmt.Sprintf("%d", workerID))
		span.SetTag("task_id", task.ID)
		span.SetTag("url", task.URL)
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return err
	}

	// Update task with result (still no DB operation)
	task.StatusCode = result.StatusCode
	task.ResponseTime = result.ResponseTime
	task.CacheStatus = result.CacheStatus
	task.ContentType = result.ContentType
	task.Status = TaskStatusCompleted
	if result.Error != "" {
		task.Error = result.Error
	}

	// Use queue for critical DB operations
	err = ExecuteInQueue(ctx, func(tx *sql.Tx) error {
		// Update task status
		if err := UpdateTaskStatusTx(ctx, tx, task); err != nil {
			return err
		}

		// Insert into crawl_results if task completed successfully
		if task.Status == TaskStatusCompleted {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO crawl_results (url, response_time, status_code, error, cache_status)
				VALUES (?, ?, ?, ?, ?)
			`, task.URL, task.ResponseTime, task.StatusCode, task.Error, task.CacheStatus)

			if err != nil {
				return err
			}

			// Single query to update completed count, progress, and potentially mark job as complete
			_, err = tx.ExecContext(ctx, `
				UPDATE jobs 
				SET 
					completed_tasks = completed_tasks + 1,
					progress = CAST((completed_tasks + 1 + failed_tasks) AS FLOAT) / total_tasks * 100,
					status = CASE 
						WHEN (completed_tasks + 1 + failed_tasks) >= total_tasks THEN ?
						ELSE status 
					END,
					completed_at = CASE 
						WHEN (completed_tasks + 1 + failed_tasks) >= total_tasks THEN ?
						ELSE completed_at 
					END
				WHERE id = ? AND status = ?
			`, JobStatusCompleted, time.Now(), task.JobID, JobStatusRunning)

			return err
		} else if task.Status == TaskStatusFailed {
			// Single query to update failed count, progress, and potentially mark job as complete
			_, err = tx.ExecContext(ctx, `
				UPDATE jobs 
				SET 
					failed_tasks = failed_tasks + 1,
					progress = CAST((completed_tasks + failed_tasks + 1) AS FLOAT) / total_tasks * 100,
					status = CASE 
						WHEN (completed_tasks + failed_tasks + 1) >= total_tasks THEN ?
						ELSE status 
					END,
					completed_at = CASE 
						WHEN (completed_tasks + failed_tasks + 1) >= total_tasks THEN ?
						ELSE completed_at 
					END
				WHERE id = ? AND status = ?
			`, JobStatusCompleted, time.Now(), task.JobID, JobStatusRunning)

			return err
		}

		return nil
	})

	if err != nil {
		log.Error().Err(err).Str("task_id", task.ID).Str("job_id", task.JobID).Msg("Failed to update task and job status")
	}

	return err
}

// EnqueueURLs adds multiple URLs as tasks for a job
func EnqueueURLs(ctx context.Context, db *sql.DB, jobID string, urls []string, sourceType string, sourceURL string, depth int) error {
	span := sentry.StartSpan(ctx, "jobs.enqueue_urls")
	defer span.Finish()

	span.SetTag("job_id", jobID)
	span.SetData("url_count", len(urls))

	// Add batch size limit to avoid overly large transactions
	const batchSize = 100

	// Process URLs in batches
	for i := 0; i < len(urls); i += batchSize {
		end := i + batchSize
		if end > len(urls) {
			end = len(urls)
		}

		batch := urls[i:end]

		err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
			if len(batch) > 0 {
				// Prepare a single statement with multiple value sets
				valueStrings := make([]string, 0, len(batch))
				valueArgs := make([]interface{}, 0, len(batch)*9) // 9 parameters per row

				now := time.Now()

				for _, url := range batch {
					taskID := uuid.New().String()
					valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
					valueArgs = append(valueArgs,
						taskID, jobID, url, TaskStatusPending, depth, now, 0, sourceType, sourceURL)
				}

				stmt := fmt.Sprintf(`
					INSERT INTO tasks (
						id, job_id, url, status, depth, created_at, retry_count,
						source_type, source_url
					) VALUES %s`, strings.Join(valueStrings, ","))

				_, err := tx.ExecContext(ctx, stmt, valueArgs...)
				if err != nil {
					return err
				}

				// Add this: Update job total_tasks after inserting
				_, err = tx.ExecContext(ctx, `
					UPDATE jobs 
					SET total_tasks = total_tasks + ?
					WHERE id = ?
				`, len(batch), jobID)

				if err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// StartTaskMonitor starts a background process that monitors for pending tasks
func (wp *WorkerPool) StartTaskMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-wp.stopCh:
				return
			case <-ticker.C:
				if err := wp.checkForPendingTasks(ctx); err != nil {
					log.Error().Err(err).Msg("Error checking for pending tasks")
				}
			}
		}
	}()

	log.Info().Msg("Task monitor started")
}

// checkForPendingTasks looks for any pending tasks and adds their jobs to the pool
func (wp *WorkerPool) checkForPendingTasks(ctx context.Context) error {
	// Query for jobs with pending tasks
	rows, err := wp.db.QueryContext(ctx, `
		SELECT DISTINCT job_id FROM tasks 
		WHERE status = ? 
		LIMIT 100
	`, TaskStatusPending)

	if err != nil {
		return err
	}
	defer rows.Close()

	// For each job with pending tasks, add it to the worker pool
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			log.Error().Err(err).Msg("Failed to scan job ID")
			continue
		}

		// Check if already in our active jobs
		wp.jobsMutex.RLock()
		active := wp.jobs[jobID]
		wp.jobsMutex.RUnlock()

		if !active {
			// Add job to the worker pool
			wp.AddJob(jobID)

			// Update job status if needed
			_, err := wp.db.ExecContext(ctx, `
				UPDATE jobs
				SET status = ?, started_at = CASE WHEN started_at IS NULL THEN ? ELSE started_at END
				WHERE id = ? AND status = ?
			`, JobStatusRunning, time.Now(), jobID, JobStatusPending)

			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status")
			}
		}
	}

	return rows.Err()
}

// recoverStaleTasks checks for and resets stale tasks
func (wp *WorkerPool) recoverStaleTasks(ctx context.Context) error {
	return ExecuteInQueue(ctx, func(tx *sql.Tx) error {
		staleTime := time.Now().Add(-TaskStaleTimeout)

		// Get stale tasks
		rows, err := tx.QueryContext(ctx, `
			SELECT id, job_id, url, retry_count 
			FROM tasks 
			WHERE status = ? 
			AND started_at < ?
		`, TaskStatusRunning, staleTime)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var taskID, jobID, url string
			var retryCount int
			if err := rows.Scan(&taskID, &jobID, &url, &retryCount); err != nil {
				continue
			}

			if retryCount >= MaxTaskRetries {
				// Mark as failed if max retries exceeded
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks 
					SET status = ?,
						error = ?,
						completed_at = ?
					WHERE id = ?
				`, TaskStatusFailed, "Max retries exceeded", time.Now(), taskID)
			} else {
				// Reset to pending for retry
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks 
					SET status = ?,
						started_at = NULL,
						retry_count = retry_count + 1
					WHERE id = ?
				`, TaskStatusPending, taskID)
			}

			if err != nil {
				log.Error().Err(err).
					Str("task_id", taskID).
					Msg("Failed to update stale task")
			}
		}

		return rows.Err()
	})
}

// recoveryMonitor periodically checks for and recovers stale tasks
func (wp *WorkerPool) recoveryMonitor(ctx context.Context) {
	defer wp.wg.Done()

	ticker := time.NewTicker(wp.recoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			if err := wp.recoverStaleTasks(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to recover stale tasks")
			}
		}
	}
}
