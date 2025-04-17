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
	taskBatch        *TaskBatch
	batchTimer       *time.Ticker
	cleanupInterval  time.Duration
}

// TaskBatch holds groups of tasks for batch processing
type TaskBatch struct {
	tasks     []*Task
	jobCounts map[string]struct {
		completed int
		failed    int
	}
	mu sync.Mutex
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
		taskBatch: &TaskBatch{
			tasks:     make([]*Task, 0, 50),
			jobCounts: make(map[string]struct{ completed, failed int }),
		},
		batchTimer:      time.NewTicker(10 * time.Second),
		cleanupInterval: time.Minute, // Run cleanup every minute
	}

	// Start the batch processor
	wp.wg.Add(1)
	go wp.processBatches(context.Background())

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

	// Run initial cleanup
	if err := CleanupStuckJobs(ctx, wp.db); err != nil {
		log.Error().Err(err).Msg("Failed to perform initial job cleanup")
	}

	// Start monitors
	wp.StartTaskMonitor(ctx)
	wp.StartCleanupMonitor(ctx)
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.stopping.Store(true)
	log.Debug().Msg("Stopping worker pool")
	close(wp.stopCh)
	wp.wg.Wait()
	log.Debug().Msg("Worker pool stopped")
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

	log.Debug().
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

			log.Debug().
				Int("current_workers", wp.currentWorkers).
				Int("target_workers", maxRequired).
				Msg("Scaling down worker pool")

			wp.currentWorkers = maxRequired
			// Note: We don't actually stop excess workers, they'll exit on next task completion
		}()
	}

	log.Debug().
		Str("job_id", jobID).
		Msg("Removed job from worker pool")
}

// worker processes tasks from the database
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	defer wp.wg.Done()

	// Create a dedicated crawler for this worker
	workerConfig := crawler.DefaultConfig()
	workerConfig.MaxConcurrency = 1
	workerConfig.RateLimit = 5
	workerCrawler := crawler.New(workerConfig, fmt.Sprintf("%d", workerID))

	// Add a counter for consecutive errors to implement exponential backoff
	consecutiveErrors := 0
	maxSleep := 10 * time.Second

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
				return
			}

			if err := wp.processNextTask(ctx, workerID, workerCrawler); err != nil {
				consecutiveErrors++
				sleepTime := time.Duration(100*(1<<uint(min(consecutiveErrors, 6)))) * time.Millisecond
				if sleepTime > maxSleep {
					sleepTime = maxSleep
				}

				if err != sql.ErrNoRows {
					log.Error().Err(err).Int("worker_id", workerID).Msg("Failed to process task")
				}
				time.Sleep(sleepTime)
			} else {
				consecutiveErrors = 0
				// Always sleep a little between tasks (reduce lock contention)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
}

