# Batching Architecture Design

## Analysis: SELECT vs UPDATE Batching

### GetNextTask (SELECT + UPDATE)

**Current pattern** (lines 262-335 in queue.go):

```go
// Each worker calls this independently
GetNextTask(ctx, jobID) {
    tx.QueryRowContext(`
        SELECT ... FROM tasks
        WHERE status = 'pending' AND job_id = $1
        ORDER BY priority_score DESC
        LIMIT 1
        FOR UPDATE SKIP LOCKED  // ← Row-level lock prevents conflicts
    `)
    tx.ExecContext(`UPDATE tasks SET status = 'running' WHERE id = $2`)
}
```

**Why batching SELECT won't help:**

1. `FOR UPDATE SKIP LOCKED` is DESIGNED for concurrent access
2. Each worker needs a DIFFERENT task (not the same batch)
3. Lock contention is intentional (prevents duplicate work)
4. Workers naturally spread across different jobs
5. This is the FAST PATH (no waiting, immediate claim)

**Verdict**: ❌ Don't batch GetNextTask - it's already optimal

### UpdateTaskStatus (UPDATE only)

**Current pattern** (lines 474-553 in queue.go):

```go
// Each worker calls this after task completion
UpdateTaskStatus(ctx, task) {
    tx.ExecContext(`
        UPDATE tasks
        SET status = $1, completed_at = $2, error = $3, ...
        WHERE id = $4
    `)  // ← Fires trigger
}
```

**Why batching UPDATE is critical:**

1. All workers update at roughly the same time (after crawling)
2. Each UPDATE fires the trigger individually
3. Trigger updates the SAME job row → row-level lock contention
4. 50 workers = 50 concurrent trigger executions fighting for job row lock
5. This is the BOTTLENECK causing deadlocks

**Verdict**: ✅ MUST batch UpdateTaskStatus

## Recommended Architecture: Shared Batching Infrastructure

### Core Design Principle

**One flexible batching system** that can handle different operation types:

- Update task status (immediate priority)
- Future: Batch INSERT for discovered URLs
- Future: Batch SELECT for bulk prefetching

### Architecture: Generic Batch Processor

```go
// Generic batch operation
type BatchOperation interface {
    Execute(ctx context.Context, tx *sql.Tx) error
    Merge(other BatchOperation) (BatchOperation, error) // Combine similar ops
}

// Batch manager handles all batching
type BatchManager struct {
    updateBatch   *TaskUpdateBatch
    updateTimer   *time.Ticker
    updateChannel chan *db.Task

    // Future batches
    // insertBatch *URLInsertBatch
    // selectBatch *TaskPrefetchBatch

    mu sync.Mutex
}
```

**Benefits:**

1. Single goroutine manages all batching (no coordination overhead)
2. Shared infrastructure (timers, channels, transaction logic)
3. Easy to add new batch types later
4. Consistent error handling and logging
5. Unified metrics (one place to measure batch performance)

**Drawbacks:** None significant - this is the idiomatic Go approach

### Alternative: Separate Batchers

```go
type TaskUpdateBatcher struct { ... }
type URLInsertBatcher struct { ... }
type TaskSelectBatcher struct { ... }
```

**Benefits:**

