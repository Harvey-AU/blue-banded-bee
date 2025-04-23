package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/Harvey-AU/blue-banded-bee/internal/db"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// WorkerPool manages a pool of workers that process crawl tasks
type WorkerPool struct {
	db               *sql.DB
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
func NewWorkerPool(db *sql.DB, crawler *crawler.Crawler, numWorkers int, dbConfig *db.Config) *WorkerPool {
	// Validate inputs
	if db == nil {
		panic("database connection is required")
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
		task, err := GetNextPendingTask(ctx, wp.db, jobID)
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
			// Process the task
			result, err := wp.processTask(ctx, task)
			now := time.Now()
			if err != nil {
				// mark as failed
				_, updErr := wp.db.ExecContext(ctx, `
					UPDATE tasks
					SET status = $1, completed_at = $2, error = $3
					WHERE id = $4`,
					TaskStatusFailed, now, err.Error(), task.ID)
				if updErr != nil {
					log.Error().Err(updErr).Str("task_id", task.ID).Msg("Failed to mark task as failed")
				}
			} else {
				// mark as completed with metrics
				_, updErr := wp.db.ExecContext(ctx, `
					UPDATE tasks
					SET status = $1, completed_at = $2, status_code = $3, response_time = $4, cache_status = $5, content_type = $6
					WHERE id = $7`,
					TaskStatusCompleted, now, result.StatusCode, result.ResponseTime, result.CacheStatus, result.ContentType, task.ID)
				if updErr != nil {
					log.Error().Err(updErr).Str("task_id", task.ID).Msg("Failed to mark task as completed")
				}
			}
			// update job progress via DbQueue helper
			if err := db.NewDbQueue(wp.db).UpdateJobProgress(ctx, task.JobID); err != nil {
				log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to update job progress via helper")
			}
			return nil
		}
	}

	// No tasks found in any job
	return sql.ErrNoRows
}

