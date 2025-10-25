# Throughput Optimisation Plan

**Date:** 2025-10-25 **Status:** Planning **Goal:** Increase throughput from 2-3
tasks/sec to 124-186 tasks/sec (2-3 tasks/sec per job across 62 concurrent jobs)

## Current Bottleneck Analysis

### Root Cause

Workers process tasks synchronously, blocking on HTTP requests (1-2s each). With
50 workers and 62 concurrent jobs:

- Each worker handles 1 task at a time
- Workers idle during network I/O
- Round-robin claiming gives each job ~0.8 workers on average
- Result: 2-3 tasks/sec total instead of 124-186 tasks/sec

### Key Code Locations

- [worker.go:333-393](../internal/jobs/worker.go#L333-L393) - Worker loop
  (synchronous processing)
- [worker.go:492-539](../internal/jobs/worker.go#L492-L539) -
  `processNextTask()` (blocks on HTTP)
- [worker.go:1522-1572](../internal/jobs/worker.go#L1522-L1572) -
  `processTask()` (calls crawler, blocks 1-2s)
- [worker.go:395-434](../internal/jobs/worker.go#L395-L434) -
  `claimPendingTask()` (round-robin across jobs)
- [queue.go:264-331](../internal/db/queue.go#L264-L331) - `GetNextTask()` (task
  claiming with job filter)

### Job Concurrency Field

The `jobs.concurrency` database field exists (currently set to 3) but is **not
enforced** in worker allocation. See:

- [types.go:56](../internal/jobs/types.go#L56) - `Concurrency int` field
  definition
- [scripts/load-test-simple.sh:21](../scripts/load-test-simple.sh#L21) - Jobs
  created with `"concurrency": 3`

## Recommended Solution: Option 2 + Option 3

Implement **concurrent task processing per worker** (Option 2) combined with
**job concurrency enforcement** (Option 3).

### Benefits

- Most RAM-efficient approach
- Respects existing `job.concurrency` settings
- 50-100 workers can handle 300-600 concurrent HTTP requests
- Each job guaranteed minimum concurrency level

### Target Architecture

- 50-100 workers (configurable per environment)
- Each worker processes multiple tasks concurrently (5-10 goroutines)
- Job concurrency setting enforced: each job gets `job.concurrency` concurrent
  tasks
- Worker allocation: ensure jobs with pending tasks get their concurrency quota

## Implementation Tasks

### Phase 1: Add Concurrent Task Processing per Worker

**Goal:** Allow each worker to process multiple tasks concurrently using
goroutines.

#### Task 1.1: Refactor Worker Loop for Concurrency

**File:**
[internal/jobs/worker.go:333-393](../internal/jobs/worker.go#L333-L393)

**Current Behaviour:**

```go
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
    for {
        // Claim one task
        if err := wp.processNextTask(ctx); err != nil {
            // Handle error, backoff
        }
        time.Sleep(baseSleep)  // Wait between tasks
    }
}
```

**Target Behaviour:**

```go
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
    // Semaphore to limit concurrent tasks per worker
    sem := make(chan struct{}, workerConcurrency)
    var wg sync.WaitGroup

    for {
        // Claim task
        task := claimTask()

        // Launch goroutine to process task
        sem <- struct{}{}  // Acquire
        wg.Add(1)
        go func(t *Task) {
            defer wg.Done()
            defer func() { <-sem }()  // Release
            wp.processTask(ctx, t)
        }(task)
    }
}
```

**Implementation Steps:**

1. Add `workerConcurrency` config field to `WorkerPool` (default: 5-10)
2. Modify `worker()` function to use goroutines with semaphore
3. Add `sync.WaitGroup` for graceful shutdown
4. Handle task claiming separately from task processing
5. Test with increasing concurrency levels (2, 5, 10)

**Success Criteria:**

- Single worker processes multiple tasks concurrently
- HTTP requests don't block other task processing
- Graceful shutdown waits for in-flight tasks

---

#### Task 1.2: Separate Task Claiming from Task Processing

**File:**
[internal/jobs/worker.go:492-539](../internal/jobs/worker.go#L492-L539)

**Current Behaviour:** `processNextTask()` combines claiming and processing
(blocking).

**Target Behaviour:**

- `claimNextTask()` - Non-blocking, returns task or nil
- `processClaimedTask()` - Async processing in goroutine

**Implementation Steps:**

1. Split `processNextTask()` into `claimNextTask()` and `processClaimedTask()`
2. Move task preparation logic to separate function
3. Update worker loop to claim tasks in main thread, process in goroutines
4. Add error handling for concurrent task processing

**Success Criteria:**

- Task claiming is fast (<50ms)
- Task processing happens asynchronously
- Error handling doesn't block claiming

---

#### Task 1.3: Add Worker Concurrency Configuration

**File:** [cmd/app/main.go:137-149](../cmd/app/main.go#L137-L149)

**Current Configuration:**

```go
var jobWorkers int
switch appEnv {
case "production":
    jobWorkers = 50
case "staging":
    jobWorkers = 10
default:
    jobWorkers = 5
}
```

**Target Configuration:**

```go
var jobWorkers int
var workerConcurrency int

switch appEnv {
case "production":
    jobWorkers = 100          // More workers
    workerConcurrency = 5     // 5 concurrent tasks per worker = 500 total
case "staging":
    jobWorkers = 20
    workerConcurrency = 3     // 60 concurrent tasks
default:
    jobWorkers = 5
    workerConcurrency = 2     // 10 concurrent tasks
}
```

**Implementation Steps:**

1. Add `workerConcurrency` parameter to `NewWorkerPool()`
2. Add environment variable `WORKER_CONCURRENCY` for override
3. Calculate and log total concurrency on startup
4. Update worker pool initialisation

**Success Criteria:**

- Production achieves 500+ concurrent task capacity
- Configuration is environment-aware
- Logs show total concurrency clearly

---

### Phase 2: Enforce Job Concurrency Limits

**Goal:** Respect the `job.concurrency` database field to ensure each job gets
fair worker allocation.

#### Task 2.1: Add Job Concurrency Tracking

**File:**
[internal/jobs/worker.go:200-299](../internal/jobs/worker.go#L200-L299)

**Current Job Cache:**

```go
type jobInfo struct {
    DomainName string
    FindLinks  bool
    CrawlDelay int
}
wp.jobInfoCache map[string]*jobInfo
```

**Target Job Cache:**

```go
type jobInfo struct {
    DomainName      string
    FindLinks       bool
    CrawlDelay      int
    Concurrency     int           // From database
    RunningTasks    int           // Current count
    runningMutex    sync.RWMutex  // Protect counter
}
```

**Implementation Steps:**

1. Add `Concurrency` field to `jobInfo` struct
2. Add `RunningTasks` counter and mutex
3. Load `concurrency` from database in `AddJob()`
4. Increment/decrement `RunningTasks` in task lifecycle
5. Add methods: `canClaimTask()`, `claimTask()`, `releaseTask()`

**Success Criteria:**

- Job cache tracks concurrency limit and current running tasks
- Thread-safe counter operations
- Jobs don't exceed their concurrency limit

---

#### Task 2.2: Modify Task Claiming to Respect Concurrency

**File:**
[internal/jobs/worker.go:395-434](../internal/jobs/worker.go#L395-L434)

**Current Behaviour:**

```go
func (wp *WorkerPool) claimPendingTask(ctx context.Context) (*db.Task, error) {
    // Round-robin: try each job once
    for _, jobID := range activeJobs {
        task, err := wp.dbQueue.GetNextTask(ctx, jobID)
        if task != nil {
            return task, nil  // First available task
        }
    }
    return nil, sql.ErrNoRows
}
```

**Target Behaviour:**

```go
func (wp *WorkerPool) claimPendingTask(ctx context.Context) (*db.Task, error) {
    // Try jobs that haven't hit their concurrency limit
    for _, jobID := range activeJobs {
        jobInfo := wp.jobInfoCache[jobID]

        // Check if job can accept more tasks
        if !jobInfo.canClaimTask() {
            continue  // Skip jobs at concurrency limit
        }

        task, err := wp.dbQueue.GetNextTask(ctx, jobID)
        if task != nil {
            jobInfo.claimTask()  // Increment running count
            return task, nil
        }
    }
    return nil, sql.ErrNoRows
}
```

**Implementation Steps:**

1. Add `canClaimTask()` check before claiming
2. Increment `RunningTasks` when task claimed
3. Decrement `RunningTasks` when task completes/fails
4. Add logging when jobs hit concurrency limit
5. Prioritise jobs below their concurrency quota

**Success Criteria:**

- Jobs with concurrency=3 never exceed 3 concurrent tasks
- Worker pool respects per-job concurrency limits
- Logs show when jobs hit/release concurrency limits

---

#### Task 2.3: Update Task Completion to Release Concurrency

**File:**
[internal/jobs/worker.go:529-534](../internal/jobs/worker.go#L529-L534)

**Current Behaviour:**

```go
result, err := wp.processTask(taskCtx, jobsTask)
if err != nil {
    return wp.handleTaskError(ctx, task, err)
} else {
    return wp.handleTaskSuccess(ctx, task, result)
}
```

**Target Behaviour:**

```go
result, err := wp.processTask(taskCtx, jobsTask)

// Always release concurrency slot
defer func() {
    jobInfo := wp.jobInfoCache[task.JobID]
    jobInfo.releaseTask()
}()

if err != nil {
    return wp.handleTaskError(ctx, task, err)
} else {
    return wp.handleTaskSuccess(ctx, task, result)
}
```

**Implementation Steps:**

1. Add `defer` to release concurrency slot after task processing
2. Ensure release happens on both success and error paths
3. Handle job removal edge case (job removed while task processing)
4. Add metrics/logging for concurrency slot usage
5. Test concurrent task completion

**Success Criteria:**

- Concurrency slots always released (even on panic)
- No concurrency slot leaks
- Jobs can process new tasks immediately after completion

---

### Phase 3: Testing and Validation

#### Task 3.1: Unit Tests for Concurrent Processing

**New File:** `internal/jobs/worker_concurrency_test.go`

**Test Cases:**

1. Worker processes multiple tasks concurrently
2. Worker respects concurrency semaphore
3. Graceful shutdown waits for in-flight tasks
4. Panic in one task doesn't affect others
5. Job concurrency limits are enforced
6. Concurrency slots are released on error

**Implementation Steps:**

1. Create mock crawler that simulates 1-2s delays
2. Test worker with concurrency=5, verify 5 tasks in-flight
3. Test job with concurrency=3, verify never exceeds 3
4. Test shutdown with in-flight tasks
5. Test error handling releases slots

---

#### Task 3.2: Load Test with Concurrent Processing

**File:** [scripts/load-test-simple.sh](../scripts/load-test-simple.sh)

**Test Scenario:**

1. Create 62 jobs with `concurrency: 3` (186 concurrent tasks expected)
2. Monitor throughput with new architecture
3. Verify each job gets 3 concurrent tasks
4. Measure actual tasks/second across all jobs

**Success Criteria:**

- Total throughput: 120-180 tasks/second
- Each job: 2-3 tasks/second
- No concurrency limit violations
- No worker pool saturation

---

#### Task 3.3: Monitor Production Metrics

**Metrics to Track:**

- Tasks/second total
- Tasks/second per job
- Worker concurrency utilisation (slots in use / total slots)
- Job concurrency utilisation (running tasks / concurrency limit per job)
- Database connection pool usage
- HTTP request latency distribution

**Implementation Steps:**

1. Add Prometheus metrics for worker concurrency
2. Add metrics for job concurrency tracking
3. Update Grafana dashboard with new metrics
4. Set up alerts for concurrency anomalies
5. Monitor production for 24-48 hours after deployment

---

## Configuration Recommendations

### Production

```go
jobWorkers = 100                    // Total workers
workerConcurrency = 5               // Tasks per worker
totalConcurrency = 500              // 100 × 5
```

With 62 jobs at concurrency=3:

- Required slots: 62 × 3 = 186
- Available slots: 500
- Headroom: 314 slots (good margin)

### Staging

```go
jobWorkers = 20
workerConcurrency = 3
totalConcurrency = 60
```

### Development

```go
jobWorkers = 5
workerConcurrency = 2
totalConcurrency = 10
```

## Expected Outcome

### Before

- 50 workers, synchronous processing
- 2-3 tasks/second total across 62 jobs
- Workers idle during HTTP requests

### After

- 100 workers, 5 concurrent tasks each = 500 capacity
- 120-180 tasks/second total across 62 jobs
- 2-3 tasks/second per job
- Full utilisation of worker capacity
- Respect for job concurrency limits

## Risk Assessment

### Low Risk

- Graceful shutdown may need tuning (wait for in-flight tasks)
- Database connection pool may need adjustment (more concurrent queries)

### Medium Risk

- Memory usage will increase (more goroutines, HTTP connections)
- Need to monitor for goroutine leaks

### Mitigation

- Start with low `workerConcurrency` (2-3) and scale up
- Add goroutine count monitoring
- Test thoroughly in staging before production
- Keep worker semaphore to prevent runaway concurrency

## Next Steps

1. Review this plan and confirm approach
2. Create feature branch: `feat/concurrent-task-processing`
3. Implement Phase 1 (concurrent processing per worker)
4. Test in development and staging
5. Implement Phase 2 (job concurrency enforcement)
6. Load test with 62 concurrent jobs
7. Deploy to production with monitoring
8. Validate 120-180 tasks/second throughput

## References

- [Current metrics](./performance-comparison-summary.md)
- [Worker pool architecture](../architecture/ARCHITECTURE.md#worker-pool)
- [Database queue implementation](../internal/db/queue.go)
- [Load testing script](../../scripts/load-test-simple.sh)