1. Isolation (one batcher failing doesn't affect others)
2. Independent tuning (different batch sizes/intervals)

**Drawbacks:**

1. Code duplication (timer logic, channel handling, transaction patterns)
2. Multiple goroutines (coordination complexity)
3. Harder to reason about overall system behaviour

**Verdict**: ❌ Over-engineered for our needs

## Recommended Implementation: Phase 1

### Focus: Task Update Batching Only

**File**: `internal/db/batch.go` (new file)

```go
package db

import (
    "context"
    "database/sql"
    "sync"
    "time"
    "github.com/rs/zerolog/log"
)

const (
    MaxBatchSize     = 100              // Flush when batch reaches this size
    MaxBatchInterval = 5 * time.Second  // Flush every N seconds regardless
    BatchChannelSize = 500              // Buffer for incoming updates
)

// TaskUpdate represents a pending task status update
type TaskUpdate struct {
    Task      *Task
    UpdatedAt time.Time
}

// BatchManager coordinates all batching operations
type BatchManager struct {
    queue   *DbQueue
    updates chan *TaskUpdate
    stopCh  chan struct{}
    wg      sync.WaitGroup
}

func NewBatchManager(queue *DbQueue) *BatchManager {
    bm := &BatchManager{
        queue:   queue,
        updates: make(chan *TaskUpdate, BatchChannelSize),
        stopCh:  make(chan struct{}),
    }

    // Start the batch processor
    bm.wg.Add(1)
    go bm.processUpdateBatches()

    return bm
}

// QueueTaskUpdate adds a task update to the batch
func (bm *BatchManager) QueueTaskUpdate(task *Task) {
    update := &TaskUpdate{
        Task:      task,
        UpdatedAt: time.Now(),
    }

    select {
    case bm.updates <- update:
        // Queued successfully
    default:
        // Channel full - this is a critical error, don't drop updates
        log.Error().
            Str("task_id", task.ID).
            Msg("Update batch channel full, blocking")
        bm.updates <- update // Block until space available
    }
}

// processUpdateBatches accumulates and flushes task updates
func (bm *BatchManager) processUpdateBatches() {
    defer bm.wg.Done()

    ticker := time.NewTicker(MaxBatchInterval)
    defer ticker.Stop()

    batch := make([]*TaskUpdate, 0, MaxBatchSize)

    flush := func() {
        if len(batch) == 0 {
            return
        }

        if err := bm.flushTaskUpdates(context.Background(), batch); err != nil {
            log.Error().
                Err(err).
                Int("batch_size", len(batch)).
                Msg("Failed to flush task update batch")
        }

        // Reset batch
        batch = batch[:0]
    }

    for {
        select {
        case update := <-bm.updates:
            batch = append(batch, update)

            // Flush if batch is full
            if len(batch) >= MaxBatchSize {
                flush()
                ticker.Reset(MaxBatchInterval)
            }

        case <-ticker.C:
            flush()

        case <-bm.stopCh:
            // Drain remaining updates
            for {
                select {
                case update := <-bm.updates:
                    batch = append(batch, update)
                default:
                    flush()
                    return
                }
            }
        }
    }
}

// flushTaskUpdates performs a true batch UPDATE using PostgreSQL
func (bm *BatchManager) flushTaskUpdates(ctx context.Context, updates []*TaskUpdate) error {
    if len(updates) == 0 {
        return nil
    }

    start := time.Now()

    err := bm.queue.Execute(ctx, func(tx *sql.Tx) error {
        // Build arrays of values for batch UPDATE
        ids := make([]string, len(updates))
        statuses := make([]string, len(updates))
        completedAts := make([]sql.NullTime, len(updates))
        errors := make([]sql.NullString, len(updates))
        responseTimes := make([]sql.NullInt64, len(updates))
        cacheStatuses := make([]sql.NullString, len(updates))

        for i, update := range updates {
            task := update.Task
            ids[i] = task.ID
            statuses[i] = task.Status

            if !task.CompletedAt.IsZero() {
                completedAts[i] = sql.NullTime{Time: task.CompletedAt, Valid: true}
            }

            if task.Error != "" {
                errors[i] = sql.NullString{String: task.Error, Valid: true}
            }

            if task.ResponseTime > 0 {
                responseTimes[i] = sql.NullInt64{Int64: task.ResponseTime, Valid: true}
            }

            if task.CacheStatus != "" {
                cacheStatuses[i] = sql.NullString{String: task.CacheStatus, Valid: true}
            }
        }

        // Use PostgreSQL's UPDATE FROM with unnest to batch update all tasks
        // This executes as a SINGLE UPDATE statement, firing the trigger ONCE
        query := `
            UPDATE tasks
            SET
                status = updates.status,
                completed_at = updates.completed_at,
                error = updates.error,
                response_time = updates.response_time,
                cache_status = updates.cache_status
            FROM (
                SELECT
                    unnest($1::text[]) AS id,
                    unnest($2::text[]) AS status,
                    unnest($3::timestamptz[]) AS completed_at,
                    unnest($4::text[]) AS error,
                    unnest($5::bigint[]) AS response_time,
                    unnest($6::text[]) AS cache_status
            ) AS updates
            WHERE tasks.id = updates.id
        `

        result, err := tx.ExecContext(ctx, query,
            pq.Array(ids),
            pq.Array(statuses),
            pq.Array(completedAts),
            pq.Array(errors),
            pq.Array(responseTimes),
            pq.Array(cacheStatuses),
        )

        if err != nil {
            return fmt.Errorf("batch update failed: %w", err)
        }

        rowsAffected, _ := result.RowsAffected()

        log.Info().
            Int("batch_size", len(updates)).
            Int64("rows_updated", rowsAffected).
            Dur("duration_ms", time.Since(start)).
            Msg("Batch update completed")

        return nil
    })

    return err
}

func (bm *BatchManager) Stop() {
    close(bm.stopCh)
    bm.wg.Wait()
}
```

### Integration with WorkerPool

**File**: `internal/jobs/worker.go`

```go
type WorkerPool struct {
    db           *sql.DB
    dbQueue      DbQueueInterface
    crawler      CrawlerInterface
    batchManager *db.BatchManager  // ← New field
    // ... existing fields
}

func NewWorkerPool(...) *WorkerPool {
    wp := &WorkerPool{
        // ... existing initialization
        batchManager: db.NewBatchManager(dbQueue),
    }
    return wp
}

func (wp *WorkerPool) Stop() {
    // ... existing stop logic
    wp.batchManager.Stop()  // ← Flush remaining batches
}

// In handleTaskError (line 1533)
func (wp *WorkerPool) handleTaskError(...) {
    // ... existing error handling

    // OLD: updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
    // NEW:
    wp.batchManager.QueueTaskUpdate(task)
}

// In handleTaskSuccess (line 1622)
func (wp *WorkerPool) handleTaskSuccess(...) {
    // ... existing success handling

    // OLD: updErr := wp.dbQueue.UpdateTaskStatus(ctx, task)
    // NEW:
    wp.batchManager.QueueTaskUpdate(task)
}
```

## Phase 2 Considerations (Future)

### When to Add Batch SELECT (Prefetching)

**Consider if:**

1. Workers spend significant time waiting for GetNextTask
2. Database shows SELECT contention in pg_stat_activity
3. Task claim rate becomes bottleneck

**Implementation approach:**

```go
// Prefetch N tasks, distribute to workers via channel
type TaskPrefetchBatch struct {
    taskQueue chan *Task
    ...
}

// Worker pulls from prefetch queue instead of calling GetNextTask
task := <-wp.batchManager.PrefetchQueue()
```

**Complexity**: Medium (requires coordinator pattern)

### When to Add Batch INSERT (URL Discovery)

**Consider if:**

1. Link discovery creates INSERT storms
2. Many tasks discovering links simultaneously
3. Database shows INSERT contention

**Implementation approach:**

```go
// Queue discovered URLs, batch INSERT every N seconds
bm.QueueURLInsert(jobID, urls)
```

**Complexity**: Low (similar to update batching)

## Metrics to Track

After implementing batch updates:

1. **Batch efficiency**
   - Average batch size (target: 40-60)
   - Batch flush rate (target: 6-12 per minute)
   - Channel fullness (should stay <80%)

2. **Database impact**
   - Transaction count (expect 95% reduction)
   - Trigger executions (expect 95% reduction)
   - Rollback rate (target: <1%)

3. **Latency**
   - Update latency P50/P95/P99 (expect 2-5 seconds)
   - Job completion accuracy (should be 100%)

## Risk Assessment

**Low Risk** ✅

- Channel-based design is standard Go pattern
- Batch flushing is transactional (all-or-nothing)
- Graceful shutdown ensures no data loss
- Easy rollback (remove QueueTaskUpdate calls)

**Potential Issues:**

1. **Memory pressure** if channel fills
   - Mitigation: Monitor channel size, alert at 80%

2. **Latency concerns** for real-time progress
   - Mitigation: 5s max delay is acceptable (dashboard polls every 60s)

3. **Batch transaction timeout**
   - Mitigation: MaxBatchSize prevents oversized batches

## Summary

**Recommended approach:**

1. **Shared BatchManager** with focus on UPDATE batching
2. **Generic infrastructure** ready for future batch types
3. **Phase 1: Task updates only** (high impact, low risk)
4. **Phase 2: Add SELECT/INSERT batching** only if metrics indicate need

This gives us the 95% transaction reduction we need while keeping the
architecture clean and extensible.
