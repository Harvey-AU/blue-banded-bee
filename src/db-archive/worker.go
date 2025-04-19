package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/src/crawler"
	"github.com/rs/zerolog/log"
)

// WorkerPool manages a pool of workers that process tasks using PostgreSQL queue
type WorkerPool struct {
	queue        *TaskQueue
	crawler      *crawler.Crawler
	workerCount  int
	stopCh       chan struct{}
	wg           sync.WaitGroup
	taskInterval time.Duration
}

// NewWorkerPool creates a new worker pool with PostgreSQL task queue
func NewWorkerPool(db *DB, crawler *crawler.Crawler, workerCount int) *WorkerPool {
	return &WorkerPool{
		queue:        NewTaskQueue(db.client),
		crawler:      crawler,
		workerCount:  workerCount,
		stopCh:       make(chan struct{}),
		taskInterval: 100 * time.Millisecond,
	}
}

// Start begins processing tasks with the worker pool
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.workerCount).Msg("Starting PostgreSQL worker pool")

	wp.wg.Add(wp.workerCount)
	for i := 0; i < wp.workerCount; i++ {
		go wp.worker(ctx, i)
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	log.Debug().Msg("Stopping PostgreSQL worker pool")
	close(wp.stopCh)
	wp.wg.Wait()
	log.Debug().Msg("PostgreSQL worker pool stopped")
}

// worker continuously processes tasks
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	defer wp.wg.Done()

	log.Debug().Int("worker_id", workerID).Msg("Starting worker")

	for {
		select {
		case <-wp.stopCh:
			log.Debug().Int("worker_id", workerID).Msg("Worker received stop signal")
			return
		case <-ctx.Done():
			log.Debug().Int("worker_id", workerID).Msg("Worker context cancelled")
			return
		default:
			// Try to get and process a task
			if err := wp.processNextTask(ctx, workerID); err != nil {
				log.Error().
					Err(err).
					Int("worker_id", workerID).
					Msg("Error processing task")

				// Sleep to avoid hammering the database on errors
				time.Sleep(wp.taskInterval * 10)
			} else {
				// If no task available or successful, sleep briefly to prevent CPU spinning
				time.Sleep(wp.taskInterval)
			}
		}
	}
}

// processNextTask gets and processes a single task
func (wp *WorkerPool) processNextTask(ctx context.Context, workerID int) error {
	// Get the next available task
	task, err := wp.queue.GetNextTask(ctx)
	if err != nil {
		return err
	}

	// No task available
	if task == nil {
		return nil
	}

	log.Debug().
		Int("worker_id", workerID).
		Str("task_id", task.ID).
		Str("url", task.URL).
		Msg("Processing task")

	// Process the task
	result, err := wp.crawler.WarmURL(ctx, task.URL)

	// Record result data
	if result != nil {
		task.StatusCode = result.StatusCode
		task.ResponseTime = result.ResponseTime
		task.CacheStatus = result.CacheStatus
		task.ContentType = result.ContentType
	}

	// Handle task completion
	if err != nil {
		log.Error().
			Err(err).
			Int("worker_id", workerID).
			Str("task_id", task.ID).
			Str("url", task.URL).
			Msg("Task failed")

		task.Error = err.Error()
		return wp.queue.FailTask(ctx, task, err)
	}

	// Mark as completed
	return wp.queue.CompleteTask(ctx, task)
}
