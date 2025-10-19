# Issue #4: Cookie Jar Support

**Priority:** ⚠️ **LOW** - Not recommended for cache warming
**Cost:** Medium complexity, security risks
**Status:** Not needed for current use case

## Problem Statement

HTTP cookie jars maintain session state across requests by storing cookies sent by servers and automatically including them in subsequent requests to the same domain.

## Current Architecture Reality

**Critical fact:** All workers share a **single Crawler instance** with **one HTTP client**.

```go
// cmd/app/main.go
cr := crawler.New(crawlerConfig)  // Single shared crawler
workerPool := jobs.NewWorkerPool(pgDB.GetDB(), dbQueue, cr, jobWorkers, ...)
// ↑ All workers share this same crawler instance

// internal/jobs/worker.go
type WorkerPool struct {
    crawler CrawlerInterface  // Single shared crawler
    numWorkers int            // 50 worker goroutines
}
```

**Architecture:**
- 1 Crawler → 1 HTTP client → 1 cookie jar (if enabled)
- 50 workers → all share the same cookie jar
- Workers switch jobs constantly (any worker can process any job's tasks)

## Why Cookie Jars Are Problematic

### 1. Cross-Job Cookie Leakage (Security Risk)

**Scenario:**
```
Worker 1 processes job-A (customer-1.webflow.io) → receives session cookie
Worker 2 processes job-B (customer-2.webflow.io) → REUSES Worker 1's cookie
Result: Worker 2 sees customer-1's session data
```

**This is a data leak** if jobs belong to different users/accounts.

### 2. No Benefit for Cache Warming

Cache warming doesn't require session state:
- We're hitting public URLs
- CDN caching is independent of cookies
- Session cookies are for logged-in content (not our use case)

### 3. Memory Overhead (Minimal but Unnecessary)

**Realistic estimate:**
- Cookie jar stores cookies **per domain** (not per page)
- example.com sets 1 session cookie → stored once
- 10,000 pages on example.com → still 1 session cookie
- **Realistic:** 20 domains × 5 cookies × 1KB = **100 KB total**

**NOT** 400 MB as incorrectly calculated (that assumed per-page storage).

### 4. Cleanup Complexity

Would require:
- Per-job cookie jar isolation
- Clear cookies when switching jobs
- Detect job transitions in worker loop
- Additional locking/synchronisation

## If You Absolutely Need Cookies (You Don't)

### Architecture Option 1: Per-Job Crawler (Complex)

**Create separate Crawler instance per job:**

```go
type WorkerPool struct {
    jobCrawlers map[string]*crawler.Crawler  // jobID → dedicated crawler
    crawlerMutex sync.RWMutex
}
```

**Pros:**
- True job isolation
- Cookies scoped to job

**Cons:**
- Need to create/destroy crawlers dynamically
- Higher memory (1 HTTP client per active job)
- More complex lifecycle management

**Memory:** 20 jobs × (HTTP client + cookie jar) ≈ 20 MB

### Architecture Option 2: Manual Cookie Management (Error-Prone)

**Extract cookies from responses, store per-job, inject into requests:**

```go
type JobCookies struct {
    cookies map[string][]*http.Cookie  // domain → cookies
    mu      sync.RWMutex
}
```

**Pros:**
- Full control over cookie lifecycle

**Cons:**
- Reimplementing cookiejar.Jar logic
- Easy to get wrong (cookie expiry, domain matching, etc.)
- Doesn't leverage Colly's built-in mechanisms

## Recommendation

### ❌ **DO NOT IMPLEMENT**

**Reasons:**
1. **No benefit** - Cache warming doesn't need session state
2. **Security risk** - Cross-job cookie leakage with shared crawler
3. **Complexity** - Would require per-job crawler isolation
4. **Alternative exists** - If you need authenticated crawling, create separate jobs with auth headers

### ✅ **Alternative: Use Request Headers Instead**

If you need to crawl authenticated content:

```go
// Set custom headers per request
c.OnRequest(func(r *colly.Request) {
    if authToken := getJobAuthToken(jobID); authToken != "" {
        r.Headers.Set("Authorization", "Bearer " + authToken)
    }
})
```

**Benefits:**
- Explicit auth control
- No session leakage
- Simpler architecture
- Per-job isolation without separate crawlers

## Implementation Complexity

**If forced to implement:**
- Per-job crawler instances: ~200 lines (lifecycle management)
- Cookie cleanup logic: ~50 lines
- Tests: ~100 lines
- **Total: ~350 lines of complex, error-prone code**

**For minimal benefit in cache warming use case.**

## Cost-Benefit Analysis

| Aspect | Cost | Benefit |
|--------|------|---------|
| Development time | 2-3 days | None for cache warming |
| Code complexity | High (per-job crawler lifecycle) | None |
| Memory overhead | 20 MB (20 jobs) | None |
| Security risk | High (cookie leakage) | None |
| Maintenance burden | Medium (cleanup, testing) | None |

**Verdict:** ❌ Not worth implementing

## Related Issues

- **Issue #3 (Domain Rate Limiter)** - Solves rate limiting without cookies
- **Issue #1 (Cache Warming Timeout)** - Primary performance issue (cookies irrelevant)
