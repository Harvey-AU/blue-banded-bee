# Pattern 3: Worker Pool Scaling Behaviour

**Severity:** Unknown (may be working as designed) **Urgency:** Low (stable
performance, no crashes) **Complexity:** Low (likely just needs observability)

---

## Observations

```
Logs: 3,165 "Job performance scaling evaluated"
Logs: 564 "Starting worker"
Goroutines: Stable 40-85 throughout test (no correlation with evaluations)
```

**Key disconnect:** 3,165 scaling evaluations but goroutine count doesn't track
evaluations.

---

## User Context

> "Question here is whether it should be scaling or not, is this because we're
> hitting some caps/thresholds or other rules are reducing worker pool?"

---

## Current Behaviour

Worker pool evaluates scaling conditions frequently (3,165 times during
82-minute test), but:

1. **Goroutines remain stable** at 40-85 throughout
2. **No correlation** between evaluation frequency and worker count changes
3. **"Starting worker" logs (564)** don't match evaluation frequency

**Hypothesis:** Evaluations are running but **determining no scaling needed**
(already optimal), not logging the decision.

---

## Research Questions

### 1. Why don't evaluations result in scaling?

**Possible reasons:**

a. **Evaluation determines scaling is not needed** (already optimal)

- Current worker count matches workload
- Adding more workers wouldn't improve throughput
- System correctly identifies bottleneck elsewhere (DB, rate limits)

b. **Hitting concurrency caps**

- `maxWorkersPerJob` limit reached
- `maxTotalWorkers` limit reached
- Per-domain concurrency limits preventing new workers

c. **Resource limits**

- DB connection pool approaching limit (35 of 90 used, app configured for 48)
- Memory thresholds preventing worker addition
- CPU thresholds (though 75% headroom available)

d. **Rate limiting preventing additions**

- Domain crawl delays making more workers ineffective
- Respectful crawling constraints (18,734 waiting tasks)

### 2. What are the scaling rules?

**Investigation needed:**

```go
// Review evaluateScaling() logic
func (wp *WorkerPool) evaluateScaling(jobID string) {
    // What conditions trigger scale-up?
    // What conditions trigger scale-down?
    // What caps/limits are checked?

    // Add logging for decision reasoning
    log.Info().
        Str("job_id", jobID).
        Str("decision", decision). // "scale_up" | "scale_down" | "no_change"
        Str("reason", reason).     // "at_max_workers" | "sufficient_capacity" | etc.
        Int("current_workers", currentWorkers).
        Int("max_workers", maxWorkers).
        Int("pending_tasks", pendingTasks).
        Msg("Scaling decision")
}
```

**Constants to review:**

- `maxWorkersPerJob` - What is this set to?
- `maxTotalWorkers` - What is the global limit?
- `maxConcurrencyPerDomain` - Domain-level concurrency limit
- DB connection pool size vs worker count relationship

### 3. Is current scaling optimal?

**Evidence suggesting it's working correctly:**

1. ✅ **75% CPU headroom** - Not CPU-constrained, more workers wouldn't help
2. ✅ **Stable goroutines** - System found equilibrium, not thrashing
3. ✅ **Tasks completing steadily** - 9,726 tasks claimed during test
4. ✅ **No crashes or panics** - Stable under extreme load (10x normal)
5. ✅ **Resource efficiency** - Not over-provisioning workers unnecessarily

**Hypothesis:** Scaling is **constrained by design**, not by mistake.

**Bottleneck is likely:**

- Domain rate limiting (18,734 waiting tasks)
- DB contention during priority update storms
- Concurrency limits to prevent overwhelming target sites

**More workers would NOT help** if:

- Tasks are waiting for domain delays (respectful crawling)
- DB is bottleneck (adding workers increases contention)
- Target websites would rate-limit us

---

## Analysis: Are 564 Worker Starts Correct?

**Math check:**

```
Total test duration: 82 minutes
Worker starts: 564
Average: 6.9 worker starts per minute

Jobs running: 45-50 concurrent
Workers per job (average): 40-85 goroutines / 45 jobs ≈ 1-2 workers per job
```

**Possible explanations:**

1. **Workers are short-lived** - Start, process task, exit (task-based
   lifecycle)
2. **Workers replaced frequently** - Old workers exit, new ones start (churn)
3. **Scaling up/down dynamically** - Responding to task bursts (sitemap
   processing)
4. **Multiple worker types** - Different worker pools (main workers, sitemap
   workers, etc.)

**This pattern is NORMAL if:**

- Workers have task-based lifecycle (start → claim task → process → exit)
- System scales up during bursts (20k tasks created in 2 minutes at 07:56-07:58)
- System scales down during quiet periods (no pending tasks)

---

## Optimisation Candidates

### Phase 1: Observability (Low Risk, High Value)

**Add telemetry to understand scaling decisions:**

