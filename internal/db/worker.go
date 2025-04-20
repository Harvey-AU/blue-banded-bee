package db

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Harvey-AU/blue-banded-bee/internal/crawler"
	"github.com/rs/zerolog/log"
)

const (
	jitterMin = 200 * time.Millisecond
	jitterMax = 800 * time.Millisecond
)

// WorkerPool manages a pool of workers that process tasks using PostgreSQL queue
type WorkerPool struct {
	queue        *DbQueue
	crawler      *crawler.Crawler
	workerCount  int
	stopCh       chan struct{}
	wg           sync.WaitGroup
	taskInterval time.Duration
	rand         *rand.Rand
	db           *sql.DB
}

// NewWorkerPool creates a new worker pool with PostgreSQL task queue
func NewWorkerPool(db *sql.DB, crawler *crawler.Crawler, workerCount int) *WorkerPool {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &WorkerPool{
		queue:        NewDbQueue(db),
		crawler:      crawler,
		workerCount:  workerCount,
		stopCh:       make(chan struct{}),
		taskInterval: 100 * time.Millisecond,
		rand:         r,
		db:           db,
	}
}

// Start begins processing tasks with the worker pool
func (wp *WorkerPool) Start(ctx context.Context) {
	log.Info().Int("workers", wp.workerCount).Msg("Starting worker pool")

	wp.wg.Add(wp.workerCount)
	for i := 0; i < wp.workerCount; i++ {
		go wp.worker(ctx, i)
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	log.Debug().Msg("Stopping worker pool")
	close(wp.stopCh)
	wp.wg.Wait()
	log.Debug().Msg("Worker pool stopped")
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

				// Random delay between 200ms and 800ms
				sleepMin := jitterMin * time.Millisecond
				sleepMax := jitterMax * time.Millisecond
				jitter := time.Duration(wp.rand.Int63n(int64(sleepMax-sleepMin))) + sleepMin
				time.Sleep(jitter)
			} else {

				time.Sleep(1000)
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
		Int("page_id", task.PageID).
		Str("path", task.Path).
		Msg("Processing task")

	// Process the task
	var domainName string
	err = wp.db.QueryRowContext(ctx, `
		SELECT d.name FROM domains d
		JOIN jobs j ON j.domain_id = d.id
		WHERE j.id = ?`, task.JobID).Scan(&domainName)
	if err != nil {
		return fmt.Errorf("failed to get domain name: %w", err)
	}

	// Construct full URL from domain and path
	fullURL := fmt.Sprintf("https://%s%s", domainName, task.Path)
	result, err := wp.crawler.WarmURL(ctx, fullURL)

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
			Str("url", fullURL).
			Msg("Task failed")

		task.Error = err.Error()
		return wp.queue.FailTask(ctx, task, err)
	}

	// Mark as completed
	return wp.queue.CompleteTask(ctx, task)
}
