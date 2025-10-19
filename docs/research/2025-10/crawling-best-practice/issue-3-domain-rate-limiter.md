# Issue #3: Domain-Level Rate Limiting

## Goal

Prevent rate limiting (HTTP 429) and respect robots.txt Crawl-delay by enforcing
a minimum time interval between requests to the same domain, regardless of how
many workers are active.

## Problem Analysis

### Current Behaviour

**What exists:**

- `applyCrawlDelay(task)` - Each worker sleeps for robots.txt Crawl-delay
  **before** making request
- `Colly LimitRule` - Global `Parallelism: 10` across ALL domains

**The Issue:**

```
10 workers claim tasks for everlane.com
↓
All 10 call applyCrawlDelay() → sleep 1 second in parallel
↓
All 10 wake up at same time
↓
All 10 hit Colly's global Parallelism: 10 semaphore
↓
All 10 get through (different domain slots available)
↓
10 concurrent requests to everlane.com from same IP
↓
everlane.com rate limiter: HTTP 429
```

**Root cause:** No coordination between workers to prevent concurrent requests
to the same domain.

### What Crawl-delay Actually Means

**robots.txt intent:** "Wait X seconds **between consecutive requests** to this
domain from this crawler"

**Current implementation:** "Each worker waits X seconds **before its own
request** (no coordination)"

**Correct implementation:** "Enforce X seconds **since the last request to this
domain by any worker**"

## Technical Solution

### Domain-Level Rate Limiter (Single Source of Truth)

**Architecture:**

```
WorkerPool
  ├─ domainRateLimiter *DomainRateLimiter (shared across all workers)
  └─ workers (50x goroutines)
       └─ Before crawler.WarmURL():
            domainRateLimiter.Wait(domain, crawlDelay)
```

**Data Structure:**

```go
type DomainRateLimiter struct {
    lastRequest map[string]time.Time  // domain → timestamp of last request
    mu          sync.RWMutex            // Protects map access
}
```

**Core Logic:**

```go
func (d *DomainRateLimiter) Wait(ctx context.Context, domain string, crawlDelay int) error {
    d.mu.Lock()

    // Minimum 1 request/second, or use robots.txt Crawl-delay
    minInterval := time.Duration(crawlDelay) * time.Second
    if minInterval < 1*time.Second {
        minInterval = 1 * time.Second
    }

    // Check when we last hit this domain
    var sleepDuration time.Duration
    if lastTime, exists := d.lastRequest[domain]; exists {
        elapsed := time.Since(lastTime)
        if elapsed < minInterval {
            sleepDuration = minInterval - elapsed
        }
    }

    // Reserve this time slot BEFORE sleeping (prevents race)
    d.lastRequest[domain] = time.Now().Add(sleepDuration)
    d.mu.Unlock()  // ← CRITICAL: Release lock before sleeping

    // Sleep outside mutex so other domains can proceed
    if sleepDuration > 0 {
        select {
        case <-time.After(sleepDuration):
            return nil
        case <-ctx.Done():
            return ctx.Err()  // ← Respect 2-minute task timeout
        }
    }
    return nil
}
```

### Where to Integrate

**File:** `internal/jobs/worker.go` **Function:** `processTask()` **Location:**
After `applyCrawlDelay()`, before `crawler.WarmURL()`

**Current flow:**

```go
func (wp *WorkerPool) processTask(ctx context.Context, task *Task) (*crawler.CrawlResult, error) {
    urlStr := constructTaskURL(task.Path, task.DomainName)

    applyCrawlDelay(task)  // Per-worker sleep (keep for now)

    result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
    // ...
}
```

**New flow:**

```go
func (wp *WorkerPool) processTask(ctx context.Context, task *Task) (*crawler.CrawlResult, error) {
    urlStr := constructTaskURL(task.Path, task.DomainName)

    // NEW: Domain-level rate limiting (enforces interval between requests)
    if err := wp.domainRateLimiter.Wait(ctx, task.DomainName, task.CrawlDelay); err != nil {
        return nil, fmt.Errorf("domain rate limiter cancelled: %w", err)
    }

    result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
    // ...
}
```

**Note:** Can remove `applyCrawlDelay()` since domain rate limiter enforces the
same delay (but at domain level, not worker level).

