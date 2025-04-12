package common

import (
	"context"
	"database/sql"
	"sync"

	"github.com/rs/zerolog/log"
)

// DbOperation represents a database operation to be executed
type DbOperation struct {
	Fn   func(*sql.Tx) error
	Done chan error
	Ctx  context.Context
}

// DbQueue serializes all database operations through a single goroutine
type DbQueue struct {
	operations chan DbOperation
	db         *sql.DB
	wg         sync.WaitGroup
	stopped    bool
	mu         sync.Mutex
}

// NewDbQueue creates and starts a new database queue
func NewDbQueue(db *sql.DB) *DbQueue {
	queue := &DbQueue{
		operations: make(chan DbOperation, 100),
		db:         db,
	}
	queue.Start()
	return queue
}

// Start begins processing operations
func (q *DbQueue) Start() {
	q.wg.Add(1)
	go q.processOperations()
}

// Stop gracefully stops the queue
func (q *DbQueue) Stop() {
	q.mu.Lock()
	if !q.stopped {
		q.stopped = true
		close(q.operations)
	}
	q.mu.Unlock()
	q.wg.Wait()
}

// processOperations handles database operations sequentially
func (q *DbQueue) processOperations() {
	defer q.wg.Done()

	for op := range q.operations {
		// Check if context is canceled
		if op.Ctx != nil && op.Ctx.Err() != nil {
			op.Done <- op.Ctx.Err()
			continue
		}

		// Start transaction
		tx, err := q.db.BeginTx(op.Ctx, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to begin transaction")
			op.Done <- err
			continue
		}

		// Execute the operation
		err = op.Fn(tx)
		if err != nil {
			tx.Rollback()
			op.Done <- err
			continue
		}

		// Commit the transaction
		err = tx.Commit()
		if err != nil {
			log.Error().Err(err).Msg("Failed to commit transaction")
		}
		op.Done <- err
	}
}

// Execute adds an operation to the queue and waits for it to complete
func (q *DbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
	done := make(chan error, 1)

	q.mu.Lock()
	stopped := q.stopped
	q.mu.Unlock()

	if stopped {
		return sql.ErrConnDone
	}

	q.operations <- DbOperation{Fn: fn, Done: done, Ctx: ctx}
	return <-done
}

// QueueProvider defines an interface for accessing a DB queue
type QueueProvider interface {
	GetQueue() *DbQueue
}
