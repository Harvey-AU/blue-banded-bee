# Blue Banded Bee - Comprehensive Application Review

**Date:** January 2025 **Scope:** End-to-end architecture, logic paths,
strengths, and areas for improvement

---

## Executive Summary

Blue Banded Bee is a **well-architected web cache warming service** built in Go
with a PostgreSQL (Supabase) backend. The codebase demonstrates mature
engineering practices: clean separation of concerns, robust error handling, and
thoughtful concurrency design. However, there are notable gaps in test coverage
for critical business logic and some unbounded resource growth patterns that
need attention before scaling.

**Overall Assessment: 8/10** - Production-ready with room for improvement in
testing and observability.

---

## 1. Architecture Overview

### What It Does

A cache warming and site health monitoring service focused on Webflow sites:

- **Cache Warming**: Warms CDN caches after publishing with retry on cache
  misses
- **Site Health**: Detects broken links (404s), slow pages, and tracks
  performance
- **Integrations**: Webflow OAuth, Google Analytics 4, Slack notifications
- **Automation**: Scheduled crawls (6/12/24/48h intervals), webhook-triggered
  crawls

### System Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (Vanilla JS)                                       │
│  dashboard.html + bb-data-binder.js + auth.js               │
└────────────────────────┬────────────────────────────────────┘
                         │ REST API + Supabase Realtime
┌────────────────────────▼────────────────────────────────────┐
│  API Layer (internal/api/)                                   │
│  Jobs, Auth, Integrations, Dashboard endpoints              │
│  Middleware: CORS, Rate Limit, JWT Auth, Request ID         │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Job System (internal/jobs/)                                 │
│  JobManager: Creation, lifecycle, URL discovery             │
│  WorkerPool: Task claiming, concurrent processing           │
│  DomainLimiter: Adaptive rate limiting per domain           │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Crawler (internal/crawler/)                                 │
│  HTTP requests, sitemap parsing, robots.txt, CDN detection  │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Database (Supabase PostgreSQL)                             │
│  66 migrations, RLS policies, Vault for secrets             │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Core Logic Paths

### Job Creation → Task Processing → Completion

```
1. API POST /v1/jobs
   └─► JobManager.CreateJob()
       ├─► Check for duplicate active jobs on same domain
       ├─► Insert job record (status: pending)
       ├─► Discover sitemap URLs (background goroutine)
       │   ├─► Fetch robots.txt
       │   ├─► Parse sitemap.xml (with fallbacks)
       │   └─► Filter URLs against robots.txt rules
       └─► Enqueue discovered URLs as tasks

2. Worker Pool (20-50 workers)
   └─► claimPendingTask()
       ├─► SELECT ... FOR UPDATE SKIP LOCKED (lock-free claiming)
       ├─► Check domain rate limit via DomainLimiter.Acquire()
       └─► Process task:
           ├─► crawler.WarmURL() - first request (cold cache)
           ├─► Second request (verify cache populated)
           ├─► Extract links if find_links=true
           ├─► Record: status, response time, cache status, performance breakdown
           └─► Batch update to database (every 50ms or 4 tasks)

3. Completion Detection
   └─► Monitor checks: completed + failed + skipped >= total
       ├─► Update job status to "completed"
       ├─► Calculate duration and average time per task
       └─► Send Slack notification (if configured)
```

### Adaptive Rate Limiting (Standout Feature)

```
DomainLimiter tracks per-domain:
├─► baseDelay (from robots.txt crawl-delay)
├─► adaptiveDelay (increases on 429/503, decreases on success streak)
├─► errorStreak / successStreak counters
└─► Concurrency reduction: -1 slot per 5s above base delay

On 429/503 error:
├─► Increment adaptiveDelay by 500ms (up to 60s max)
├─► Set backoff window
└─► Reduce concurrent requests to domain

On 5+ consecutive successes:
├─► Probe: reduce delay by 500ms
├─► If probe fails: revert and set floor
└─► Gradually recover throughput
```

---

## 3. Strengths

### Architecture & Design

