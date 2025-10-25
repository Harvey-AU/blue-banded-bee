# Throughput Optimisation – Detailed Implementation Plan (v2.2)

**Date:** 2025-10-25  
**Goal:** Lift end-to-end throughput from 2–3 tasks/sec (50 sequential workers)
to 120–180 tasks/sec by allowing each worker to handle several tasks
concurrently while enforcing per-job concurrency limits.

---

## 1. Executive Overview

### Current Baseline

- Worker pool launches one goroutine per worker; each goroutine blocks on
  `processNextTask`.
- Task throughput is throttled by HTTP latency (1–2 s per request); idle time
  dominates.
- Database already tracks desired per-job concurrency but nothing enforces it.

### Target State

- 100 workers, each able to process up to 5 tasks in parallel (configurable per
  environment).
- Job-level concurrency respected via an atomic counter on the `jobs` table.
- Graceful shutdown, structured logging, batching, and observability remain
  intact.

### Guiding Principles

1. Preserve existing worker APIs and batching pathways.
2. Keep concurrency configurable at runtime (environment variables).
3. Avoid regressions in error handling, logging, and back-off behaviour.
4. Keep plan compatible with Australian English spellings and current CI linting
   rules.

---

## 2. Key Constraints & Dependencies

- **Graceful shutdown:** Workers must wait for in-flight goroutines to finish.
- **Back-off:** Idle workers should retain exponential back-off to avoid hot
  loops.
- **Batch manager:** Task completion still flows through the batch manager; any
  counter we introduce must integrate with its bulk updates.
- **Database:** We can add a migration (Supabase-managed) but must avoid
  destructive changes.
- **Observability:** Reuse the OpenTelemetry provider already initialised in
  `internal/observability`; metrics must be cheap and cardinality-aware.

---

## 3. Phase 1 – Per-Worker Task Concurrency

### 3.1 Worker Loop Refactor

- Extend `WorkerPool` with:
  - `workerConcurrency int`
  - `workerSemaphores []chan struct{}`
  - `workerWaitGroups []*sync.WaitGroup`
- Update `NewWorkerPool` signature to accept concurrency and initialise
  per-worker semaphore/waitgroup structures. Add validation (min 1, max 20) and
  ensure we pass the value from `cmd/app/main.go`.
- Worker loop changes:
  - Maintain back-off state in the outer loop only.
  - Use a buffered channel (one per worker) as a semaphore; launch goroutines
    only when a slot is free.
  - Deliver task outcomes back over a results channel so back-off logic lives in
    the main goroutine.
  - `defer wg.Wait()` so shutdown honours in-flight work.
  - Add the `errors` import for `errors.Is(err, sql.ErrNoRows)`.

### 3.2 Configuration

- Introduce `WORKER_CONCURRENCY` in `cmd/app/main.go` using helper `getEnvInt`.
- Validate range at start-up and log combined capacity
  (`pool_size × concurrency`).
- Document the new variable in `README.md`, `docs/development/DEVELOPMENT.md`,
  and `.env.example`.

### 3.3 Observability

- Extend `internal/observability/observability.go` to emit:
  - `worker.concurrent_tasks` (int64 up-down counter, label `worker.id`).
  - `worker.concurrency_capacity` (gauge, label `worker.id`, recorded on start).
- Wrap increments/decrements in helper
  `RecordWorkerConcurrency(ctx, workerID, delta, capacity)` and call it in the
  worker goroutines (increment on launch, decrement on completion).
- Keep logging noise low (info on worker start, debug for capacity snapshots if
  required).

### 3.4 Testing

- Unit tests for worker concurrency behaviour
  (`internal/jobs/worker_concurrency_test.go`):
  - Parallel processing (atomic counter to capture peak concurrency).
  - Back-off path (simulate `sql.ErrNoRows` and ensure sleeps/back-off
    increments).
  - Graceful shutdown (cancel context mid-flight).
- Run targeted tests with `go test ./internal/jobs -race`.

---

## 4. Phase 2 – Enforce Job Concurrency

### 4.1 Schema & Migration

- Add `running_tasks INTEGER NOT NULL DEFAULT 0` to `jobs`.
- Backfill existing rows:
  ```sql
  UPDATE jobs
  SET running_tasks = (
      SELECT COUNT(*)
      FROM tasks
      WHERE job_id = jobs.id AND status = 'running'
  )
  WHERE running_tasks = 0;
  ```
- Create supporting index:
  ```sql
  CREATE INDEX IF NOT EXISTS idx_jobs_running_tasks
  ON jobs (id, running_tasks, concurrency);
  ```

### 4.2 Task Claim Path (`internal/db/queue.go`)

