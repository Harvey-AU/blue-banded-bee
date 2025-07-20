package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// JobPerformance tracks performance metrics for a specific job
type JobPerformance struct {
	RecentTasks  []int64   // Last 5 task response times for this job
	CurrentBoost int       // Current performance boost workers for this job
	LastCheck    time.Time // When we last evaluated this job
}

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
	workersMutex     sync.RWMutex
	taskBatch        *TaskBatch
	batchTimer       *time.Ticker
	cleanupInterval  time.Duration
	notifyCh         chan struct{}
	jobManager       *JobManager // Reference to JobManager for duplicate checking
	
	// Performance scaling
	jobPerformance map[string]*JobPerformance
	perfMutex      sync.RWMutex
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

		stopCh:           make(chan struct{}),
		notifyCh:         make(chan struct{}, 1), // Buffer of 1 to prevent blocking
		recoveryInterval: 1 * time.Minute,
		taskBatch: &TaskBatch{
			tasks:     make([]*Task, 0, 50),
			jobCounts: make(map[string]struct{ completed, failed int }),
		},
		batchTimer:      time.NewTicker(10 * time.Second),
		cleanupInterval: time.Minute,
		
		// Performance scaling
		jobPerformance: make(map[string]*JobPerformance),
	}

	// Start the batch processor
	wp.wg.Go(func() {
		wp.processBatches(context.Background())
	})

	// Start the notification listener
	wp.wg.Go(func() {
		wp.listenForNotifications(context.Background())
	})

	return wp
}

func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.numWorkers).Msg("Starting worker pool")

	for i := 0; i < wp.numWorkers; i++ {
		i := i
		wp.wg.Go(func() {
			wp.worker(ctx, i)
		})
	}

	// Start the recovery monitor
	wp.wg.Go(func() {
		wp.recoveryMonitor(ctx)
	})

	// Run initial cleanup
	if err := wp.CleanupStuckJobs(ctx); err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to perform initial job cleanup")
	}

	// Recover jobs that were running before restart
	if err := wp.recoverRunningJobs(ctx); err != nil {
		sentry.CaptureException(err)
		log.Error().Err(err).Msg("Failed to recover running jobs on startup")
	}

	wp.StartTaskMonitor(ctx)
	wp.StartCleanupMonitor(ctx)
}

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

func (wp *WorkerPool) AddJob(jobID string, options *JobOptions) {
	wp.jobsMutex.Lock()
	wp.jobs[jobID] = true
	wp.jobsMutex.Unlock()

	// Initialise performance tracking for this job
	wp.perfMutex.Lock()
	wp.jobPerformance[jobID] = &JobPerformance{
		RecentTasks:  make([]int64, 0, 5),
		CurrentBoost: 0,
		LastCheck:    time.Now(),
	}
	wp.perfMutex.Unlock()

	// Simple scaling: add 5 workers per job, maximum of 50 total
	wp.workersMutex.Lock()
	targetWorkers := min(wp.currentWorkers + 5, 50)
	
	if targetWorkers > wp.currentWorkers {
		wp.workersMutex.Unlock()
		wp.scaleWorkers(context.Background(), targetWorkers)
	} else {
		wp.workersMutex.Unlock()
	}

	log.Debug().
		Str("job_id", jobID).
		Int("current_workers", wp.currentWorkers).
		Int("target_workers", targetWorkers).
		Msg("Added job to worker pool")
}

func (wp *WorkerPool) RemoveJob(jobID string) {
	wp.jobsMutex.Lock()
	delete(wp.jobs, jobID)
	wp.jobsMutex.Unlock()

	// Remove performance boost for this job
	wp.perfMutex.Lock()
	var jobBoost int
	if perf, exists := wp.jobPerformance[jobID]; exists {
		jobBoost = perf.CurrentBoost
		delete(wp.jobPerformance, jobID)
	}
	wp.perfMutex.Unlock()

	// Simple scaling: remove 5 workers per job + any performance boost, minimum of base count
	wp.workersMutex.Lock()
	targetWorkers := max(wp.currentWorkers - 5 - jobBoost, wp.baseWorkerCount)

	log.Debug().
		Str("job_id", jobID).
		Int("current_workers", wp.currentWorkers).
		Int("target_workers", targetWorkers).
		Int("job_boost_removed", jobBoost).
		Msg("Scaling down worker pool")

	wp.currentWorkers = targetWorkers
	// Note: We don't actually stop excess workers, they'll exit on next task completion
	wp.workersMutex.Unlock()

	log.Debug().
		Str("job_id", jobID).
		Msg("Removed job from worker pool")
}

