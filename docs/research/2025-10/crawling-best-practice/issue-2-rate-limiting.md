# Issue #2: Per-Domain Rate Limiting

## Goal

Prevent HTTP 429 errors from aggressive sites by limiting concurrent requests
per domain, not globally.

## Problem Analysis

### Current Behaviour

**What exists:**

```go
// crawler.go - Global limit rule
c.Limit(&colly.LimitRule{
    DomainGlob:  "*",           // ← Applies to ALL domains
    Parallelism: 10,            // ← 10 concurrent globally
    RandomDelay: 333ms,         // ← 1s / RateLimit(3)
})

// worker.go - Per-worker delay
applyCrawlDelay(task)  // Sleeps for robots.txt crawl_delay
```

**What's missing:**

- `DomainGlob: "*"` creates a **global** semaphore (all domains share 10 slots)
- If 22 workers all claim everlane.com tasks, up to 10 can hit it simultaneously
- `applyCrawlDelay()` is serial per-worker (not concurrency control)

**Evidence from everlane.com failure:**

```
- 22 workers active
- 34 tasks claimed for everlane.com
- 10 concurrent requests hit everlane.com (via global semaphore)
- everlane.com responds with HTTP 429 after 26 successful tasks
- 2 retries without backoff (then task marked failed)
```

### Why This Happens

**Scenario:**

1. Job created for everlane.com with 500 URLs
2. 22 workers all claim everlane.com tasks (via database queue)
3. Each worker calls `applyCrawlDelay()` → sleeps 0-1 second (from robots.txt)
4. All 22 hit Colly's global `Parallelism: 10` semaphore
5. First 10 get through → 10 concurrent requests to everlane.com
6. everlane.com rate limiter: "too many from this IP" → HTTP 429
7. Workers retry up to 2 times (no backoff) → repeated 429 errors

**Root cause:** No per-domain concurrency cap means aggressive sites get
hammered.

## Technical Solution

**IMPORTANT:** The proper solution is implemented in **Issue #3 (Domain Rate
Limiter)**. This issue documents the problem analysis only.

### ~~Option A: Per-Domain LimitRules in Colly~~ (SUPERSEDED BY ISSUE #3)

**File:** `internal/crawler/crawler.go` **Location:** `New()` function where
`c.Limit()` is called

**Implementation:**

```go
// Add aggressive site rules first (most specific)
aggressiveSites := []string{
    "everlane.com",
    "shopify.com",
    "bigcommerce.com",
    // Add sites that commonly rate-limit
}

for _, domain := range aggressiveSites {
    if err := c.Limit(&colly.LimitRule{
        DomainGlob:  domain,
        Parallelism: 2,              // Only 2 concurrent per domain
        RandomDelay: 500 * time.Millisecond,
        Delay:       1 * time.Second,
    }); err != nil {
        log.Error().Err(err).Str("domain", domain).Msg("Failed to set domain limit")
    }
}

// Global fallback for all other domains
if err := c.Limit(&colly.LimitRule{
    DomainGlob:  "*",
    Parallelism: 10,            // 10 concurrent for normal sites
    RandomDelay: 333 * time.Millisecond,
}); err != nil {
    log.Error().Err(err).Msg("Failed to set global limit")
}
```

**How Colly resolves rules:**

- Most specific `DomainGlob` wins
- `everlane.com` matches first rule → `Parallelism: 2`
- `example.com` matches fallback `*` → `Parallelism: 10`

**Pros:**

- Uses Colly's built-in mechanism
- Per-domain limits enforced across all workers
- Easy to add new aggressive domains

**Cons:**

- Hardcoded domain list (needs maintenance)
- Can't dynamically adjust based on 429 responses

### Option B: Dynamic Domain Semaphore in Worker Pool

**File:** `internal/jobs/worker.go` **Location:** `WorkerPool` struct +
`processTask()` function

**Implementation:**

