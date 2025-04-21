package common

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// DbOperation represents a database operation to be executed
type DbOperation struct {
	Fn        func(*sql.Tx) error
	Done      chan error
	Ctx       context.Context
	StartTime time.Time
	ID        string
}

// DbQueue serializes all database operations through a single goroutine
type DbQueue struct {
	operations  chan DbOperation
	db          *sql.DB
	wg          sync.WaitGroup
	stopped     bool
	mu          sync.Mutex
	workerCount int // Number of parallel workers
}

// NewDbQueue creates and starts a new database queue
func NewDbQueue(db *sql.DB) *DbQueue {
	queue := &DbQueue{
		operations:  make(chan DbOperation, 200),
		db:          db,
		workerCount: 2,
	}
	queue.Start()
	return queue
}

// Start begins processing operations
func (q *DbQueue) Start() {
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.processOperations(i) // Pass worker ID
	}
}

// Stop gracefully stops the queue
func (q *DbQueue) Stop() {
	q.mu.Lock()
	if !q.stopped {
		q.stopped = true
		close(q.operations)
	}
	q.mu.Unlock()

	// Wait with timeout for operations to complete
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Queue stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Warn().Msg("Queue stop timed out")
	}
}

// processOperations handles database operations sequentially
func (q *DbQueue) processOperations(workerID int) {
	defer q.wg.Done()

	for op := range q.operations {
		waitDuration := time.Since(op.StartTime)
		execStart := time.Now()

		// Log when operation starts executing
		log.Debug().
			Int("worker_id", workerID).
			Str("operation_id", op.ID).
			Dur("queue_wait_ms", waitDuration).
			Time("execution_start", execStart).
			Msg("⏱️ TIMING: DB operation starting execution")

		// Check if context is canceled
		if op.Ctx != nil && op.Ctx.Err() != nil {
			op.Done <- op.Ctx.Err()
			continue
		}

		// Add retry logic for database locks
		var lastErr error
		success := false

		// We'll try up to 3 times with increasing backoff
		for attempt := 0; attempt < 3; attempt++ {
			// If this is a retry, add some backoff
			if attempt > 0 {
				backoffTime := time.Duration(100*(1<<attempt)) * time.Millisecond // Exponential backoff
				log.Warn().
					Int("worker_id", workerID).
					Str("operation_id", op.ID).
					Int("attempt", attempt+1).
					Dur("backoff", backoffTime).
					Err(lastErr).
					Msg("Retrying database operation after lock error")
				time.Sleep(backoffTime)
			}

			// Start transaction
			tx, err := q.db.BeginTx(op.Ctx, nil)
			if err != nil {
				lastErr = err
				if strings.Contains(err.Error(), "database is locked") {
					// Retry on locks
					continue
				}
				// Don't retry on other tx begin errors
				log.Error().Err(err).Msg("Failed to begin transaction")
				break
			}

			// Execute the operation
			err = op.Fn(tx)
			if err != nil {
				tx.Rollback()
				lastErr = err
				if strings.Contains(err.Error(), "database is locked") {
					// Retry on locks
					continue
				}
				// Don't retry on other execution errors
				break
			}

			// Commit the transaction
			commitStart := time.Now()
			err = tx.Commit()

			if err != nil {
				lastErr = err
				if strings.Contains(err.Error(), "database is locked") {
					// Retry on locks
					continue
				}
				// Don't retry on other commit errors
				log.Error().Err(err).Msg("Failed to commit transaction")
				break
			}

			// Success!
			execDuration := time.Since(execStart)
			commitDuration := time.Since(commitStart)
			totalDuration := time.Since(op.StartTime)

			log.Debug().
				Int("worker_id", workerID).
				Str("operation_id", op.ID).
				Dur("execution_ms", execDuration).
				Dur("commit_ms", commitDuration).
				Dur("total_ms", totalDuration).
				Bool("succeeded", true).
				Msg("⏱️ TIMING: DB operation execution completed")

			success = true
			break
		}

		// Report final result
		if success {
			op.Done <- nil
		} else {
			if lastErr != nil {
				log.Error().
					Err(lastErr).
					Int("worker_id", workerID).
					Str("operation_id", op.ID).
					Msg("Database operation failed after retries")
			}
			op.Done <- lastErr
		}
	}
}

// Execute adds an operation to the queue and waits for it to complete
func (q *DbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	if q.stopped {
		return fmt.Errorf("queue is stopped")
	}

	queueStart := time.Now()
	operationID := uuid.New().String()[:8] // Generate short unique ID

	log.Debug().
		Str("operation_id", operationID).
		Time("queue_submit", queueStart).
		Int("queue_size", len(q.operations)).
		Msg("⏱️ TIMING: DB operation submitted to queue")

	done := make(chan error, 1)
	select {
	case q.operations <- DbOperation{
		Fn:        fn,
		Done:      done,
		Ctx:       ctx,
		StartTime: queueStart,
		ID:        operationID,
	}:
		err := <-done
		queueDuration := time.Since(queueStart)

		log.Debug().
			Str("operation_id", operationID).
			Dur("queue_wait_ms", queueDuration).
			Bool("succeeded", err == nil).
			Msg("⏱️ TIMING: DB operation completed")

		return err
	case <-ctx.Done():
		log.Debug().
			Str("operation_id", operationID).
			Msg("⏱️ TIMING: DB operation cancelled before execution")
		return ctx.Err()
	}
}

// QueueProvider defines an interface for accessing a DB queue
type QueueProvider interface {
	GetQueue() *DbQueue
}
