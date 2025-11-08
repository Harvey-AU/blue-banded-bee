# Pattern 2: Waiting Task Backlog

**Severity:** Low (by design, tasks eventually process) **Urgency:** Low
(functioning as intended for respectful crawling) **Complexity:** Medium (would
require state machine refactor)

---

## Observations

```
Tasks in waiting state: 18,734 (peak)
Tasks in pending state: 531
Tasks in progress: 53
Workers active: 40-85 goroutines (stable)
```

**Key metric:** Large backlog despite abundant resources (75% CPU free, 55 DB
connections available)

---

## User Context

> "This is partly by design, we have delays per domain and limits of number of
> connections to db/workers to reduce pressure, but if we can improve this
> through any logic/design or workflow changes that'd be great."

---

## Current Behaviour

Tasks enter "waiting" state when:

1. **Domain rate limiting active** - Respecting crawl delays from robots.txt
2. **Worker pool at capacity** - Max workers per job reached
3. **Concurrency limits** - Max concurrent requests to domain reached

**This is intentional throttling** to:

- Prevent overwhelming target websites (respectful crawling)
- Respect robots.txt crawl-delay directives
- Avoid database connection exhaustion
- Prevent memory pressure from too many concurrent requests

---

## Research Questions

### 1. What are the waiting conditions?

**Current problem:** We don't track **why** tasks are waiting

**Investigation needed:**

```go
type WaitingReason string

const (
    WaitingForDomainDelay     WaitingReason = "domain_delay"
    WaitingForWorkerCapacity  WaitingReason = "worker_capacity"
    WaitingForConcurrencyLimit WaitingReason = "concurrency_limit"
    WaitingForDependency      WaitingReason = "dependency" // If we ever add task dependencies
)

// When transitioning to waiting, log the reason
log.Info().
    Str("task_id", taskID).
    Str("waiting_reason", string(reason)).
    Msg("Task transitioned to waiting state")
```

**Expected distribution:**

- **Hypothesis:** 80-90% waiting for domain_delay (respectful crawling working
  as designed)
- If >50% waiting for worker_capacity: Worker pool sizing issue
- If >50% waiting for concurrency_limit: Limits too conservative

### 2. Are waiting tasks eventually processed?

**Current problem:** No visibility into time spent waiting

**Metrics needed:**

```go
// When task transitions waiting → pending
duration := time.Since(task.WaitingStartedAt)

// Histogram metric
waitingDuration.Observe(duration.Seconds())

// Log percentiles
log.Info().
    Dur("p50", p50).
    Dur("p95", p95).
    Dur("p99", p99).
    Msg("Waiting state duration distribution")
```

**Success criteria:**

- If median < 30 seconds: Working as designed (normal crawl delays)
- If median 1-5 minutes: Acceptable (respecting slow domains)
- If median > 5 minutes: State machine issue (tasks stuck)

### 3. Can we optimise state transitions?

**Current approach:** Polling-based

```
Every N seconds:
1. Scan waiting tasks
2. Check if delay expired
3. Transition to pending
```

**Alternative:** Event-driven

```go
// When task transitions to waiting, schedule wake-up
timer := time.AfterFunc(delayDuration, func() {
    transitionToPending(taskID)
})

// Store timer with task
task.WakeUpTimer = timer

// If job cancelled, cancel all timers
for _, task := range job.WaitingTasks {
    task.WakeUpTimer.Stop()
}
```

**Benefits:**

- Faster transitions (no polling delay)
- Less DB load (no periodic scans)
- Precise wake-up timing

**Risks:**

- Memory overhead (1 timer per waiting task = 18k timers at peak)
- Timer cleanup complexity (on job cancel, task fail, etc.)
- Persistence challenge (timers lost on restart)

---

## Optimisation Candidates

### Phase 1: Observability (Low Risk, High Value)

- [ ] Add `waiting_reason` field to task state transitions
- [ ] Add metrics: `tasks_waiting_by_reason{reason="domain_delay"}` (gauge)
- [ ] Add histogram: `waiting_duration_seconds` (time in waiting state)
- [ ] Dashboard: Show waiting task breakdown by reason

**Expected insights:**