- Replace `GetNextTask` with a single transaction that:
  1. Locks the relevant job row (`FOR UPDATE`).
  2. Checks `running_tasks < concurrency` (or treats `NULL/0` as unlimited).
  3. Selects the next pending task (`FOR UPDATE SKIP LOCKED`).
  4. Marks the task as running and increments `running_tasks` in the same CTE
     chain.
- Continue honouring priority ordering (`priority_score DESC`, then
  `created_at ASC`).
- Ensure the function still returns `nil, nil` when no eligible task exists (so
  worker back-off is triggered).

### 4.3 Counter Decrement

- When a task finishes (success/failure) or re-queues for retry, decrement
  `running_tasks` atomically.
- Update both code paths:
  1. `DbQueue.UpdateTaskStatus` (single-task updates).
  2. `db.BatchManager` bulk flushes (wrap existing `UPDATE` statements in a CTE
     that returns job IDs and applies
     `running_tasks = GREATEST(running_tasks - 1, 0)`).
- Include tests that cover:
  - Boundary conditions (counter never drops below zero).
  - Retry path (running → pending reduces the counter once).
  - Batch updates process multiple rows.

### 4.4 Validation

- API: confirm `/v1/jobs` already defaults concurrency and add validation for a
  maximum (`MaxJobConcurrency`, e.g. 50).
- Add defensive logging when the counter drifts (should not happen if both paths
  update correctly).

---

## 5. Phase 3 – Verification & Rollout

### 5.1 Unit & Integration Tests

- `go test ./... -race -cover`: ensure new code keeps overall coverage ≥
  existing baseline (38.9%) and new packages >80%.
- Add integration tests:
  - Worker pool processes tasks concurrently without violating per-job limits.
  - Concurrency-limited job stays capped while another job progresses.

### 5.2 Load Testing

- Update `scripts/load-test-simple.sh` to accept `WORKER_CONCURRENCY`.
- Test plan:
  1. Baseline (current behaviour) for comparison.
  2. Concurrent workers without job limits (sanity check).
  3. Concurrent workers with mixed job concurrency values.
  4. Target scenario: 100 workers × 5 concurrency, 62 jobs × 3 concurrency →
     verify 120–180 tasks/sec.
- Capture metrics: total throughput, per-job throughput, worker slot
  utilisation, DB connections, memory/CPU.

### 5.3 Deployment

- Staging soak: `WORKER_POOL_SIZE=10`, `WORKER_CONCURRENCY=3`, run 24 h.
- Production canary: roll out to 20% of workers (e.g. 20 threads) for 1 h.
- Full rollout: 100 workers, concurrency tuned between 3–5 depending on load
  test results.
- Rollback lever: set `WORKER_CONCURRENCY=1` (existing behaviour) and redeploy.

---

## 6. Config & Documentation Updates

| File                              | Change                                                       |
| --------------------------------- | ------------------------------------------------------------ |
| `README.md`                       | Document `WORKER_CONCURRENCY` and combined capacity example. |
| `docs/development/DEVELOPMENT.md` | Add tuning guidance by environment/site speed.               |
| `.env.example`                    | Include default concurrency for local dev (5).               |
| `CHANGELOG.md`                    | Summarise feature once merged.                               |

---

## 7. Risks & Mitigations

| Risk                 | Impact                        | Mitigation                                                                                                         |
| -------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Goroutine leaks      | Memory/FD pressure            | Always release semaphore in `defer`, use `wg.Wait()` on exit, add tests.                                           |
| Counter drift        | Jobs block permanently        | Atomically increment/decrement in _all_ paths, monitor metric exposed from DB (optional query in admin dashboard). |
| DB saturation        | Connection pool exhaustion    | Validate Supabase pool limits before deployment; load test with staging data.                                      |
| Hot loop regressions | CPU spike during idle         | Worker back-off tests + `-race` to catch shared state mutations.                                                   |
| Metrics overhead     | Latency or cardinality issues | Use minimal labels (`worker.id`), drop debug logging in production.                                                |

---

## 8. Timeline (Estimate)

| Phase     | Tasks                                                 | Duration              |
| --------- | ----------------------------------------------------- | --------------------- |
| Phase 1   | Worker refactor, config, observability, unit tests    | 3–4 days              |
| Phase 2   | Migration, dequeue rewrite, counter decrements, tests | 2–3 days              |
| Phase 3   | Load tests, staging soak, production rollout          | 2–3 days              |
| **Total** |                                                       | **7–10 working days** |

---

## 9. Next Actions

1. Raise Supabase migration for `running_tasks`.
2. Create feature branch `feat/worker-task-concurrency`.
3. Implement Phase 1, following Extract → Test → Commit cadence.
4. Keep daily updates (or as agreed) on progress, blockers, and metrics.
5. Prepare rollout playbook and monitoring dashboard before deployment.

---

**Plan Status:** Ready for implementation (pending approval)  
**Author:** Claude (reviewed by Codex)  
**Revision:** 2.2 (streamlined following feedback on excessive detail)