| Strength                            | Evidence                                                                         |
| ----------------------------------- | -------------------------------------------------------------------------------- |
| **Clean separation of concerns**    | 15 internal packages with clear responsibilities, no circular dependencies       |
| **Interface-first design**          | Each package defines interfaces for testability (`DBClient`, `CrawlerInterface`) |
| **Lock-free concurrency**           | `FOR UPDATE SKIP LOCKED` eliminates worker contention                            |
| **Adaptive rate limiting**          | Self-tuning algorithm learns domain limits without manual config                 |
| **Multi-level concurrency control** | Per-worker, per-job, per-domain limits prevent resource exhaustion               |
| **Generated columns**               | `jobs.duration_seconds` computed in DB, not application                          |
| **Batch operations**                | Task updates debounced (100 max, 2s flush) to reduce DB round-trips              |

### Database & Supabase Integration (9/10)

| Strength                   | Evidence                                                          |
| -------------------------- | ----------------------------------------------------------------- |
| **Migration discipline**   | 66 timestamped migrations, all schema changes via Supabase        |
| **RLS everywhere**         | All tables have Row Level Security with multi-org support         |
| **Vault for secrets**      | Slack/Google/Webflow tokens encrypted in Supabase Vault           |
| **Pool tuning**            | Environment-aware: 70 max (prod) → 5 max (staging) → 2 max (dev)  |
| **Optimised indexes**      | `idx_tasks_claim_optimised` eliminated 50-70% latency on hot path |
| **Trigger-based counters** | Job stats updated O(1) incrementally via `update_job_counters()`  |

### Error Handling & Resilience

| Strength                   | Evidence                                                              |
| -------------------------- | --------------------------------------------------------------------- |
| **Retry classification**   | Blocking (429/503) vs retryable (timeout/5xx) vs terminal (4xx)       |
| **Stale task recovery**    | Tasks stuck >3 min reset to pending (runs every 1 min)                |
| **Counter reconciliation** | Fixes running_tasks drift on startup and after recovery               |
| **Panic recovery**         | `processNextTask` recovers panics and logs to Sentry                  |
| **Graceful shutdown**      | Coordinated drain of worker pool, background services, DB connections |

### API Design (8/10)

| Strength                     | Evidence                                                   |
| ---------------------------- | ---------------------------------------------------------- |
| **Standardised responses**   | All endpoints return `{status, data, message, request_id}` |
| **Request tracing**          | Unique `X-Request-ID` on every request for correlation     |
| **Rate limiting**            | Per-IP: 20 req/sec with 10-request burst                   |
| **CSRF protection**          | HMAC-signed OAuth state, Cross-Origin middleware           |
| **Pool saturation handling** | Returns 429 with `Retry-After` when DB overwhelmed         |

### Code Quality

| Strength                           | Evidence                                                         |
| ---------------------------------- | ---------------------------------------------------------------- |
| **Refactoring methodology**        | Extract + Test + Commit pattern yielded 80% complexity reduction |
| **Cyclomatic complexity enforced** | Max 35, linter blocks merge                                      |
| **Australian English enforced**    | `misspell` linter with AU locale                                 |
| **Structured logging**             | Zerolog with contextual fields (job_id, request_id)              |

---

## 4. Weaknesses & Concerns

### Critical: Test Coverage Gaps

| Area             | Coverage   | Risk                            |
| ---------------- | ---------- | ------------------------------- |
| `internal/jobs/` | **13.8%**  | Core business logic undertested |
| `internal/db/`   | **0.5%**   | Database layer nearly untested  |
| `internal/api/`  | Fragmented | Integration tests incomplete    |

**Root cause**: `WorkerPool` uses concrete types instead of interfaces,
preventing proper mocking. Documented in code but not yet fixed.

### Memory Growth (Unbounded Caches)

| Cache                   | Location                | Cleanup Mechanism  | Risk Level   |
| ----------------------- | ----------------------- | ------------------ | ------------ |
| `DomainLimiter.domains` | `domain_limiter.go:98`  | **NONE**           | **CRITICAL** |
| `jobInfoCache`          | `worker.go:115`         | `RemoveJob()` only | HIGH         |
| `jobPerformance`        | `worker.go:111`         | `RemoveJob()` only | MEDIUM       |
| `jobFailureCounters`    | `worker.go:121`         | `RemoveJob()` only | MEDIUM       |
| `priorityUpdateTracker` | `worker.go:126`         | `RemoveJob()` only | MEDIUM       |
| `domainState.jobStates` | `domain_limiter.go:251` | **NONE**           | HIGH         |