// processNextTask processes the next available task from any active job
func (wp *WorkerPool) processNextTask(ctx context.Context, workerID int, workerCrawler *crawler.Crawler) error {
	task, err := GetNextPendingTask(ctx, wp.db)
	if err == sql.ErrNoRows {
		// This isn't really an error - just means no tasks available
		// Sleep a bit to prevent tight loop
		time.Sleep(100 * time.Millisecond)
		return nil
	}
	if err != nil {
		// This is a real error
		return err
	}
	if task == nil {
		// No tasks available
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Process task...
	return nil
}

// processTask processes a single task using the crawler
func (wp *WorkerPool) processTask(ctx context.Context, task *Task, workerID int, workerCrawler *crawler.Crawler) error {
	// Record start time
	taskStart := time.Now()
	taskID := task.ID

	log.Debug().
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

	log.Debug().
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

			dbUpdateStart := time.Now()
			log.Debug().
				Int("worker_id", workerID).
				Str("task_id", taskID).
				Time("db_update_start", dbUpdateStart).
				Msg("⏱️ TIMING: Starting DB update for failed task")

			if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task for retry")
				return err // Return error to prevent task from being lost
			}

			log.Debug().
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

	// Update task with the result data
	task.Status = TaskStatusCompleted
	// Store the crawler result data in the task
	task.StatusCode = result.StatusCode
	task.ResponseTime = result.ResponseTime
	task.CacheStatus = result.CacheStatus
	task.ContentType = result.ContentType

	// Only keep high-level error message in tasks table
	if result.Error != "" {
		task.Error = "Failed: " + result.Error[:min(20, len(result.Error))] + "..."
	}

	// All detailed result data will go to crawl_results only

	// Record start time for batch logging
	dbUpdateStart := time.Now()
	log.Debug().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Time("db_batch_queued", dbUpdateStart).
		Msg("⏱️ TIMING: Task queued for batch DB update")

	wp.taskBatch.mu.Lock()
	// Add task to batch
	wp.taskBatch.tasks = append(wp.taskBatch.tasks, task)

	// Update job counters
	counts, exists := wp.taskBatch.jobCounts[task.JobID]
	if !exists {
		counts = struct{ completed, failed int }{0, 0}
	}

	if task.Status == TaskStatusCompleted {
		counts.completed++
	} else if task.Status == TaskStatusFailed {
		counts.failed++
	}
	wp.taskBatch.jobCounts[task.JobID] = counts

	// If batch is large enough, trigger immediate flush
	shouldFlush := len(wp.taskBatch.tasks) >= 50
	batchSize := len(wp.taskBatch.tasks)
	wp.taskBatch.mu.Unlock()

	// Log batch queuing information
	log.Debug().
		Int("worker_id", workerID).
		Str("task_id", taskID).
		Int("current_batch_size", batchSize).
		Bool("triggering_flush", shouldFlush).
		Dur("task_processing_duration_ms", time.Since(taskStart)).
		Msg("⏱️ TIMING: Task added to batch queue")

	if shouldFlush {
		go wp.flushBatches(ctx)
	}

	return nil
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

	log.Debug().
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
	log.Debug().
		Int("current_workers", wp.currentWorkers).
		Msg("Worker count adjusted, excess workers will exit on next task attempt")
}

// Batch processor goroutine
func (wp *WorkerPool) processBatches(ctx context.Context) {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.batchTimer.C:
			wp.flushBatches(ctx)
		case <-wp.stopCh:
			wp.flushBatches(ctx) // Final flush before shutdown
			return
		case <-ctx.Done():
			return
		}
	}
}