- Confirm 80%+ waiting for domain delays (validates design)
- Identify if worker capacity or concurrency limits are too conservative
- Detect stuck tasks (p99 > 10 minutes)

### Phase 2: Optimise Polling (Low Risk, Medium Impact)

**Current state:** Unknown polling interval (need to check code)

**Optimisations:**

```go
// Instead of fixed interval, poll based on next wake-up time
nextWakeUp := findEarliestWakeUpTime(waitingTasks)
sleepDuration := time.Until(nextWakeUp)
time.Sleep(sleepDuration)
```

**Benefits:**

- No unnecessary polls when all tasks have long delays
- Faster transitions for tasks with short delays
- Reduced DB load

### Phase 3: Hybrid Approach (Medium Risk, High Impact)

**Combine polling for safety with event-driven for performance:**

```go
// Event-driven for short delays (<5 minutes)
if delayDuration < 5*time.Minute {
    timer := time.AfterFunc(delayDuration, transitionToPending)
}

// Polling for long delays (>5 minutes)
// Scans every minute, low frequency, low impact
```

**Benefits:**

- Fast transitions for 90%+ of tasks (short delays)
- Safety net for long delays (polling catches stragglers)
- Bounded memory (only timers for short-delay tasks)

### Phase 4: Priority Queue for Pending Tasks (Medium Risk)

**Current:** Tasks transition waiting → pending in arbitrary order

**Alternative:** Pending tasks in priority queue

```go
type PendingQueue struct {
    heap *priorityHeap // Min-heap ordered by priority (high priority first)
}

// Worker claims highest-priority pending task
task := pendingQueue.PopHighestPriority()
```

**Benefits:**

- Critical path tasks processed first (higher priority)
- Better throughput (work on important tasks during contention)

**Trade-off:**

- More complex queue management
- Potential starvation of low-priority tasks (need fairness mechanism)

---

## Analysis: Is This Actually a Problem?

**Evidence suggesting "by design, not a bug":**

1. ✅ Workers stable at 40-85 goroutines (not blocked or crashing)
2. ✅ 75% CPU headroom (not resource-constrained)
3. ✅ 55 DB connections available (not connection pool exhaustion)
4. ✅ Tasks completing steadily (9,032 claimed during test)
5. ✅ No stuck task errors in logs

**Expected behaviour for 45-50 concurrent jobs:**

- Each job crawls different domain with different crawl delays
- 18k waiting tasks / 45 jobs = **400 tasks per job** in backlog
- If average crawl delay is 10 seconds per task: 400 tasks × 10s = **4,000
  seconds = 67 minutes to clear**
- This is **normal** for respectful crawling of large sites

**Recommendation:**

- **No immediate action required** (system working as designed)
- Implement Phase 1 (observability) to **validate hypothesis**
- Only proceed to optimisations if metrics show actual problems:
  - > 50% waiting for worker_capacity (not domain_delay)
  - p99 waiting duration >10 minutes
  - Tasks stuck in waiting state without progressing

---

## Alternative Perspective: What if 18k Backlog is Good?

**Counterargument:** Large backlog might be **intentional and beneficial**:

1. **Queue depth indicates work availability** - Workers always have tasks to
   claim
2. **Smooth out bursty discovery** - Sitemap processing creates 20k tasks in 2
   minutes; waiting state prevents overwhelming workers
3. **Respectful crawling** - 18k tasks spread across 45 domains with 10-30s
   delays = hours of work, as intended

**If we "optimised" this away:**

- Tasks might claim workers immediately → violate crawl delays
- Workers might overwhelm target sites → get rate-limited or blocked
- System might consume more resources for no throughput gain

**Conclusion:** Measure first, optimise only if data shows actual inefficiency.

---

## Success Metrics

**Phase 1 (Observability) Success:**

- [ ] Waiting reason breakdown shows 80%+ domain_delay (validates design)
- [ ] p95 waiting duration <5 minutes (reasonable)
- [ ] p99 waiting duration <15 minutes (no stuck tasks)
- [ ] No tasks waiting >30 minutes without progress

**If optimisation needed:**

- [ ] Reduce p95 waiting duration by 30% (for non-domain-delay tasks)
- [ ] Reduce DB polling queries by 50%
- [ ] Maintain crawl delay compliance (no robots.txt violations)