**Realistic memory leak scenario:**

- 1,000 unique domains over 6 months of operation
- 10,000 stuck jobs with cached `RobotsRules` (~100KB each)
- **Potential 200-400MB unreclaimable memory**

**Risk**: Long-running server accumulates entries indefinitely. The
`jobInfoCache` and related maps only clean up when `RemoveJob()` is called—stuck
or abandoned jobs leak permanently. The `DomainLimiter.domains` map has no
eviction mechanism whatsoever.

### Hard-Coded Timeouts

```go
TaskStaleTimeout = 3 * time.Minute      // Not configurable
taskProcessingTimeout = 2 * time.Minute  // Not configurable
```

**Risk**: Legitimate long-running tasks (large pages) falsely marked as stuck.

### Rate Limit Detection

The primary rate limit detection mechanism uses HTTP status code comparison
(`worker.go:3778-3785`):

```go
if resp.StatusCode == 429 || resp.StatusCode == 503 {
    // Handle rate limiting
}
```

String matching in error messages (`domain_limiter.go:544-554`) serves as a
**fallback** for cases where errors propagate without structured response data.

**Actual risk**: The coupling between error string format across modules lacks a
formal contract. If error message formatting changes in the crawler, the
fallback detection could silently break. However, the primary HTTP status code
path is robust.

**Recommendation**: Consider defining error types (e.g., `RateLimitError`) to
formalise the contract rather than relying on string matching as fallback.

### Missing Observability

- No metrics for queue depth (pending/waiting task counts)
- Cannot detect queue saturation until scale-up logic triggers
- Hard to diagnose why jobs stall

### Frontend Architecture (Deliberate Trade-offs)

Per CLAUDE.md, the frontend deliberately uses vanilla JavaScript without a build
process. The following are **acknowledged trade-offs**, not oversights:

| Characteristic                                | Trade-off Rationale                                                             |
| --------------------------------------------- | ------------------------------------------------------------------------------- |
| No build process                              | **Deliberate**: Simplicity over optimisation; matches Webflow integration model |
| Dual attribute format (`data-bb-*` + `bbb-*`) | Migration path exists; not blocking current functionality                       |
| 1560-line `auth.js`                           | Consolidates all auth concerns in one place; acceptable for current scale       |

**Future consideration**: If the dashboard grows significantly, modularisation
may become worthwhile. For now, these are reasonable choices for the project's
scope.

---

## 5. Security Assessment

### Strengths

- ✅ RLS on all tables (100% coverage)
- ✅ Secrets in Vault (never in application code)
- ✅ Parameterised queries throughout (no SQL injection)
- ✅ HMAC-signed OAuth state (CSRF protection)
- ✅ JWT validation via JWKS with proper expiry checks
- ✅ Security headers (CSP, HSTS, X-Frame-Options)

### Concerns

- ⚠️ **CORS allows all origins** (`middleware.go:164`) - However,
  defence-in-depth mitigations are in place:
  - Robust CSP headers restrict script/resource origins
  - HSTS enforced with 1-year max-age
  - X-Frame-Options prevents clickjacking
  - All authenticated endpoints require valid JWT
  - Acceptable for a dashboard accessed by authenticated users
- ⚠️ No rate limiting on webhook endpoints
- ⚠️ Share link access has no rate limiting per token
- ⚠️ Slack OIDC trigger commented out (requires manual enablement)

---

## 6. Performance Characteristics

### Database Query Patterns

| Query         | Optimisation                                                            |
| ------------- | ----------------------------------------------------------------------- |
| Task claiming | `FOR UPDATE SKIP LOCKED` + composite index                              |
| Job listing   | Separate indexes for filtered vs unfiltered                             |
| RLS policies  | Subquery wrapping prevents per-row function calls (10,000x improvement) |

### Concurrency Tuning

| Environment | Max Pool | Max Workers | Job Concurrency |
| ----------- | -------- | ----------- | --------------- |
| Production  | 70       | 50          | 20 (default)    |
| Staging     | 5        | 10          | 20              |
| Development | 2-3      | 5           | 20              |

### Bottleneck Analysis

