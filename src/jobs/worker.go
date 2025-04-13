package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
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
	stopping         atomic.Bool
	activeJobs       sync.WaitGroup
	baseWorkerCount  int
	currentWorkers   int
	jobRequirements  map[string]int // Track worker requirements per job
	workersMutex     sync.RWMutex   // Protect scaling operations
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(db *sql.DB, crawler *crawler.Crawler, numWorkers int) *WorkerPool {
	wp := &WorkerPool{
		db:              db,
		crawler:         crawler,
		numWorkers:      numWorkers,
		baseWorkerCount: numWorkers,
		currentWorkers:  numWorkers,
		jobs:            make(map[string]bool),
		jobRequirements: make(map[string]int),

		stopCh:           make(chan struct{}),
		recoveryInterval: 1 * time.Minute,
	}

	return wp
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
	wp.stopping.Store(true)
	log.Info().Msg("Stopping worker pool")
	close(wp.stopCh)
	wp.wg.Wait()
	log.Info().Msg("Worker pool stopped")
}

// WaitForJobs waits for all active jobs to complete
func (wp *WorkerPool) WaitForJobs() {
	wp.activeJobs.Wait()
}

// AddJob adds a job to be processed by the worker pool
func (wp *WorkerPool) AddJob(jobID string, options *JobOptions) {
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true

	// Store the worker requirement for this job
	requiredWorkers := wp.baseWorkerCount
	if options != nil && options.RequiredWorkers > 0 {
		requiredWorkers = options.RequiredWorkers
		wp.jobRequirements[jobID] = options.RequiredWorkers
	}

	// Calculate the maximum required workers across all jobs
	maxRequired := wp.baseWorkerCount
	for _, count := range wp.jobRequirements {
		if count > maxRequired {
			maxRequired = count
		}
	}
	wp.jobsMutex.Unlock()

	// Scale up if needed
	if maxRequired > wp.currentWorkers {
		wp.scaleWorkers(context.Background(), maxRequired)
	}

	log.Info().
		Str("job_id", jobID).
		Int("required_workers", requiredWorkers).
		Msg("Added job to worker pool")
}

// RemoveJob removes a job from the worker pool
func (wp *WorkerPool) RemoveJob(jobID string) {
	wp.jobsMutex.Lock()
	delete(wp.jobs, jobID)

	// Remove worker requirement for this job
	delete(wp.jobRequirements, jobID)

	// Calculate the maximum required workers across remaining jobs
	maxRequired := wp.baseWorkerCount
	for _, count := range wp.jobRequirements {
		if count > maxRequired {
			maxRequired = count
		}
	}
	wp.jobsMutex.Unlock()

	// Scale down if possible (in a separate goroutine to avoid blocking)
	if maxRequired < wp.currentWorkers {
		go func() {
			wp.workersMutex.Lock()
			defer wp.workersMutex.Unlock()

			log.Info().
				Int("current_workers", wp.currentWorkers).
				Int("target_workers", maxRequired).
				Msg("Scaling down worker pool")

			wp.currentWorkers = maxRequired
			// Note: We don't actually stop excess workers, they'll exit on next task completion
		}()
	}

	log.Info().
		Str("job_id", jobID).
		Msg("Removed job from worker pool")
}

// worker processes tasks from the database
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	defer wp.wg.Done()

	// Create a dedicated crawler for this worker
	workerConfig := crawler.DefaultConfig()
	workerConfig.MaxConcurrency = 1 // Each worker handles one request at a time
	workerConfig.RateLimit = 5      // Rate limit per worker
	workerCrawler := crawler.New(workerConfig, fmt.Sprintf("%d", workerID))

	for {
		select {
		case <-wp.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Check if this worker should exit (we've scaled down)
			wp.workersMutex.RLock()
			shouldExit := workerID >= wp.currentWorkers
			wp.workersMutex.RUnlock()

			if shouldExit {
				log.Info().
					Int("worker_id", workerID).
					Msg("Worker exiting due to scale down")
				return
			}

			if err := wp.processNextTask(ctx, workerID, workerCrawler); err != nil {
				if err != sql.ErrNoRows {
					log.Error().Err(err).Int("worker_id", workerID).Msg("Failed to process task")
				}
				time.Sleep(time.Second)
			}
		}
	}
}