func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	log.Info().Int("worker_id", workerID).Msg("Starting worker")

	// Track consecutive no-task counts for backoff
	consecutiveNoTasks := 0
	maxSleep := 5 * time.Second // Note: Changed from 30 to 5 seconds, to increase resonsiveness when inactive.
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
					sleepTime := min(time.Duration(float64(baseSleep) * math.Pow(1.5, float64(min(consecutiveNoTasks, 10)))), maxSleep)

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
				Float64("priority", task.PriorityScore).
				Msg("Found and claimed pending task")

			// Convert db.Task to jobs.Task for processing
			jobsTask := &Task{
				ID:            task.ID,
				JobID:         task.JobID,
				PageID:        task.PageID,
				Path:          task.Path,
				Status:        TaskStatus(task.Status),
				CreatedAt:     task.CreatedAt,
				StartedAt:     task.StartedAt, 
				RetryCount:    task.RetryCount,
				SourceType:    task.SourceType,
				SourceURL:     task.SourceURL,
				PriorityScore: task.PriorityScore,
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
				// Check if this is a blocking error (403/429)
				if isBlockingError(err) {
					// For blocking errors, only retry twice (less aggressive)
					if task.RetryCount < 2 {
						task.RetryCount++
						task.Status = string(TaskStatusPending)
						task.StartedAt = time.Time{} // Reset started time
						log.Warn().
							Err(err).
							Str("task_id", task.ID).
							Int("retry_count", task.RetryCount).
							Msg("Task blocked (403/429), limited retry scheduled")
					} else {
						// Mark as permanently failed after 2 retries
						task.Status = string(TaskStatusFailed)
						task.CompletedAt = now
						task.Error = err.Error()
						log.Error().
							Err(err).
							Str("task_id", task.ID).
							Int("retry_count", task.RetryCount).
							Msg("Task blocked permanently after 2 retries")
					}
				} else if isRetryableError(err) && task.RetryCount < MaxTaskRetries {
					// For other retryable errors, use normal retry limit
					task.RetryCount++
					task.Status = string(TaskStatusPending)
					task.StartedAt = time.Time{} // Reset started time
					log.Warn().
						Err(err).
						Str("task_id", task.ID).
						Int("retry_count", task.RetryCount).
						Msg("Task failed with retryable error, scheduling retry")
				} else {
					// Mark as permanently failed
					task.Status = string(TaskStatusFailed)
					task.CompletedAt = now
					task.Error = err.Error()
					log.Error().
						Err(err).
						Str("task_id", task.ID).
						Int("retry_count", task.RetryCount).
						Msg("Task failed permanently")
				}
				updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
				if updErr != nil {
					sentry.CaptureException(updErr)
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
				task.ContentLength = result.ContentLength
				task.RedirectURL = result.RedirectURL

				// Performance metrics
				task.DNSLookupTime = result.Performance.DNSLookupTime
				task.TCPConnectionTime = result.Performance.TCPConnectionTime
				task.TLSHandshakeTime = result.Performance.TLSHandshakeTime
				task.TTFB = result.Performance.TTFB
				task.ContentTransferTime = result.Performance.ContentTransferTime

				// Second request metrics
				task.SecondResponseTime = result.SecondResponseTime
				task.SecondCacheStatus = result.SecondCacheStatus
				if result.SecondPerformance != nil {
					task.SecondContentLength = result.SecondContentLength
					task.SecondDNSLookupTime = result.SecondPerformance.DNSLookupTime
					task.SecondTCPConnectionTime = result.SecondPerformance.TCPConnectionTime
					task.SecondTLSHandshakeTime = result.SecondPerformance.TLSHandshakeTime
					task.SecondTTFB = result.SecondPerformance.TTFB
					task.SecondContentTransferTime = result.SecondPerformance.ContentTransferTime
				}

				// Marshal JSONB fields
				var err error
				task.Headers, err = json.Marshal(result.Headers)
				if err != nil {
					log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to marshal headers")
				}
				if result.SecondHeaders != nil {
					task.SecondHeaders, err = json.Marshal(result.SecondHeaders)
					if err != nil {
						log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to marshal second headers")
					}
				}
				if result.CacheCheckAttempts != nil {
					task.CacheCheckAttempts, err = json.Marshal(result.CacheCheckAttempts)
					if err != nil {
						log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to marshal cache check attempts")
					}
				}

				updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
				if updErr != nil {
					sentry.CaptureException(updErr)
					log.Error().Err(updErr).Str("task_id", task.ID).Msg("Failed to mark task as completed")
				}

				// Evaluate job performance for scaling
				if result.ResponseTime > 0 {
					wp.evaluateJobPerformance(task.JobID, result.ResponseTime)
				}
			}
			return nil
		}
	}

	// No tasks found in any job
	return sql.ErrNoRows
}

