package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/Harvey-AU/blue-banded-bee/internal/util"
	"github.com/getsentry/sentry-go"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// WorkerPool manages a pool of workers that process crawl tasks
type WorkerPool struct {
	db               *sql.DB
	dbQueue          *db.DbQueue
	dbConfig         *db.Config
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
	jobRequirements  map[string]int
	workersMutex     sync.RWMutex
	taskBatch        *TaskBatch
	batchTimer       *time.Ticker
	cleanupInterval  time.Duration
	notifyCh         chan struct{}
	jobManager       *JobManager // Reference to JobManager for duplicate checking
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
func NewWorkerPool(db *sql.DB, dbQueue *db.DbQueue, crawler *crawler.Crawler, numWorkers int, dbConfig *db.Config) *WorkerPool {
	// Validate inputs
	if db == nil {
		panic("database connection is required")
	}
	if dbQueue == nil {
		panic("database queue is required")
	}
	if crawler == nil {
		panic("crawler is required")
	}
	if numWorkers < 1 {
		panic("numWorkers must be at least 1")
	}
	if dbConfig == nil {
		panic("database configuration is required")
	}

	wp := &WorkerPool{
		db:              db,
		dbQueue:         dbQueue,
		dbConfig:        dbConfig,
		crawler:         crawler,
		numWorkers:      numWorkers,
		baseWorkerCount: numWorkers,
		currentWorkers:  numWorkers,
		jobs:            make(map[string]bool),
		jobRequirements: make(map[string]int),

		stopCh:           make(chan struct{}),
		notifyCh:         make(chan struct{}, 1), // Buffer of 1 to prevent blocking
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

	// Start the notification listener
	wp.wg.Add(1)
	go wp.listenForNotifications(context.Background())

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
	if err := wp.CleanupStuckJobs(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to perform initial job cleanup")
	}

	// Recover jobs that were running before restart
	if err := wp.recoverRunningJobs(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to recover running jobs on startup")
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

	log.Info().Int("worker_id", workerID).Msg("Starting worker")

	// Track consecutive no-task counts for backoff
	consecutiveNoTasks := 0
	maxSleep := 30 * time.Second
	baseSleep := 200 * time.Millisecond // Faster processing when active

	for {
		select {
		case <-wp.stopCh:
			log.Debug().Int("worker_id", workerID).Msg("Worker received stop signal")
			return
		case <-ctx.Done():
			log.Debug().Int("worker_id", workerID).Msg("Worker context cancelled")
			return
		case <-wp.notifyCh:
			// Reset backoff when notified of new tasks
			consecutiveNoTasks = 0
		default:
			// Check if this worker should exit (we've scaled down)
			wp.workersMutex.RLock()
			shouldExit := workerID >= wp.currentWorkers
			wp.workersMutex.RUnlock()

			if shouldExit {
				return
			}

			if err := wp.processNextTask(ctx); err != nil {
				if err == sql.ErrNoRows {
					consecutiveNoTasks++
					// Only log occasionally during quiet periods
					if consecutiveNoTasks == 1 || consecutiveNoTasks%10 == 0 {
						log.Debug().Msg("Waiting for new tasks")
					}
					// Exponential backoff with a maximum
					sleepTime := time.Duration(float64(baseSleep) * math.Pow(1.5, float64(min(consecutiveNoTasks, 10))))
					if sleepTime > maxSleep {
						sleepTime = maxSleep
					}

					// Wait for either the backoff duration or a notification
					select {
					case <-time.After(sleepTime):
					case <-wp.notifyCh:
						consecutiveNoTasks = 0
					case <-wp.stopCh:
						return
					case <-ctx.Done():
						return
					}
				} else {
					log.Error().Err(err).Int("worker_id", workerID).Msg("Failed to process task")
					time.Sleep(baseSleep)
				}
			} else {
				consecutiveNoTasks = 0
				// Quick sleep between tasks when active
				time.Sleep(baseSleep)
			}
		}
	}
}

// processNextTask processes the next available task from any active job
func (wp *WorkerPool) processNextTask(ctx context.Context) error {
	// Get the list of active jobs
	wp.jobsMutex.RLock()
	activeJobs := make([]string, 0, len(wp.jobs))
	for jobID := range wp.jobs {
		activeJobs = append(activeJobs, jobID)
	}
	wp.jobsMutex.RUnlock()

	// If no active jobs, return immediately
	if len(activeJobs) == 0 {
		return sql.ErrNoRows
	}

	// Try to get a task from each active job
	for _, jobID := range activeJobs {
		task, err := wp.dbQueue.GetNextTask(ctx, jobID)
		if err == sql.ErrNoRows {
			continue // Try next job
		}
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Error getting next pending task")
			return err // Return actual errors
		}
		if task != nil {
			log.Info().
				Str("task_id", task.ID).
				Str("job_id", task.JobID).
				Int("page_id", task.PageID).
				Str("path", task.Path).
				Msg("Found and claimed pending task")

			// Convert db.Task to jobs.Task for processing
			jobsTask := &Task{
				ID:         task.ID,
				JobID:      task.JobID,
				PageID:     task.PageID,
				Path:       task.Path,
				Status:     TaskStatus(task.Status),
				CreatedAt:  task.CreatedAt,
				StartedAt:  task.StartedAt, 
				RetryCount: task.RetryCount,
				SourceType: task.SourceType,
				SourceURL:  task.SourceURL,
			}
			
			// Need to fetch additional info from the database
			var domainName string
			var findLinks bool
			err := wp.db.QueryRowContext(ctx, `
				SELECT d.name, j.find_links 
				FROM domains d
				JOIN jobs j ON j.domain_id = d.id
				WHERE j.id = $1
			`, task.JobID).Scan(&domainName, &findLinks)
			
			if err != nil {
				log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to get domain name and find_links setting")
			} else {
				jobsTask.DomainName = domainName
				jobsTask.FindLinks = findLinks
			}
			
			// Process the task
			result, err := wp.processTask(ctx, jobsTask)
			now := time.Now()
			if err != nil {
				// mark as failed
				task.Status = string(TaskStatusFailed)
				task.CompletedAt = now
				task.Error = err.Error()
				updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
				if updErr != nil {
					log.Error().Err(updErr).Str("task_id", task.ID).Msg("Failed to mark task as failed")
				}
			} else {
				// mark as completed with metrics
				task.Status = string(TaskStatusCompleted)
				task.CompletedAt = now
				task.StatusCode = result.StatusCode
				task.ResponseTime = result.ResponseTime
				task.CacheStatus = result.CacheStatus
				task.ContentType = result.ContentType
				updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
				if updErr != nil {
					log.Error().Err(updErr).Str("task_id", task.ID).Msg("Failed to mark task as completed")
				}
			}
			// update job progress
			if err := wp.dbQueue.UpdateJobProgress(ctx, task.JobID); err != nil {
				log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to update job progress via helper")
			}
			return nil
		}
	}

	// No tasks found in any job
	return sql.ErrNoRows
}

// EnqueueURLs adds multiple URLs as tasks for a job
// Legacy wrapper that delegates to dbQueue.EnqueueURLs
func (wp *WorkerPool) EnqueueURLs(ctx context.Context, jobID string, pageIDs []int, urls []string, sourceType string, sourceURL string) error {
	log.Debug().
		Str("job_id", jobID).
		Str("source_type", sourceType).
		Int("url_count", len(urls)).
		Msg("EnqueueURLs called")
	
	// Check if we have a job manager to use for duplicate checking
	// If not, fall back to direct dbQueue usage
	if wp.jobManager != nil {
		return wp.jobManager.EnqueueJobURLs(ctx, jobID, pageIDs, urls, sourceType, sourceURL)
	}
	
	return wp.dbQueue.EnqueueURLs(ctx, jobID, pageIDs, urls, sourceType, sourceURL)
}

// StartTaskMonitor starts a background process that monitors for pending tasks
func (wp *WorkerPool) StartTaskMonitor(ctx context.Context) {
	log.Info().Msg("Starting task monitor to check for pending tasks")
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("Task monitor stopped due to context cancellation")
				return
			case <-wp.stopCh:
				log.Info().Msg("Task monitor stopped due to stop signal")
				return
			case <-ticker.C:
				log.Debug().Msg("Task monitor checking for pending tasks")
				if err := wp.checkForPendingTasks(ctx); err != nil {
					log.Error().Err(err).Msg("Error checking for pending tasks")
				}
			}
		}
	}()

	log.Info().Msg("Task monitor started successfully")
}