```go
type ScalingDecision string

const (
    ScaleUp      ScalingDecision = "scale_up"
    ScaleDown    ScalingDecision = "scale_down"
    NoChange     ScalingDecision = "no_change"
)

type ScalingReason string

const (
    AtMaxWorkers        ScalingReason = "at_max_workers"
    SufficientCapacity  ScalingReason = "sufficient_capacity"
    NoPendingTasks      ScalingReason = "no_pending_tasks"
    DBConnectionLimit   ScalingReason = "db_connection_limit"
    MemoryThreshold     ScalingReason = "memory_threshold"
)

// Log every scaling evaluation with decision + reason
log.Info().
    Str("job_id", jobID).
    Str("decision", string(decision)).
    Str("reason", string(reason)).
    Int("current_workers", currentWorkers).
    Int("max_workers", maxWorkers).
    Int("pending_tasks", pendingTasks).
    Int("waiting_tasks", waitingTasks).
    Int("db_connections", dbConnections).
    Msg("Scaling evaluation completed")
```

**Metrics to add:**

```go
// Counters
scaling_decisions_total{decision="scale_up|scale_down|no_change",reason="..."}

// Gauges
active_workers{job_id="..."}
max_workers_per_job
max_total_workers

// Histograms
worker_lifetime_seconds (how long workers live before exiting)
```

### Phase 2: Review Scaling Caps (Low Risk, Medium Impact)

**Current constraints to review:**

```go
// Example constants (actual values unknown)
const (
    maxWorkersPerJob   = 10  // ??? - need to verify
    maxTotalWorkers    = 100 // ??? - need to verify
    maxDBConnections   = 48  // Current app config (could increase to 60-70)
)
```

**Questions:**

1. **maxWorkersPerJob**: Is 10 (or current value) appropriate?
   - For job with 5,000 pending tasks: 10 workers = 500 tasks per worker
   - If average task takes 10 seconds: 500 × 10s = 5,000 seconds = 83 minutes
     per worker cycle
   - Is this acceptable throughput?

2. **maxTotalWorkers**: Is 100 (or current value) appropriate?
   - With 50 concurrent jobs: 100 workers = 2 workers per job (on average)
   - Does this match observed 40-85 goroutines (0.8-1.7 per job)?

3. **DB connection pool**: Currently app configured for 48, but Supabase allows
   90
   - Peak usage: 35 connections
   - Headroom: 13 connections (27% of configured limit)
   - Could increase to 60-70 for better concurrency during bursts

### Phase 3: Optimise Scaling Algorithm (Medium Risk, High Impact)

**Current approach (assumed):** Fixed caps, static evaluation

**Alternative:** Dynamic scaling based on task backlog + domain delays

```go
// Calculate effective pending tasks (exclude those waiting for domain delays)
effectivePendingTasks := pendingTasks + waitingTasksReadyNow

// Scale workers based on effective backlog
desiredWorkers := min(
    effectivePendingTasks / tasksPerWorker,
    maxWorkersPerJob,
    (maxDBConnections - currentDBConnections) / connectionsPerWorker,
)

// Only scale up if we have work AND resources
if desiredWorkers > currentWorkers && effectivePendingTasks > threshold {
    scaleUp(desiredWorkers - currentWorkers)
}
```

**Benefits:**

- Don't scale up when tasks are just waiting for domain delays
- Respect DB connection pool limits dynamically
- Scale proportionally to actual workload (not fixed caps)

---

## Success Metrics

**Phase 1 (Observability) Success:**

- [ ] Scaling decision breakdown shows reasoning distribution
  - If 80%+ "sufficient_capacity" → System correctly not over-scaling
  - If 50%+ "at_max_workers" → Caps may be too conservative
  - If 20%+ "db_connection_limit" → Need to increase pool size
- [ ] Worker lifetime histogram shows expected pattern
  - If median < 30 seconds: Task-based lifecycle (normal)
  - If median > 5 minutes: Long-lived workers (expected for crawling)
- [ ] No scaling thrashing (rapid up/down cycles)

**If optimisation needed:**

- [ ] Increase maxWorkersPerJob based on empirical data (if caps too
      conservative)
- [ ] Increase DB connection pool from 48 to 60-70 (utilize available Supabase
      capacity)
- [ ] Reduce scaling evaluation frequency if 90%+ result in "no_change" (reduce
      overhead)

---

## Conclusion

**Current verdict: Likely working as designed.**

**Evidence:**

1. ✅ Stable goroutine count (40-85) despite high load
2. ✅ 75% CPU headroom (not resource-constrained)
3. ✅ Tasks completing steadily (9,726 claimed)
4. ✅ No crashes or scaling thrashing

**Recommendation:**

- **Implement Phase 1 (observability)** to confirm hypothesis
- Only proceed to optimisations if metrics show:
  - > 50% evaluations hitting maxWorkers cap (too conservative)
  - Significant pending task backlog with idle workers (under-scaling)
  - DB connection pool exhaustion preventing scaling (need to increase pool)

**Most likely outcome:** Scaling is correctly identifying that more workers
wouldn't help due to:

- Domain rate limiting (18,734 waiting tasks)
- DB contention (priority update storms)
- Respectful crawling constraints (crawl delays)