// EnqueueURLs is a wrapper that ensures all task enqueuing goes through the JobManager.
// This allows for centralised logic, such as duplicate checking, to be applied.
func (wp *WorkerPool) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
	log.Debug().
		Str("job_id", jobID).
		Str("source_type", sourceType).
		Int("url_count", len(pages)).
		Msg("EnqueueURLs called via WorkerPool, passing to JobManager")

	// The jobManager must be set for the worker pool to function correctly.
	if wp.jobManager == nil {
		panic("jobManager is not set on WorkerPool")
	}

	return wp.jobManager.EnqueueJobURLs(ctx, jobID, pages, sourceType, sourceURL)
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
		wp.wg.Go(func() {
			wp.worker(ctx, workerID)
		})
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
	wp.wg.Go(func() {
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
	})
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
			Int("total_links_found", len(result.Links["header"])+len(result.Links["footer"])+len(result.Links["body"])).
			Bool("find_links_enabled", task.FindLinks).
			Msg("Starting link processing and priority assignment")

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

		isHomepage := task.Path == "/"

		processLinkCategory := func(links []string, priority float64) {
			if len(links) == 0 {
				return
			}

			// 1. Filter links for same-domain
			var filtered []string
			for _, link := range links {
				linkURL, err := url.Parse(link)
				if err != nil {
					continue
				}
				if isSameOrSubDomain(linkURL.Hostname(), task.DomainName) {
					linkURL.Fragment = ""
					if linkURL.Path != "/" && strings.HasSuffix(linkURL.Path, "/") {
						linkURL.Path = strings.TrimSuffix(linkURL.Path, "/")
					}
					filtered = append(filtered, linkURL.String())
				}
			}

			if len(filtered) == 0 {
				return
			}

			// 2. Create page records
			pageIDs, paths, err := db.CreatePageRecords(ctx, wp.dbQueue, domainID, task.DomainName, filtered)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create page records for links")
				return
			}

			// 3. Create a slice of db.Page for enqueuing
			pagesToEnqueue := make([]db.Page, len(pageIDs))
			for i := range pageIDs {
				pagesToEnqueue[i] = db.Page{
					ID:   pageIDs[i],
					Path: paths[i],
					// Priority will be set by the caller of processLinkCategory
				}
			}

			// 4. Enqueue new tasks
			if err := wp.EnqueueURLs(ctx, task.JobID, pagesToEnqueue, "link", urlStr); err != nil {
				log.Error().Err(err).Msg("Failed to enqueue discovered links")
				return // Stop if enqueuing fails
			}

			// 5. Update priorities for the newly created tasks
			if err := wp.updateTaskPriorities(ctx, task.JobID, domainID, priority, paths); err != nil {
				log.Error().Err(err).Msg("Failed to update task priorities for discovered links")
			}
		}

		// Apply priorities based on page type and link category
		if isHomepage {
			log.Debug().Str("task_id", task.ID).Msg("Processing links from HOMEPAGE")
			processLinkCategory(result.Links["header"], 1.000)
			processLinkCategory(result.Links["footer"], 0.990)
			processLinkCategory(result.Links["body"], task.PriorityScore*0.9) // Children of homepage
		} else {
			log.Debug().Str("task_id", task.ID).Msg("Processing links from regular page")
			// For all other pages, only process body links
			processLinkCategory(result.Links["body"], task.PriorityScore*0.9) // Children of other pages
		}
	}

	return result, nil
}

// isRetryableError checks if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	
	// Network/timeout errors that should be retried
	networkErrors := strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		strings.Contains(errorStr, "connection") ||
		strings.Contains(errorStr, "network") ||
		strings.Contains(errorStr, "temporary") ||
		strings.Contains(errorStr, "reset by peer") ||
		strings.Contains(errorStr, "broken pipe") ||
		strings.Contains(errorStr, "unexpected eof")
	
	// Server errors that should be retried (likely due to load/temporary issues)
	serverErrors := strings.Contains(errorStr, "internal server error") ||
		strings.Contains(errorStr, "bad gateway") ||
		strings.Contains(errorStr, "service unavailable") ||
		strings.Contains(errorStr, "gateway timeout") ||
		strings.Contains(errorStr, "502") ||
		strings.Contains(errorStr, "503") ||
		strings.Contains(errorStr, "504") ||
		strings.Contains(errorStr, "500")
	
	return networkErrors || serverErrors
}

// isBlockingError checks if an error indicates we're being blocked
func isBlockingError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	
	// Blocking/rate limit errors that need special handling
	return strings.Contains(errorStr, "403") ||
		strings.Contains(errorStr, "forbidden") ||
		strings.Contains(errorStr, "429") ||
		strings.Contains(errorStr, "too many requests") ||
		strings.Contains(errorStr, "rate limit")
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