// checkForPendingTasks looks for any pending tasks and adds their jobs to the pool
func (wp *WorkerPool) checkForPendingTasks(ctx context.Context) error {
	log.Debug().Msg("Checking database for jobs with pending tasks")
	// Query for jobs with pending tasks
	rows, err := wp.db.QueryContext(ctx, `
		SELECT DISTINCT job_id FROM tasks 
		WHERE status = $1 
		LIMIT 100
	`, TaskStatusPending)

	if err != nil {
		log.Error().Err(err).Msg("Failed to query for jobs with pending tasks")
		return err
	}
	defer rows.Close()

	jobsFound := 0
	foundIDs := make([]string, 0, 100)
	// For each job with pending tasks, add it to the worker pool
	for rows.Next() {
		jobsFound++
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			log.Error().Err(err).Msg("Failed to scan job ID")
			continue
		}
		foundIDs = append(foundIDs, jobID)

		// Check if already in our active jobs
		wp.jobsMutex.RLock()
		active := wp.jobs[jobID]
		wp.jobsMutex.RUnlock()

		if !active {
			// Add job to the worker pool
			log.Info().Str("job_id", jobID).Msg("Adding job with pending tasks to worker pool")
			wp.AddJob(jobID, nil)

			// Update job status if needed
			_, err := wp.db.ExecContext(ctx, `
				UPDATE jobs SET
					status = $1,
					started_at = CASE WHEN started_at IS NULL THEN $2 ELSE started_at END
				WHERE id = $3 AND status = $4
			`, JobStatusRunning, time.Now(), jobID, JobStatusPending)

			if err != nil {
				log.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job status")
			} else {
				log.Info().Str("job_id", jobID).Msg("Updated job status to running")
			}
		} else {
			log.Debug().Str("job_id", jobID).Msg("Job already active in worker pool")
		}
	}

	if jobsFound == 0 {
		log.Debug().Msg("No jobs with pending tasks found")
	} else {
		log.Debug().Int("count", jobsFound).Msg("Found jobs with pending tasks")
	}

	foundSet := make(map[string]struct{}, len(foundIDs))
	for _, id := range foundIDs {
		foundSet[id] = struct{}{}
	}
	var toRemove []string
	wp.jobsMutex.RLock()
	for jobID := range wp.jobs {
		if _, ok := foundSet[jobID]; !ok {
			toRemove = append(toRemove, jobID)
		}
	}
	wp.jobsMutex.RUnlock()
	for _, id := range toRemove {
		log.Info().Str("job_id", id).Msg("Job has no pending tasks, removing from worker pool")
		wp.RemoveJob(id)
	}

	return rows.Err()
}