```go
// Add to WorkerPool struct
type WorkerPool struct {
    // ... existing fields
    domainSemaphores map[string]chan struct{}  // domain → semaphore
    domainSemMutex   sync.RWMutex
}

// Acquire domain semaphore before calling crawler
func (wp *WorkerPool) acquireDomainSlot(domain string, maxConcurrent int) func() {
    wp.domainSemMutex.Lock()
    sem, exists := wp.domainSemaphores[domain]
    if !exists {
        sem = make(chan struct{}, maxConcurrent)
        wp.domainSemaphores[domain] = sem
    }
    wp.domainSemMutex.Unlock()

    sem <- struct{}{}  // Block until slot available

    return func() { <-sem }  // Release function
}

// In processTask(), before crawler.WarmURL():
maxConcurrent := 10  // Default
if isAggressiveDomain(task.DomainName) {
    maxConcurrent = 2  // Aggressive sites
}
release := wp.acquireDomainSlot(task.DomainName, maxConcurrent)
defer release()

result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
```

**Pros:**

- Dynamic semaphores created per-domain as needed
- Can adjust `maxConcurrent` based on error rates
- No hardcoded domain list

**Cons:**

- More complex implementation
- Duplicates Colly's existing functionality
- Semaphore map grows unbounded (minor memory leak)

### Recommended Solution

**Use Issue #3 (Domain Rate Limiter)** - This is the proper solution that
enforces minimum time intervals between requests to the same domain, preventing
concurrent hammering.

The options above are documented for reference but are **not recommended**:

- Option A (LimitRules) requires hardcoding domain lists and doesn't scale
- Option B (semaphores) duplicates Colly functionality and is complex

Exponential backoff (below) can still be added as a defensive layer.

## Exponential Backoff (Optional Defensive Layer)

**File:** `internal/jobs/worker.go` **Location:** `processNextTask()` where
`isBlockingError()` is checked

**Current code:**

```go
if isBlockingError(err) {
    log.Error().Err(err).Str("task_id", dbTask.ID).Msg("Blocking error detected")
    // ← No backoff, retries immediately
}
```

**Add backoff:**

```go
func (wp *WorkerPool) handleBlockingError(task *db.Task, err error) {
    retryCount := task.RetryCount
    backoffDuration := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
    maxBackoff := 60 * time.Second

    if backoffDuration > maxBackoff {
        backoffDuration = maxBackoff
    }

    log.Warn().
        Err(err).
        Str("task_id", task.ID).
        Int("retry_count", retryCount).
        Dur("backoff", backoffDuration).
        Msg("Blocking error - applying exponential backoff")

    time.Sleep(backoffDuration)
}

// In processNextTask():
if isBlockingError(err) {
    wp.handleBlockingError(dbTask, err)
    // Then retry or mark failed based on retry count
}
```

**Backoff schedule:**

- Retry 0: 1 second (2^0)
- Retry 1: 2 seconds (2^1)
- Retry 2: 4 seconds (2^2)
- Retry 3: 8 seconds (2^3)
- Retry 4+: 60 seconds (capped)

## Testing Strategy

### Test Case 1: Per-Domain Limit

1. Create job for everlane.com with 100 URLs
2. Monitor concurrent request count via logs
3. Verify max 2 concurrent requests to everlane.com (if using aggressive list)
4. Verify other domains still get 10 concurrent

### Test Case 2: Exponential Backoff

1. Mock server that returns HTTP 429 for first 3 requests
2. Track retry timing
3. Verify backoff: 1s, 2s, 4s between retries
4. Confirm success on 4th attempt

### Test Case 3: No Regression for Normal Sites

1. Create job for cooperative site (e.g. teamharvey.co)
2. Verify still gets 10 concurrent workers
3. Confirm no performance degradation

## Deployment Impact

**Risk Level:** MEDIUM (changing concurrency behaviour)

**Breaking Changes:** None (only improves rate limiting)

**Performance Impact:**

- Negative: Slower crawls for aggressive sites (2 concurrent vs 10)
- Positive: Eliminates 429 errors and repeated retries
- Positive: Prevents IP bans from aggressive crawling

**Rollback Plan:**

- Remove per-domain `LimitRule` entries (keep only global `*` rule)
- Remove exponential backoff sleep

## Success Metrics

**Before:**

- everlane.com: HTTP 429 after 26 tasks
- 34 running tasks all failing
- 2 retries per task without backoff

**After:**

- everlane.com: Zero HTTP 429 errors
- Max 2 concurrent requests to everlane.com
- Successful completion of all tasks
- Failures reduced with exponential backoff on retries

**Monitoring:**

- Track 429 error rate (should be near-zero)
- Monitor per-domain concurrent request count
- Alert if any domain exceeds configured `Parallelism` limit