// updateTaskPriorities updates the priority scores for tasks of linked pages
func (wp *WorkerPool) updateTaskPriorities(ctx context.Context, jobID string, domainID int, newPriority float64, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// Update all task priorities in a single query
	result, err := wp.db.ExecContext(ctx, `
		UPDATE tasks t
		SET priority_score = $1
		FROM pages p
		WHERE t.page_id = p.id
		AND t.job_id = $2
		AND p.domain_id = $3
		AND p.path = ANY($4)
		AND t.priority_score < $1
	`, newPriority, jobID, domainID, paths)

	if err != nil {
		return fmt.Errorf("failed to update task priorities: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Info().
			Str("job_id", jobID).
			Int64("tasks_updated", rowsAffected).
			Float64("new_priority", newPriority).
			Msg("Updated task priorities for discovered links")
	}

	return nil
}

// evaluateJobPerformance checks if a job needs performance scaling
func (wp *WorkerPool) evaluateJobPerformance(jobID string, responseTime int64) {
	wp.perfMutex.Lock()
	defer wp.perfMutex.Unlock()

	perf, exists := wp.jobPerformance[jobID]
	if !exists {
		return // Job not tracked
	}

	// Add response time to recent tasks (sliding window of 5)
	perf.RecentTasks = append(perf.RecentTasks, responseTime)
	if len(perf.RecentTasks) > 5 {
		perf.RecentTasks = perf.RecentTasks[1:] // Remove oldest
	}

	// Only evaluate after we have at least 3 tasks
	if len(perf.RecentTasks) < 3 {
		return
	}

	// Calculate average response time
	var total int64
	for _, rt := range perf.RecentTasks {
		total += rt
	}
	avgResponseTime := total / int64(len(perf.RecentTasks))

	// Determine needed boost workers based on performance tiers
	var neededBoost int
	switch {
	case avgResponseTime >= 4000: // 4000ms+
		neededBoost = 20
	case avgResponseTime >= 3000: // 3000-4000ms
		neededBoost = 15
	case avgResponseTime >= 2000: // 2000-3000ms
		neededBoost = 10
	case avgResponseTime >= 1000: // 1000-2000ms
		neededBoost = 5
	default: // 0-1000ms
		neededBoost = 0
	}

	// Check if boost needs to change
	if neededBoost != perf.CurrentBoost {
		boostDiff := neededBoost - perf.CurrentBoost
		
		log.Info().
			Str("job_id", jobID).
			Int64("avg_response_time", avgResponseTime).
			Int("old_boost", perf.CurrentBoost).
			Int("new_boost", neededBoost).
			Int("boost_diff", boostDiff).
			Msg("Job performance scaling triggered")

		// Update current boost
		perf.CurrentBoost = neededBoost
		perf.LastCheck = time.Now()

		// Apply scaling to worker pool
		if boostDiff > 0 {
			// Need more workers
			wp.workersMutex.Lock()
			targetWorkers := wp.currentWorkers + boostDiff
			if targetWorkers > 50 { // Respect global max
				targetWorkers = 50
				perf.CurrentBoost = perf.CurrentBoost - (wp.currentWorkers + boostDiff - 50) // Adjust boost to actual
			}
			wp.workersMutex.Unlock()
			
			if targetWorkers > wp.currentWorkers {
				go wp.scaleWorkers(context.Background(), targetWorkers)
			}
		}
		// Note: For scaling down (boostDiff < 0), we let workers naturally exit
		// when they check shouldExit in the worker loop
	}
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

	var conn *pgx.Conn
	var err error

	connect := func() (*pgx.Conn, error) {
		c, err := pgx.Connect(ctx, wp.dbConfig.ConnectionString())
		if err != nil {
			return nil, err
		}
		_, err = c.Exec(ctx, "LISTEN new_tasks")
		if err != nil {
			c.Close(ctx)
			return nil, err
		}
		return c, nil
	}

	conn, err = connect()
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect for notifications initially")
		return
	}
	defer conn.Close(ctx)

	for {
		select {
		case <-wp.stopCh:
			log.Debug().Msg("Notification listener received stop signal.")
			return
		case <-ctx.Done():
			log.Debug().Msg("Notification listener context cancelled.")
			return
		default:
			// Non-blocking check for stop signal before waiting for notification
		}

		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil || wp.stopping.Load() {
				return // Context cancelled or pool is stopping
			}
			log.Error().Err(err).Msg("Error waiting for notification, reconnecting...")
			conn.Close(ctx)
			time.Sleep(5 * time.Second) // Wait before reconnecting

			conn, err = connect()
			if err != nil {
				log.Error().Err(err).Msg("Failed to reconnect for notifications")
				continue
			}
			continue
		}

		log.Debug().Str("channel", notification.Channel).Msg("Received database notification")
		// Notify workers of new tasks (non-blocking)
		select {
		case wp.notifyCh <- struct{}{}:
		default:
			// Channel already has notification pending
		}
	}
}