// SetJobManager sets the JobManager reference for duplicate task checking
func (wp *WorkerPool) SetJobManager(jm *JobManager) {
	wp.jobManager = jm
}

// recoverStaleTasks checks for and resets stale tasks
func (wp *WorkerPool) recoverStaleTasks(ctx context.Context) error {
	return wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		staleTime := time.Now().Add(-TaskStaleTimeout)

		// Get stale tasks
		rows, err := tx.QueryContext(ctx, `
			SELECT t.id, t.job_id, t.page_id, p.path, t.retry_count 
			FROM tasks t
			JOIN pages p ON t.page_id = p.id
			WHERE status = $1 
			AND started_at < $2
		`, TaskStatusRunning, staleTime)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var taskID, jobID string
			var pageID int
			var path string
			var retryCount int
			if err := rows.Scan(&taskID, &jobID, &pageID, &path, &retryCount); err != nil {
				continue
			}

			if retryCount >= MaxTaskRetries {
				// Mark as failed if max retries exceeded
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks 
					SET status = $1,
						error = $2,
						completed_at = $3
					WHERE id = $4
				`, TaskStatusFailed, "Max retries exceeded", time.Now(), taskID)
			} else {
				// Reset to pending for retry
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks 
					SET status = $1,
						started_at = NULL,
						retry_count = retry_count + 1
					WHERE id = $2
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

// recoverRunningJobs finds jobs that were in 'running' state when the server shut down
// and resets their 'running' tasks to 'pending', then adds them to the worker pool
func (wp *WorkerPool) recoverRunningJobs(ctx context.Context) error {
	log.Info().Msg("Recovering jobs that were running before restart")
	
	// Find jobs with 'running' status that have 'running' tasks
	rows, err := wp.db.QueryContext(ctx, `
		SELECT DISTINCT j.id
		FROM jobs j
		JOIN tasks t ON j.id = t.job_id
		WHERE j.status = $1
		AND t.status = $2
	`, JobStatusRunning, TaskStatusRunning)
	
	if err != nil {
		log.Error().Err(err).Msg("Failed to query for running jobs with running tasks")
		return err
	}
	defer rows.Close()
	
	var recoveredJobs []string
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			log.Error().Err(err).Msg("Failed to scan job ID during recovery")
			continue
		}
		
		// Reset running tasks to pending for this job
		err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx, `
				UPDATE tasks 
				SET status = $1,
					started_at = NULL,
					retry_count = retry_count + 1
				WHERE job_id = $2 
				AND status = $3
			`, TaskStatusPending, jobID, TaskStatusRunning)
			
			if err != nil {
				return err
			}
			
			rowsAffected, _ := result.RowsAffected()
			log.Info().
				Str("job_id", jobID).
				Int64("tasks_reset", rowsAffected).
				Msg("Reset running tasks to pending")
			
			return nil
		})
		
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to reset running tasks")
			continue
		}
		
		// Add job back to worker pool
		wp.AddJob(jobID, nil)
		recoveredJobs = append(recoveredJobs, jobID)
		
		log.Info().Str("job_id", jobID).Msg("Recovered running job and added to worker pool")
	}
	
	if len(recoveredJobs) > 0 {
		log.Info().
			Int("count", len(recoveredJobs)).
			Strs("job_ids", recoveredJobs).
			Msg("Successfully recovered running jobs from restart")
	} else {
		log.Debug().Msg("No running jobs found to recover")
	}
	
	return rows.Err()
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
	err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		// 1. Update all tasks in a single statement with CASE
		if len(tasks) > 0 {
			taskUpdateStart := time.Now()
			stmt, err := tx.PrepareContext(ctx, `
				UPDATE tasks
				SET status = $1, 
					completed_at = $2,
					error = $3 -- Only include error (for failure reason)
				WHERE id = $4
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
				if err := wp.CleanupStuckJobs(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to cleanup stuck jobs")
				}
			}
		}
	}()
	log.Info().Msg("Job cleanup monitor started")
}