### Memory Management

**Challenge:** Map grows unbounded (one entry per domain ever crawled)

**Solution:** Periodic cleanup of stale entries

```go
func (d *DomainRateLimiter) Cleanup() {
    d.mu.Lock()
    defer d.mu.Unlock()

    // Remove domains not accessed in last hour
    cutoff := time.Now().Add(-1 * time.Hour)
    for domain, lastTime := range d.lastRequest {
        if lastTime.Before(cutoff) {
            delete(d.lastRequest, domain)
        }
    }
}
```

**When to run:** Background goroutine every 5 minutes

**Memory impact:**

- Active domains (last hour): ~1,000-10,000 entries
- Each entry: ~100 bytes (string + time.Time)
- Total memory: ~1 MB (negligible)

## Implementation Steps

### 1. Create DomainRateLimiter Type

**File:** `internal/jobs/domain_rate_limiter.go` (new file)

```go
package jobs

import (
    "sync"
    "time"
)

type DomainRateLimiter struct {
    lastRequest map[string]time.Time
    mu          sync.RWMutex
}

func NewDomainRateLimiter() *DomainRateLimiter {
    return &DomainRateLimiter{
        lastRequest: make(map[string]time.Time),
    }
}

func (d *DomainRateLimiter) Cleanup() {
    d.mu.Lock()
    defer d.mu.Unlock()

    cutoff := time.Now().Add(-1 * time.Hour)
    for domain, lastTime := range d.lastRequest {
        if lastTime.Before(cutoff) {
            delete(d.lastRequest, domain)
        }
    }
}
```

### 2. Add to WorkerPool

**File:** `internal/jobs/worker.go`

**Add field to WorkerPool struct:**

```go
type WorkerPool struct {
    // ... existing fields
    domainRateLimiter *DomainRateLimiter
}
```

**Initialise in NewWorkerPool():**

```go
func NewWorkerPool(...) *WorkerPool {
    wp := &WorkerPool{
        // ... existing fields
        domainRateLimiter: NewDomainRateLimiter(),
    }

    // Start cleanup routine
    wp.wg.Add(1)
    go func() {
        defer wp.wg.Done()
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                wp.domainRateLimiter.Cleanup()
            case <-wp.stopCh:
                return
            }
        }
    }()

    return wp
}
```

### 3. Integrate into processTask()

**File:** `internal/jobs/worker.go` **Function:** `processTask()`

**Add before crawler.WarmURL():**

```go
// Apply domain-level rate limiting
wp.domainRateLimiter.Wait(task.DomainName, task.CrawlDelay)

result, err := wp.crawler.WarmURL(ctx, urlStr, task.FindLinks)
```

**Optional:** Remove `applyCrawlDelay(task)` call (redundant with domain rate
limiter)

### 4. Add Tests

**File:** `internal/jobs/domain_rate_limiter_test.go` (new file)

Test cases:

- First request to domain (no delay)
- Second request within interval (should sleep)
- Second request after interval (no sleep)
- Concurrent requests to same domain (serialize)
- Concurrent requests to different domains (parallel)
- Cleanup removes stale entries
- Cleanup preserves recent entries

## Expected Behaviour After Implementation

### Example: everlane.com with Crawl-delay: 1

**Before (broken):**

```
T=0s:  Workers 1-10 claim everlane.com tasks
T=0s:  All sleep 1 second (applyCrawlDelay)
T=1s:  All wake up, 10 concurrent requests → HTTP 429
```

**After (fixed):**

```
T=0s:  Worker 1 claims task, calls Wait("everlane.com", 1)
T=0s:  No previous request → proceeds immediately
T=0s:  Worker 1 makes request, records lastRequest["everlane.com"] = T=0s

T=0.5s: Worker 2 claims task, calls Wait("everlane.com", 1)
T=0.5s: lastRequest was 0.5s ago → sleeps 0.5s
T=1s:   Worker 2 proceeds, makes request, records lastRequest = T=1s

T=1.2s: Worker 3 claims task, calls Wait("everlane.com", 1)
T=1.2s: lastRequest was 0.2s ago → sleeps 0.8s
T=2s:   Worker 3 proceeds, makes request, records lastRequest = T=2s

Result: Strict 1 req/second to everlane.com, zero 429 errors
```