// EnqueueURLs adds multiple URLs as tasks for a job
func EnqueueURLs(ctx context.Context, db *sql.DB, jobID string, urls []string, sourceType string, sourceURL string, depth int) error {
	log.Debug().
		Str("job_id", jobID).
		Str("source_type", sourceType).
		Int("url_count", len(urls)).
		Int("depth", depth).
		Msg("EnqueueURLs called")

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Increase batch size for better performance
	const batchSize = 500

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
			// Verify job is still pending/running
			var status string
			if err := tx.QueryRowContext(ctx, "SELECT status FROM jobs WHERE id = $1", jobID).Scan(&status); err != nil {
				return fmt.Errorf("failed to verify job: %w", err)
			}
			if status != string(JobStatusPending) && status != string(JobStatusRunning) {
				return fmt.Errorf("job is in invalid state: %s", status)
			}
			// Increment sitemap_tasks or found_tasks and total_tasks
			column := "found_tasks"
			if sourceType == "sitemap" {
				column = "sitemap_tasks"
			}
			query := fmt.Sprintf(`UPDATE jobs SET %s = %s + $1, total_tasks = total_tasks + $1 WHERE id = $2`, column, column)
			if _, err := tx.ExecContext(ctx, query, len(batch), jobID); err != nil {
				return fmt.Errorf("failed to update task counts: %w", err)
			}

			// Get domain_id for this job
			var domainID int
			if err := tx.QueryRowContext(ctx,
				`SELECT domain_id FROM jobs WHERE id = $1`, jobID).Scan(&domainID); err != nil {
				return fmt.Errorf("failed to get domain_id for job: %w", err)
			}

			// Process each URL individually
			for _, urlStr := range batch {
				if urlStr == "" {
					continue
				}

				// Parse URL to get path
				parsedURL, err := url.Parse(urlStr)
				if err != nil {
					log.Error().Err(err).Str("url", urlStr).Msg("Failed to parse URL, skipping")
					continue
				}

				path := parsedURL.RequestURI()

				// Insert page if it doesn't exist
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO pages(domain_id, path) VALUES($1, $2) ON CONFLICT(domain_id, path) DO NOTHING`,
					domainID, path); err != nil {
					return fmt.Errorf("failed to insert page: %w", err)
				}

				// Get page_id
				var pageID int
				if err := tx.QueryRowContext(ctx,
					`SELECT id FROM pages WHERE domain_id = $1 AND path = $2`,
					domainID, path).Scan(&pageID); err != nil {
					return fmt.Errorf("failed to get page_id: %w", err)
				}

				// Create task for each URL
				if _, err := tx.ExecContext(ctx, `INSERT INTO tasks (job_id, page_id, path, status, depth, source_type, source_url) VALUES ($1, $2, $3, $4, $5, $6, $7)`, jobID, pageID, path, string(TaskStatusPending), depth, sourceType, sourceURL); err != nil {
					return fmt.Errorf("failed to insert task: %w", err)
				}
			}

			// Notify listeners of new tasks
			if _, err := tx.ExecContext(ctx, `SELECT pg_notify('new_tasks', $1)`, jobID); err != nil {
				return fmt.Errorf("failed to notify of new tasks: %w", err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to insert task batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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

// recoverStaleTasks checks for and resets stale tasks
func (wp *WorkerPool) recoverStaleTasks(ctx context.Context) error {
	return ExecuteInQueue(ctx, func(tx *sql.Tx) error {
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
	err := ExecuteInQueue(ctx, func(tx *sql.Tx) error {
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

		// 2. Batch insert all crawl results
		if len(tasks) > 0 {
			resultInsertStart := time.Now()
			completedTasks := filterTasksByStatus(tasks, TaskStatusCompleted)
			if len(completedTasks) > 0 {
				// Convert Task objects to CrawlResultData
				crawlResults := make([]CrawlResultData, 0, len(completedTasks))
				for _, task := range completedTasks {
					// Get domain name from job's domain_id
					var domainName string
					if err := tx.QueryRowContext(ctx, `
						SELECT d.name FROM domains d
						JOIN jobs j ON j.domain_id = d.id
						WHERE j.id = $1`, task.JobID).Scan(&domainName); err != nil {
						log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to get domain name")
						continue
					}

					// Construct full URL from domain and path
					fullURL := fmt.Sprintf("https://%s%s", domainName, task.Path)

					// Use the task's result data that we stored earlier
					crawlResults = append(crawlResults, CrawlResultData{
						JobID:        task.JobID,
						TaskID:       task.ID,
						URL:          fullURL,
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
				if err := CleanupStuckJobs(ctx, wp.db); err != nil {
					log.Error().Err(err).Msg("Failed to cleanup stuck jobs")
				}
			}
		}
	}()
	log.Info().Msg("Job cleanup monitor started")
}

func GetNextPendingTask(ctx context.Context, db *sql.DB, jobID string) (*Task, error) {
	query := `
		SELECT t.id, t.job_id, t.page_id, p.path, t.status, t.depth,
		t.created_at, t.retry_count, t.source_type, t.source_url, j.find_links
		FROM tasks t
		JOIN pages p ON t.page_id = p.id
		JOIN jobs j ON t.job_id = j.id
		WHERE t.status = $1 AND t.job_id = $2
		ORDER BY t.created_at ASC
		LIMIT 1
	`

	var task Task
	err := db.QueryRowContext(ctx, query, TaskStatusPending, jobID).Scan(
		&task.ID, &task.JobID, &task.PageID, &task.Path, &task.Status,
		&task.Depth, &task.CreatedAt, &task.RetryCount,
		&task.SourceType, &task.SourceURL, &task.FindLinks,
	)

	if err != nil {
		return nil, err
	}

	now := time.Now()
	_, err = db.ExecContext(ctx, `
		UPDATE tasks
		SET status = $1, started_at = $2
		WHERE id = $3
	`, TaskStatusRunning, now, task.ID)

	if err != nil {
		return nil, err
	}

	return &task, nil
}

// processTask processes an individual task
func (wp *WorkerPool) processTask(ctx context.Context, task *Task) (*crawler.CrawlResult, error) {
	// Get both domain name and findLinks setting in a single query
	var domainName string
	var findLinks bool
	
	if err := wp.db.QueryRowContext(ctx, `
		SELECT d.name, j.find_links FROM tasks t
		JOIN pages p ON t.page_id = p.id
		JOIN domains d ON p.domain_id = d.id
		JOIN jobs j ON t.job_id = j.id
		WHERE t.id = $1`, task.ID).Scan(&domainName, &findLinks); err != nil {
		return nil, fmt.Errorf("failed to get domain and settings for task %s: %w", task.ID, err)
	}
	
	urlStr := fmt.Sprintf("https://%s%s", domainName, task.Path)
	log.Info().Str("url", urlStr).Str("task_id", task.ID).Msg("Starting URL warm")
	
	// Set the FindLinks flag on the task
	task.FindLinks = findLinks
	
	// Create a job-specific crawler with the correct FindLinks setting
	crawlerConfig := *wp.crawler.Config() // Copy the base config
	crawlerConfig.FindLinks = findLinks   // Apply the job's FindLinks setting
	taskCrawler := crawler.New(&crawlerConfig)
	
	result, err := taskCrawler.WarmURL(ctx, urlStr)
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
	
	// Additional logs at the start of link filtering:
	if task.FindLinks {
		log.Debug().
			Str("task_id", task.ID).
			Int("links_before_filtering", len(result.Links)).
			Bool("find_links_enabled", findLinks).
			Msg("Starting link filtering")
		
		// Then continue with current filtering logic
		
		// After filtering:
		log.Debug().
			Str("task_id", task.ID).
			Int("links_after_filtering", len(filtered)).
			Str("domain_name", domainName).
			Msg("Link filtering completed")
	}
	
	return result, nil
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
