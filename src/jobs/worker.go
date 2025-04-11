package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WorkerPool manages a pool of workers that process crawl tasks
type WorkerPool struct {
	db         *sql.DB
	crawler    *crawler.Crawler
	numWorkers int
	jobs       map[string]bool
	jobsMutex  sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(db *sql.DB, crawler *crawler.Crawler, numWorkers int) *WorkerPool {
	return &WorkerPool{
		db:         db,
		crawler:    crawler,
		numWorkers: numWorkers,
		jobs:       make(map[string]bool),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.numWorkers).Msg("Starting worker pool")

	wp.wg.Add(wp.numWorkers)
	for i := 0; i < wp.numWorkers; i++ {
		go wp.worker(ctx, i)
	}
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
		// Check if job is still running
		job, err := GetJob(ctx, wp.db, jobID)
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job status")
			continue
		}

		if job.Status != JobStatusRunning && job.Status != JobStatusPending {
			wp.RemoveJob(jobID) // Job is no longer active
			continue
		}

		// Try to get a pending task
		task, err := GetNextPendingTask(ctx, wp.db, jobID)
		if err != nil {
			log.Error().Err(err).Str("job_id", jobID).Msg("Failed to get pending task")
			continue
		}

		if task == nil {
			continue // No pending tasks for this job
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
	span := sentry.StartSpan(ctx, "worker.process_task")
	defer span.Finish()

	span.SetTag("worker_id", fmt.Sprintf("%d", workerID))
	span.SetTag("task_id", task.ID)
	span.SetTag("url", task.URL)

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", task.ID).
		Str("url", task.URL).
		Msg("Processing task")

	// Crawl the URL
	result, err := wp.crawler.WarmURL(ctx, task.URL)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return err
	}

	// Update task with result
	task.StatusCode = result.StatusCode
	task.ResponseTime = result.ResponseTime
	task.CacheStatus = result.CacheStatus
	task.ContentType = result.ContentType
	task.Status = TaskStatusCompleted

	// If there's an error in the result, still mark as completed but record the error
	if result.Error != "" {
		task.Error = result.Error
	}

	// Update the task in the database
	if err := UpdateTaskStatus(ctx, wp.db, task); err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return err
	}

	// Also store results in crawl_results for completed tasks
	if task.Status == TaskStatusCompleted {
		// Add to crawl_results table
		_, err = wp.db.ExecContext(ctx, `
			INSERT INTO crawl_results (url, response_time, status_code, error, cache_status)
			VALUES (?, ?, ?, ?, ?)
		`, task.URL, task.ResponseTime, task.StatusCode, task.Error, task.CacheStatus)

		if err != nil {
			log.Error().Err(err).Str("url", task.URL).Msg("Failed to store in crawl_results")
		} else {
			log.Info().Str("url", task.URL).Msg("Stored in crawl_results")
		}
	}

	// Add this new code to update job statistics
	if err := updateJobProgress(ctx, wp.db, task.JobID); err != nil {
		log.Error().Err(err).Str("job_id", task.JobID).Msg("Failed to update job progress")
	}

	// If we're supposed to find links and this is an HTML page, add them as new tasks
	job, err := GetJob(ctx, wp.db, task.JobID)
	if err != nil {
		span.SetTag("error", "true")
		span.SetData("error.message", err.Error())
		return err
	}

	if job.FindLinks && result.ContentType != "" &&
		(result.StatusCode >= 200 && result.StatusCode < 300) &&
		task.Depth < job.MaxDepth {
		// This would be where we extract links and add new tasks
		// We'll need more HTML processing code to do this properly
		// This is a placeholder for now
	}

	log.Info().
		Int("worker_id", workerID).
		Str("task_id", task.ID).
		Str("url", task.URL).
		Int("status_code", task.StatusCode).
		Int64("response_time", task.ResponseTime).
		Str("cache_status", task.CacheStatus).
		Msg("Task completed")

	return nil
}

// EnqueueURLs adds multiple URLs as tasks for a job
func EnqueueURLs(ctx context.Context, db *sql.DB, jobID string, urls []string, sourceType string, sourceURL string, depth int) error {
	span := sentry.StartSpan(ctx, "jobs.enqueue_urls")
	defer span.Finish()

	span.SetTag("job_id", jobID)
	span.SetData("url_count", len(urls))

	for _, url := range urls {
		// Create a unique ID for the task
		taskID := uuid.New().String()

		task := &Task{
			ID:         taskID,
			JobID:      jobID,
			URL:        url,
			Status:     TaskStatusPending,
			Depth:      depth,
			CreatedAt:  time.Now(),
			RetryCount: 0,
			SourceType: sourceType,
			SourceURL:  sourceURL,
		}

		if err := CreateTask(ctx, db, task); err != nil {
			span.SetTag("error", "true")
			span.SetData("error.message", err.Error())
			log.Error().Err(err).Str("url", url).Msg("Failed to create task")
			continue
		}
	}

	log.Info().
		Str("job_id", jobID).
		Int("url_count", len(urls)).
		Str("source_type", sourceType).
		Msg("Enqueued URLs for processing")

	return nil
}