// CleanupStuckJobs finds and fixes jobs that are stuck in pending/running state
// despite having all their tasks completed
func (wp *WorkerPool) CleanupStuckJobs(ctx context.Context) error {
	span := sentry.StartSpan(ctx, "jobs.cleanup_stuck_jobs")
	defer span.Finish()

	result, err := wp.db.ExecContext(ctx, `
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


// processTask processes an individual task
func (wp *WorkerPool) processTask(ctx context.Context, task *Task) (*crawler.CrawlResult, error) {
	// Construct a proper URL for processing
	var urlStr string
	
	// Check if path is already a full URL
	if strings.HasPrefix(task.Path, "http://") || strings.HasPrefix(task.Path, "https://") {
		urlStr = util.NormaliseURL(task.Path)
	} else if task.DomainName != "" {
		// Use centralized URL construction
		urlStr = util.ConstructURL(task.DomainName, task.Path)
	} else {
		// Fallback case - assume path is a full URL but missing protocol
		urlStr = util.NormaliseURL(task.Path)
	}
	
	log.Info().Str("url", urlStr).Str("task_id", task.ID).Msg("Starting URL warm")

	result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
	if err != nil {
		log.Error().Err(err).Str("task_id", task.ID).Msg("Crawler failed")
		return result, fmt.Errorf("crawler error: %w", err)
	}
	log.Info().
		Int("status_code", result.StatusCode).
		Str("task_id", task.ID).
		Int("links_found", len(result.Links)).
		Str("content_type", result.ContentType).
		Msg("Crawler completed")

	// Process discovered links if find_links is enabled
	if task.FindLinks && len(result.Links) > 0 {
		log.Debug().
			Str("task_id", task.ID).
			Int("links_before_filtering", len(result.Links)).
			Bool("find_links_enabled", task.FindLinks).
			Msg("Starting link filtering")

		// Filter links based on requirements:
		// 1. Only same domain/subdomain links (no external domains)
		var filtered []string

		for _, link := range result.Links {
			linkURL, err := url.Parse(link)
			if err != nil {
				log.Debug().Err(err).Str("link", link).Msg("Failed to parse discovered link URL")
				continue
			}

			// Only process links from the same domain or subdomains
			isSameDomain := isSameOrSubDomain(linkURL.Hostname(), task.DomainName)
			if !isSameDomain {
				log.Debug().
					Str("link", link).
					Str("link_hostname", linkURL.Hostname()).
					Str("job_domain", task.DomainName).
					Msg("Skipping external domain link")
				continue
			}

			// Strip anchor fragments before adding (page.html#section1 -> page.html)
			linkURL.Fragment = ""
			
			// Normalise trailing slashes (/events-news/ -> /events-news)
			if linkURL.Path != "/" && strings.HasSuffix(linkURL.Path, "/") {
				linkURL.Path = strings.TrimSuffix(linkURL.Path, "/")
			}
			
			cleanLink := linkURL.String()
			
			// At this point, it's same domain, so add it (without fragment or trailing slash)
			filtered = append(filtered, cleanLink)
			log.Debug().
				Str("original_link", link).
				Str("clean_link", cleanLink).
				Str("link_hostname", linkURL.Hostname()).
				Str("job_domain", task.DomainName).
				Msg("Added same-domain link (stripped anchors)")
		}

		log.Debug().
			Str("task_id", task.ID).
			Int("links_after_filtering", len(filtered)).
			Str("domain_name", task.DomainName).
			Msg("Link filtering completed")

		// Enqueue filtered links
		if len(filtered) > 0 {
			// Get domain ID for this job
			var domainID int
			err := wp.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
				return tx.QueryRowContext(ctx, `
					SELECT domain_id FROM jobs WHERE id = $1
				`, task.JobID).Scan(&domainID)
			})
			if err != nil {
				log.Error().
					Err(err).
					Str("job_id", task.JobID).
					Msg("Failed to get domain ID for discovered links")
				return result, nil
			}
			
			// Create page records for discovered links
			pageIDs, paths, err := db.CreatePageRecords(ctx, wp.dbQueue, domainID, filtered)
			if err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID).
					Int("link_count", len(filtered)).
					Msg("Failed to create page records for links")
				return result, nil
			}
			
			// Enqueue the filtered links with proper page IDs
			if err := wp.EnqueueURLs(
				ctx,
				task.JobID,
				pageIDs,
				paths,
				"link",     // source_type is "link" for discovered links
				urlStr,     // source_url is the URL where these links were found
			); err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID).
					Int("link_count", len(filtered)).
					Msg("Failed to enqueue discovered links")
			} else {
				log.Info().
					Str("task_id", task.ID).
					Int("link_count", len(filtered)).
					Msg("Successfully enqueued discovered links")
			}
		}
	}

	return result, nil
}

// Helper function to check if a hostname is the same domain or a subdomain of the target domain
// Handles www prefix variations (www.test.com vs test.com)
func isSameOrSubDomain(hostname, targetDomain string) bool {
	// Normalise both domains by removing www prefix
	hostname = strings.ToLower(hostname)
	targetDomain = strings.ToLower(targetDomain)
	
	// Remove www. prefix if present
	normalisedHostname := strings.TrimPrefix(hostname, "www.")
	normalisedTarget := strings.TrimPrefix(targetDomain, "www.")
	
	// Direct match (after normalization)
	if normalisedHostname == normalisedTarget {
		return true
	}
	
	// Original direct match (before normalization)
	if hostname == targetDomain {
		return true
	}

	// Check if hostname ends with .targetDomain (subdomain check)
	if strings.HasSuffix(hostname, "."+targetDomain) {
		return true
	}
	
	// Check if hostname ends with .normalisedTarget (subdomain check without www)
	if strings.HasSuffix(hostname, "."+normalisedTarget) {
		return true
	}

	return false
}

// Helper function to check if a URL points to a document
func isDocumentLink(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".pdf") ||
		strings.HasSuffix(lower, ".doc") ||
		strings.HasSuffix(lower, ".docx") ||
		strings.HasSuffix(lower, ".xls") ||
		strings.HasSuffix(lower, ".xlsx") ||
		strings.HasSuffix(lower, ".ppt") ||
		strings.HasSuffix(lower, ".pptx")
}


// listenForNotifications sets up PostgreSQL LISTEN/NOTIFY



func (wp *WorkerPool) listenForNotifications(ctx context.Context) {
	defer wp.wg.Done()

	// Define notification handler
	eventCallback := func(_ *pq.Notification) {
		// Notify workers of new tasks (non-blocking)
		select {
		case wp.notifyCh <- struct{}{}:
		default:
			// Channel already has notification pending
		}
	}

	// Configure listener with simple error handling
	listener := pq.NewListener(wp.dbConfig.ConnectionString(),
		10*time.Second, // Min reconnect interval
		time.Minute,    // Max reconnect interval
		func(ev pq.ListenerEventType, err error) {
			if err != nil {
				log.Error().Err(err).Msg("Database notification error")
			}
		})

	err := listener.Listen("new_tasks")
	if err != nil {
		log.Error().Err(err).Msg("Failed to start listening for notifications")
		return
	}

	// Ensure listener is closed when we're done
	defer listener.Close()

ListenLoop:
	for {
		select {
		case <-wp.stopCh:
			return
		case <-ctx.Done():
			return
		case n := <-listener.Notify:
			if n == nil {
				// Connection lost, break inner loop to reconnect
				log.Warn().Msg("Database connection lost")
				break ListenLoop
			}
			eventCallback(n)
		case <-time.After(90 * time.Second):
			// Check connection is alive
			if err := listener.Ping(); err != nil {
				log.Error().Err(err).Msg("Database connection lost")
				break ListenLoop
			}
		}
	}
}