// Flush collected tasks in a batch
func (wp *WorkerPool) flushBatches(ctx context.Context) {
	wp.taskBatch.mu.Lock()
	tasks := wp.taskBatch.tasks
	jobCounts := wp.taskBatch.jobCounts

	// Reset batches
	wp.taskBatch.tasks = make([]*Task, 0, 50)
	wp.taskBatch.jobCounts = make(map[string]struct{ completed, failed int })
	wp.taskBatch.mu.Unlock()

	if len(tasks) == 0 {
		return // Nothing to flush
	}

	// Process the batch in a single transaction
	batchStart := time.Now()
	log.Debug().
		Int("batch_size", len(tasks)).
		Int("job_count", len(jobCounts)).
		Time("batch_update_start", batchStart).
		Msg("⏱️ TIMING: Starting batch DB update")

	// Execute everything in ONE queue operation instead of separate ones
	err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
		// 1. Update all tasks in a single statement with CASE
		if len(tasks) > 0 {
			taskUpdateStart := time.Now()
			stmt, err := tx.PrepareContext(ctx, `
				UPDATE tasks
				SET status = ?, 
					completed_at = ?,
					error = ? -- Only include error (for failure reason)
				WHERE id = ?
			`)
			if err != nil {
				return err
			}
			defer stmt.Close()

			for _, task := range tasks {
				if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
					if task.CompletedAt.IsZero() {
						task.CompletedAt = time.Now()
					}
					_, err := stmt.ExecContext(ctx,
						task.Status, task.CompletedAt,
						task.Error, task.ID)
					if err != nil {
						return err
					}
				}
			}
			log.Debug().
				Dur("task_update_duration_ms", time.Since(taskUpdateStart)).
				Int("task_count", len(tasks)).
				Msg("⏱️ TIMING: Completed batch task updates")
		}

		// 2. Batch insert all crawl results
		if len(tasks) > 0 {
			resultInsertStart := time.Now()
			completedTasks := filterTasksByStatus(tasks, TaskStatusCompleted)
			if len(completedTasks) > 0 {
				// Convert Task objects to CrawlResultData
				crawlResults := make([]CrawlResultData, 0, len(completedTasks))
				for _, task := range completedTasks {
					// Use the task's result data that we stored earlier
					crawlResults = append(crawlResults, CrawlResultData{
						JobID:        task.JobID,
						TaskID:       task.ID,
						URL:          task.URL,
						ResponseTime: task.ResponseTime,
						StatusCode:   task.StatusCode,
						Error:        task.Error,
						CacheStatus:  task.CacheStatus,
						ContentType:  task.ContentType,
					})
				}

				if err := batchInsertCrawlResults(ctx, tx, crawlResults); err != nil {
					return err
				}
			}
			log.Debug().
				Dur("result_insert_duration_ms", time.Since(resultInsertStart)).
				Int("completed_task_count", len(completedTasks)).
				Msg("⏱️ TIMING: Completed batch result inserts")
		}

		// 3. Update job counters once per job
		if len(jobCounts) > 0 {
			jobUpdateStart := time.Now()
			for jobID, counts := range jobCounts {
				// Use a simpler update without the complex CASE expressions
				_, err := tx.ExecContext(ctx, `
					UPDATE jobs
					SET 
						completed_tasks = completed_tasks + ?,
						failed_tasks = failed_tasks + ?,
						progress = CAST(100.0 * (completed_tasks + failed_tasks) / 
								  CASE WHEN total_tasks = 0 THEN 1 ELSE total_tasks END AS FLOAT)
					WHERE id = ?
				`,
					counts.completed, counts.failed, jobID)

				if err != nil {
					return err
				}
			}

			// After updating counters, check if we need to mark the job as completed
			for jobID, _ := range jobCounts {
				var total, completed, failed int
				err := tx.QueryRowContext(ctx, `
					SELECT total_tasks, completed_tasks, failed_tasks 
					FROM jobs WHERE id = ?
				`, jobID).Scan(&total, &completed, &failed)

				if err != nil {
					return err
				}

				// If all tasks are done, mark the job as completed
				if total > 0 && completed+failed >= total {
					_, err = tx.ExecContext(ctx, `
						UPDATE jobs SET
							status = ?,
							completed_at = ?,
							progress = 100.0
						WHERE id = ? AND status = ?
					`, string(JobStatusCompleted), time.Now(), jobID, string(JobStatusRunning))

					if err != nil {
						return err
					}

					log.Debug().
						Str("job_id", jobID).
						Int("total_tasks", total).
						Int("completed", completed).
						Int("failed", failed).
						Msg("Job marked as completed")
				}
			}

			log.Debug().
				Dur("job_update_duration_ms", time.Since(jobUpdateStart)).
				Int("job_count", len(jobCounts)).
				Msg("⏱️ TIMING: Completed batch job updates")
		}

		return nil
	})

	batchDuration := time.Since(batchStart)
	log.Debug().
		Int("task_count", len(tasks)).
		Int("job_count", len(jobCounts)).
		Dur("batch_duration_ms", batchDuration).
		Time("batch_completed", time.Now()).
		Bool("success", err == nil).
		Msg("⏱️ TIMING: Batch DB update completed")

	if err != nil {
		log.Error().Err(err).Int("task_count", len(tasks)).Msg("Failed to process task batch")
	}
}

// Helper functions for batch operations
func batchUpdateTasks(ctx context.Context, tx *sql.Tx, tasks []*Task) error {
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE tasks
		SET status = ?, started_at = ?, completed_at = ?,
			error = ?, retry_count = ?
		WHERE id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, task := range tasks {
		_, err := stmt.ExecContext(ctx,
			task.Status, task.StartedAt, task.CompletedAt,
			task.Error, task.RetryCount, task.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// Add new method to start the cleanup monitor
func (wp *WorkerPool) StartCleanupMonitor(ctx context.Context) {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		ticker := time.NewTicker(wp.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-wp.stopCh:
				return
			case <-ticker.C:
				if err := CleanupStuckJobs(ctx, wp.db); err != nil {
					log.Error().Err(err).Msg("Failed to cleanup stuck jobs")
				}
			}
		}
	}()
	log.Info().Msg("Job cleanup monitor started")
}