1. **Database pool saturation**: Explicit handling with 429 response
2. **Domain rate limits**: Adaptive algorithm self-tunes
3. **Worker scaling**: Dynamic allocation based on active jobs
4. **Batch writes**: Debounced to reduce transaction overhead

---

## 7. Topics Not Covered in This Review

The following areas were identified but not deeply investigated:

| Topic                       | Gap                                              | Potential Impact                                |
| --------------------------- | ------------------------------------------------ | ----------------------------------------------- |
| **Deployment architecture** | Fly.io configuration and scaling not documented  | Unclear how instances scale, health checks work |
| **Disaster recovery**       | No documented backup/restore procedures          | Data loss risk if Supabase issues occur         |
| **OAuth token refresh**     | Webflow/Google token expiry handling not audited | Potential auth failures after token expiry      |
| **API versioning**          | No deprecation policy for `/v1/` endpoints       | Breaking change risk for API consumers          |
| **Monitoring gaps**         | No runbook for common operational issues         | Slower incident response                        |

These topics warrant separate, focused reviews.

---

## 8. Recommendations (Priority Order)

### High Priority

1. **Increase jobs package test coverage to 50%+**
   - Refactor `WorkerPool` to use interfaces for mocking
   - Add integration tests for job creation → completion flow

2. **Add cache eviction for DomainLimiter** (Critical memory leak)

   ```go
   // Add TTL-based eviction for DomainLimiter.domains
   // Evict domains with no activity > 1 hour
   // Add periodic cleanup goroutine
   ```

3. **Add observability for domain limiter cache size**
   - Expose `len(domains)` as a metric
   - Alert when cache exceeds threshold (e.g., 10,000 domains)

4. **Make timeouts configurable**

   ```bash
   BBB_TASK_PROCESSING_TIMEOUT_SECONDS=120
   BBB_STALE_TASK_TIMEOUT_MINUTES=3
   ```

5. **Add queue depth metrics**
   - Record pending/waiting task counts per job
   - Expose via Prometheus for alerting

### Medium Priority

6. **Implement circuit breaker for DB errors**
   - After N consecutive failures, pause workers and alert
   - Prevents cascading failures

7. **Formalise rate limit error types**
   - Define `RateLimitError` type instead of string matching
   - Primary HTTP status detection is already robust; this improves
     maintainability

8. **Add webhook rate limiting**
   - Per-workspace throttling for Webflow webhooks
   - Prevents abuse from misconfigured publish hooks

9. **Complete Slack OIDC trigger setup**
   - Document manual enablement or automate via migration

10. **Evict jobInfoCache entries on job completion**
    - Currently only cleaned up via `RemoveJob()`
    - Add automatic cleanup when job transitions to completed/failed

### Low Priority

11. **Deprecation roadmap for legacy `organisation_id`**
    - Set timeline to remove `COALESCE` fallback

12. **Document OAuth token refresh behaviour**
    - Audit Webflow/Google token expiry handling
    - Add integration tests for token refresh flows

---

## 9. Summary Scorecard

| Dimension         | Score | Notes                                                                |
| ----------------- | ----- | -------------------------------------------------------------------- |
| **Architecture**  | 9/10  | Clean layers, interface-first, no circular deps                      |
| **Database**      | 9/10  | Excellent Supabase integration, RLS, migrations                      |
| **Worker Pool**   | 7/10  | Robust concurrency, adaptive limiting; **critical unbounded caches** |
| **API**           | 8/10  | Standardised, secure; missing some rate limits                       |
| **Testing**       | 6/10  | Good patterns, poor coverage in critical paths                       |
| **Security**      | 8/10  | RLS, Vault, JWT; CORS permissive but mitigated                       |
| **Observability** | 7/10  | Good logging, Sentry; missing queue/cache metrics                    |
| **Frontend**      | 8/10  | Deliberate simplicity trade-offs appropriate for scope               |
| **Documentation** | 9/10  | Excellent CLAUDE.md, architecture docs                               |

**Overall: 8/10** - Solid production system with clear improvement paths.
Primary concern is unbounded memory growth in long-running deployments.

---

## Conclusion

The codebase reflects disciplined engineering with a Supabase-first philosophy.
The adaptive rate limiting is particularly impressive. Focus testing efforts on
`internal/jobs/` and `internal/db/`, implement cache eviction, and add queue
depth observability to prepare for scale.