// processNextTask processes the next available task from any active job
func (wp *WorkerPool) processNextTask(ctx context.Context, workerID int, workerCrawler *crawler.Crawler) error {
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
		// Lock before checking job status and tasks
		wp.jobsMutex.Lock()

		// Check if job is still in the pool (might have been removed by another worker)
		if _, exists := wp.jobs[jobID]; !exists {
			wp.jobsMutex.Unlock()
			continue
		}

		// Check pending tasks within the lock
		var pendingCount int
		err := wp.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tasks 
			WHERE job_id = ? AND status = ?
		`, jobID, TaskStatusPending).Scan(&pendingCount)

		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to check pending tasks")
			wp.jobsMutex.Unlock()
			continue
		}

		// If no pending tasks, remove job and update status
		if pendingCount == 0 {
			delete(wp.jobs, jobID)
			wp.jobsMutex.Unlock()

			log.Info().Str("job_id", jobID).Msg("Removing job from worker pool - no pending tasks")

			// Update job status if needed (outside the lock)
			_, err = wp.db.ExecContext(ctx, `
				UPDATE jobs 
				SET status = CASE 
					WHEN status = ? THEN ?
					ELSE status 
				END
				WHERE id = ?
			`, JobStatusRunning, JobStatusCompleted, jobID)

			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status")
			}
			continue
		}

		wp.jobsMutex.Unlock()

		// Rest of the existing task processing code...
		var task *Task
		err = ExecuteInQueue(ctx, func(tx *sql.Tx) error {
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
			task, err = GetNextPendingTaskTx(ctx, tx, jobID)
			return err
		})

		if err == sql.ErrNoRows {
			wp.RemoveJob(jobID)
			continue
		} else if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to process job")
			continue
		}

		if task == nil {
			continue
		}

		// Process the task
		if err := wp.processTask(ctx, task, workerID, workerCrawler); err != nil {
			log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to process task")
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
func (wp *WorkerPool) processTask(ctx context.Context, task *Task, workerID int, workerCrawler *crawler.Crawler) error {
	// Record start time
	taskStart := time.Now()
	taskID := task.ID

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Str("url", task.URL).
		Time("task_picked", taskStart).
		Msg("⏱️ TIMING: Task picked for processing")

	// Only create spans for errors, not for every task
	var span *sentry.Span
	defer func() {
		if span != nil {
			span.Finish()
		}
	}()

	// Use the worker's crawler instead of wp.crawler
	crawlStart := time.Now()
	result, err := workerCrawler.WarmURL(ctx, task.URL)
	crawlDuration := time.Since(crawlStart)

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Str("url", task.URL).
		Dur("crawl_duration_ms", crawlDuration).
		Time("crawl_completed", time.Now()).
		Msg("⏱️ TIMING: URL crawling completed")

	if err != nil {
		// Check if error is retryable (timeout)
		isTimeout := strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "Client.Timeout exceeded")

		if isTimeout && task.RetryCount < MaxTaskRetries {
			// Increment retry count and set back to pending
			task.RetryCount++
			task.Status = TaskStatusPending
			task.Error = err.Error()
			if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task for retry")
			}

			// Add timing log for DB update start
			dbUpdateStart := time.Now()
			log.Info().
				Int("worker_id", workerID).
				Str("task_id", taskID).
				Time("db_update_start", dbUpdateStart).
				Msg("⏱️ TIMING: Starting DB update for failed task")

			if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task status")
			}

			log.Info().
				Int("worker_id", workerID).
				Str("task_id", taskID).
				Dur("db_update_duration_ms", time.Since(dbUpdateStart)).
				Time("db_update_completed", time.Now()).
				Msg("⏱️ TIMING: DB update completed for failed task")

			return nil // Return nil to allow retry
		}

		// If not retryable or max retries exceeded, mark as failed
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
			log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task status")
		}
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
	// Add timing log for DB update start
	dbUpdateStart := time.Now()
	log.Info().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Time("db_update_start", dbUpdateStart).
		Msg("⏱️ TIMING: Starting DB update for completed task")

	err = ExecuteInQueue(ctx, func(tx *sql.Tx) error {
		// Update task status
		if err := UpdateTaskStatusTx(ctx, tx, task); err != nil {
			return err
		}

		// Insert into crawl_results if task completed successfully
		if task.Status == TaskStatusCompleted {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO crawl_results (job_id, task_id, url, response_time, status_code, error, cache_status)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, task.JobID, task.ID, task.URL, task.ResponseTime, task.StatusCode, task.Error, task.CacheStatus)

			if err != nil {
				return err
			}

			// Single query to update completed count, progress, and potentially mark job as complete
			_, err = tx.ExecContext(ctx, `
				UPDATE jobs 
				SET 
					completed_tasks = completed_tasks + 1,
					progress = CAST(100.0 * (
						(SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status = ?) +
						(SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status = ?)
					) AS FLOAT) / total_tasks,
					status = CASE 
						WHEN (SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status IN (?,?,?)) >= total_tasks THEN ?
						ELSE status 
					END,
					completed_at = CASE 
						WHEN (SELECT COUNT(*) FROM tasks WHERE job_id = ? AND status IN (?,?,?)) >= total_tasks THEN ?
						ELSE completed_at 
					END
				WHERE id = ? AND status = ?
			`, task.JobID, TaskStatusCompleted, task.JobID, TaskStatusFailed,
				task.JobID, TaskStatusCompleted, TaskStatusFailed, TaskStatusSkipped, JobStatusCompleted,
				task.JobID, TaskStatusCompleted, TaskStatusFailed, TaskStatusSkipped, time.Now(),
				task.JobID, JobStatusRunning)

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

	dbUpdateDuration := time.Since(dbUpdateStart)
	totalDuration := time.Since(taskStart)

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Dur("db_update_duration_ms", dbUpdateDuration).
		Dur("total_task_duration_ms", totalDuration).
		Time("db_update_completed", time.Now()).
		Float64("db_percentage", float64(dbUpdateDuration)/float64(totalDuration)*100).
		Msg("⏱️ TIMING: DB update completed for task")

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

	// Increase batch size for better performance
	const batchSize = 500

	// Update total tasks count once at the start in a transaction
	err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
		// First verify the job exists and is in a valid state
		var status string
		err := tx.QueryRowContext(ctx, "SELECT status FROM jobs WHERE id = ?", jobID).Scan(&status)
		if err != nil {
			return fmt.Errorf("failed to verify job: %w", err)
		}

		if status != string(JobStatusPending) && status != string(JobStatusRunning) {
			return fmt.Errorf("job is in invalid state: %s", status)
		}

		// Update total tasks count
		_, err = tx.ExecContext(ctx, `
			UPDATE jobs 
			SET total_tasks = total_tasks + ?
			WHERE id = ?
		`, len(urls), jobID)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update total tasks: %w", err)
	}

	// Process URLs in larger batches
	for i := 0; i < len(urls); i += batchSize {
		end := i + batchSize
		if end > len(urls) {
			end = len(urls)
		}

		batch := urls[i:end]
		if len(batch) == 0 {
			continue
		}

		err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
			// Prepare a single statement with multiple value sets
			valueStrings := make([]string, 0, len(batch))
			valueArgs := make([]interface{}, 0, len(batch)*9)
			now := time.Now()

			for _, url := range batch {
				// Validate URL before adding
				if url == "" {
					continue
				}
				taskID := uuid.New().String()
				valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
				valueArgs = append(valueArgs,
					taskID, jobID, url, TaskStatusPending, depth, now, 0, sourceType, sourceURL)
			}

			if len(valueStrings) == 0 {
				return nil // Skip if no valid URLs in batch
			}

			stmt := fmt.Sprintf(`
				INSERT INTO tasks (
					id, job_id, url, status, depth, created_at, retry_count,
					source_type, source_url
				) VALUES %s`, strings.Join(valueStrings, ","))

			_, err := tx.ExecContext(ctx, stmt, valueArgs...)
			return err
		})

		if err != nil {
			// If batch insert fails, adjust total_tasks count back down
			adjustErr := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					UPDATE jobs 
					SET total_tasks = total_tasks - ?
					WHERE id = ?
				`, len(batch), jobID)
				return err
			})
			if adjustErr != nil {
				log.Error().Err(adjustErr).Msg("Failed to adjust total_tasks after batch failure")
			}
			return fmt.Errorf("failed to insert task batch: %w", err)
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
			wp.AddJob(jobID, nil)

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