### Multi-Domain Performance

**With multiple different domains:**

Workers processing different domains proceed in parallel with no contention. The
domain rate limiter only serializes requests to the **same** domain.

**If multiple workers hit same domain:**

```
Workers 1-5 all claim tasks for popular-domain.com:
- Worker 1: proceeds immediately
- Worker 2: waits 1 second (serialized by rate limiter)
- Worker 3: waits 1 second after Worker 2
- Worker 4: waits 1 second after Worker 3
- Worker 5: waits 1 second after Worker 4

Other workers: unaffected, working on other domains
```

## Testing Strategy

### Unit Tests

**File:** `internal/jobs/domain_rate_limiter_test.go`

1. **TestDomainRateLimiter_FirstRequest** - No delay on first request
2. **TestDomainRateLimiter_SecondRequestWithinInterval** - Enforces delay
3. **TestDomainRateLimiter_SecondRequestAfterInterval** - No delay
4. **TestDomainRateLimiter_ConcurrentSameDomain** - Serializes correctly
5. **TestDomainRateLimiter_ConcurrentDifferentDomains** - Parallel execution
6. **TestDomainRateLimiter_Cleanup** - Removes stale entries
7. **TestDomainRateLimiter_CleanupPreservesRecent** - Keeps active domains

### Integration Tests

**Manual Testing:**

1. **Create job for everlane.com** (known 429 domain)
   - Verify: Zero 429 errors
   - Verify: Max 1 req/second in logs
   - Check: `grep "everlane.com" logs | timestamps` shows 1s intervals

2. **Create job for 100 different domains**
   - Verify: High throughput (100+ req/sec total)
   - Verify: No single domain gets >1 req/sec
   - Check: Memory usage remains low (<10 MB for rate limiter map)

3. **Create job for domain with Crawl-delay: 5**
   - Verify: Requests spaced 5 seconds apart
   - Check logs: "Waiting Xs before request to domain-x.com"

## Deployment Impact

**Risk Level:** LOW

**Breaking Changes:** None (only improves rate limiting)

**Performance Impact:**

- **Positive**: Eliminates HTTP 429 errors (no retry waste)
- **Neutral**: Slightly slower for same-domain tasks (now properly serialized)
- **Memory**: +1-10 MB for rate limiter map (negligible)

**Rollback Plan:**

1. Remove `domainRateLimiter.Wait()` call from `processTask()`
2. Restart application

## Success Metrics

**Before:**

- everlane.com: HTTP 429 after 26 tasks, 2 retries without backoff
- Requests to same domain: 10 concurrent possible

**After:**

- everlane.com: Zero HTTP 429 errors, 1 req/second enforced
- Requests to same domain: Strictly serialized (1 req/interval)
- Memory overhead: <10 MB for 10,000 active domains

**Monitoring:**

- Track HTTP 429 error rate (should drop to near-zero)
- Monitor domain rate limiter map size (should stay <10k entries)
- Measure avg tasks/second (should increase 3-5x for multi-domain crawls)
- Alert if any domain exceeds configured rate (indicates bug)

## Code Locations

### Files to Create

- `internal/jobs/domain_rate_limiter.go` - Core rate limiter
- `internal/jobs/domain_rate_limiter_test.go` - Unit tests

### Files to Modify

- `internal/jobs/worker.go` - Add domainRateLimiter field and Wait() call

### Lines of Code

- New code: ~120 lines (rate limiter + tests)
- Modified code: ~15 lines (integration points)
- Total: ~135 lines

## Alternative Considered (and Rejected)

### Per-Domain Colly LimitRules

**Why rejected:**

- Requires hardcoding domain list
- Doesn't scale to 1000s of unknown domains
- Static configuration, not dynamic

### Worker Pool Semaphore

**Why rejected:**

- Duplicates Colly's functionality
- More complex (semaphore map management)
- Fights against Colly instead of complementing it
- Race condition risks

**Domain Rate Limiter is superior because:**

- Works for any number of domains (1 to 1 million)
- Simple, clean architecture
- Automatically adapts to robots.txt Crawl-delay
- Universal protection without configuration