// processJob processes a single job
func (wp *WorkerPool) processJob(job *Job) {
	if wp.stopping.Load() {
		return // Don't start new jobs during shutdown
	}

	wp.activeJobs.Add(1)
	defer wp.activeJobs.Done()

	// ... rest of your existing job processing logic ...
}

func (wp *WorkerPool) updateFailedTaskCount(ctx context.Context, jobID string) error {
	_, err := wp.db.ExecContext(ctx, `
		UPDATE jobs 
		SET failed_tasks = failed_tasks + 1
		WHERE id = ?
	`, jobID)
	return err
}

// scaleWorkers increases the worker pool size to the target number
func (wp *WorkerPool) scaleWorkers(ctx context.Context, targetWorkers int) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("Recovered from panic in scaleWorkers")
		}
	}()

	wp.workersMutex.Lock()
	defer wp.workersMutex.Unlock()

	if targetWorkers <= wp.currentWorkers {
		return // No need to scale up
	}

	workersToAdd := targetWorkers - wp.currentWorkers

	log.Info().
		Int("current_workers", wp.currentWorkers).
		Int("adding_workers", workersToAdd).
		Int("target_workers", targetWorkers).
		Msg("Scaling worker pool")

	// Start additional workers
	for i := 0; i < workersToAdd; i++ {
		workerID := wp.currentWorkers + i
		wp.wg.Add(1)
		go wp.worker(ctx, workerID)
	}

	wp.currentWorkers = targetWorkers
}

// Add a method to gracefully terminate excess workers
func (wp *WorkerPool) terminateExcessWorkers() {
	wp.workersMutex.Lock()
	defer wp.workersMutex.Unlock()

	// Note: The check for shouldExit in the worker method will handle this
	log.Info().
		Int("current_workers", wp.currentWorkers).
		Msg("Worker count adjusted, excess workers will exit on next task attempt")
}
