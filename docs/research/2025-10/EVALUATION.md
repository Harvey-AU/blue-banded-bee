# Reference Articles Evaluation

**Project**: Blue Banded Bee - Cache Warming Service **Evaluation Date**: 12
October 2025 **Evaluator**: System Analysis

## Executive Summary

This document evaluates recommendations from 10 reference articles against Blue
Banded Bee's architecture, current implementation, and roadmap.

### Scoring System (0-5 scale)

- **Relevance**: How applicable to our cache warming service with worker pools
- **Current**: Extent already implemented (0 = not at all, 5 = fully done)
- **Impact**: Potential benefit if implemented
- **Effort**: Work required to implement (0 = trivial, 5 = major)

---

## Priority 5 Recommendations (Must Do)

Sorted by Impact/Effort ratio (descending - highest value first).

| Article | Status | Concept                   | Rel | Cur | Imp | Eff | Pri | Summary                                                                  | Application Examples                                                                           |
| ------- | ------ | ------------------------- | --- | --- | --- | --- | --- | ------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------- |
| 5       | ✅     | Profile before optimising | 5   | 1   | 5   | 1   | 5   | Enable pprof HTTP endpoints - optimise based on data not assumptions     | • `/debug/pprof/*` exported via auth-protected handlers<br>• Requires system admin credentials |
| 6       | ✅     | pprof profiling           | 5   | 0   | 4   | 1   | 5   | Built-in CPU/memory profiling - needs full HTTP exposure                 | • `/debug/pprof/*` endpoints available behind system-admin auth                                |
| 9       | ⚪     | pg_stat_statements        | 5   | 0   | 5   | 1   | 5   | Enable PostgreSQL extension - identify slow queries with production data | • Enable extension in Supabase<br>• Query top 20 by total_exec_time<br>• Review monthly        |
| 8       | ⚪     | index_advisor extension   | 5   | 0   | 5   | 1   | 5   | Test virtual indexes before creating                                     | • Enable in Supabase dashboard<br>• Test slow queries<br>• Create indexes with proof           |
| 8       | ⚪     | Query Performance Advisor | 5   | 1   | 4   | 1   | 5   | Built-in Supabase dashboard tool - automated index suggestions           | • Check Supabase dashboard weekly<br>• Review suggestions<br>• Apply high-impact indexes       |
| 7       | ✅     | Timeout strategy          | 5   | 3   | 4   | 2   | 5   | Add idle_in_transaction_session_timeout - prevent zombie transactions    | • Added 30s idle timeout in DSN alongside statement_timeout<br>• Document value in DATABASE.md |
| 7       | ⚪     | Queue limits              | 5   | 1   | 4   | 3   | 5   | Return 429 with Retry-After when pool exhausted - graceful degradation   | • `main.go:227` HTTP limiter exists<br>• Not DB pool-aware<br>• Need pool exhaustion detection |
| 6       | ⚪     | Observability first       | 5   | 1   | 5   | 3   | 5   | Add OpenTelemetry traces + Prometheus metrics - comprehensive visibility | • Add OpenTelemetry traces<br>• Prometheus metrics<br>• Only logging + Sentry currently        |

**Total Priority 5 Items**: 8

---

## Priority 4 Recommendations (Should Do)

Sorted by Impact/Effort ratio (descending - highest value first).

| Article | Status | Concept              | Rel | Cur | Imp | Eff | Pri | Summary                                                                          | Application Examples                                                                                                    |
| ------- | ------ | -------------------- | --- | --- | --- | --- | --- | -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| 6       | 🟠     | Error wrapping (%w)  | 5   | 4   | 3   | 1   | 4   | Wrap errors with fmt.Errorf(%w) - preserve error chain for debugging             | • Audit all error returns<br>• `db.go` wrap SQL errors<br>• ~90 instances found via grep                                |
| 9       | 🟠     | Composite indexes    | 5   | 2   | 5   | 2   | 4   | Index query patterns not columns - some exist, more needed                       | • `tasks(job_id, status, claimed_at)`<br>• `jobs(user_id, status, created_at)`<br>• Test with index_advisor             |
| 4       | ⚪     | Go runtime profiling | 4   | 1   | 4   | 2   | 4   | Profile GC pauses and scheduler latency before optimising                        | • Add GODEBUG=gctrace=1 to staging<br>• Monitor GC pause patterns                                                       |
| 3       | ⚪     | Intelligent logging  | 5   | 1   | 4   | 3   | 4   | Define when to log at each level - currently ad-hoc and inconsistent             | • Document standards in CLAUDE.md<br>• 339 statements but inconsistent severity                                         |
| 7       | 🟠     | Pool sizing formula  | 5   | 3   | 3   | 1   | 3   | Document 2×vCPU or ¼ max_connections formula - tribal knowledge now              | • `db.go:103,155-156` in code comments only<br>• Not in docs/ folder                                                    |
| 8       | ⚪     | Cache hit rate       | 5   | 1   | 4   | 2   | 4   | Target 99% PostgreSQL cache hits - fundamental health metric                     | • Run diagnostic query monthly<br>• Monitor in Supabase Reports<br>• Adjust work_mem                                    |
| 8       | 🟠     | Index usage analysis | 5   | 2   | 4   | 2   | 4   | Find and drop unused indexes - reduce write overhead                             | • Added idx_tasks_running_started_at for stalled task query<br>• Continue periodic `supabase inspect db unused-indexes` |
| 9       | 🟠     | Timeout discipline   | 5   | 2   | 4   | 2   | 4   | Add statement_timeout and idle-in-transaction timeouts - prevent runaway queries | • Add idle_in_transaction_session_timeout<br>• Document in DATABASE.md<br>• Already have statement_timeout              |

**Total Priority 4 Items**: 7

---

## Article 1: 5 Go Design Patterns

**Source**: `5-go-patterns.md` **URL**:
https://codexplorer.medium.com/5-design-patterns-that-transformed-my-go-code-from-chaos-to-clean-df397ac79c23

### Priority Items

- **Observer for job events**: Enable extensible job lifecycle hooks without
  tight coupling - cleanly add notifications, webhooks, analytics [4 impact, 4
  effort, priority 3]

### Recommendations

| Status | Concept           | Rel | Cur | Imp | Eff | Pri | Summary                                                                          | Application Examples                                                                                        |
| ------ | ----------------- | --- | --- | --- | --- | --- | -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
|        | Observer Pattern  | 4   | 0   | 4   | 4   | 3   | Event system for job state changes - add webhooks/notifications without coupling | • `manager.go` job state changes<br>• `worker.go` task completion<br>• Add webhook/notification subscribers |
| ✅     | Strategy Pattern  | 4   | 4   | 2   | 1   | 0   | Swap crawling engines (Colly/Playwright/Selenium) via interface pattern          | • `crawler.go` already has interface<br>• `interfaces.go:11` multiple implementations                       |
| ✅     | Decorator Pattern | 2   | 4   | 2   | 1   | 0   | Wrap HTTP handlers with retry/cache/logging layers                               | • `middleware.go:20` RequestIDMiddleware<br>• `main.go:240` chains CORS, security, rate limiting            |
| ✅     | Adapter Pattern   | 2   | 4   | 1   | 1   | 0   | Wrap external APIs with consistent interface - already done where needed         | • `sentry.go` already adapts Sentry<br>• `db.go` wraps pgx<br>• `handlers.go:45` uses interfaces            |
|        | Composite Pattern | 1   | 0   | 1   | 3   | 0   | Handle nested/tree structures uniformly - not needed for flat URL lists          | Not applicable (flat URL lists, no tree traversal needed)                                                   |

---

## Article 2: 6 Go Libraries (2025)

**Source**: `6-go-libraries.md` **URL**:
https://medium.com/@puneetpm/6-go-libraries-that-completely-transformed-software-development-in-2025-9ebcbf797de3

### Recommendations

| Status | Concept       | Rel | Cur | Imp | Eff | Pri | Summary                                                                 | Application Examples                                                       |
| ------ | ------------- | --- | --- | --- | --- | --- | ----------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| ✅     | Fiber v3      | 2   | 0   | 1   | 4   | 0   | Express-like web framework - stdlib sufficient, high switching cost     | Already using stdlib `net/http` - no benefit to switching (high effort)    |
|        | Ollama Go SDK | 0   | 0   | 0   | 2   | 0   | Run local LLM models - not applicable to cache warming                  | Not applicable (cache warming service, no AI features planned)             |
|        | Templ         | 1   | 0   | 1   | 2   | 0   | Type-safe HTML templating - dashboard is vanilla JS, not needed         | Dashboard uses vanilla JS - no server-side rendering needed                |
|        | Watermill v2  | 2   | 0   | 2   | 4   | 0   | Message broker abstraction - future consideration for event-driven arch | Stage 4+ consideration - current worker pool sufficient                    |
|        | Fx (Uber)     | 2   | 0   | 2   | 3   | 0   | DI framework - adds complexity, current manual wiring is clear          | Current manual DI is simple and clear - adds complexity without clear wins |
|        | Wails v3      | 0   | 0   | 0   | 5   | 0   | Build desktop apps with Go+web - not applicable to web service          | Not applicable (web service, not desktop application)                      |

---

## Article 3: Claude Coding Traps

**Source**: `claude-traps.md` **URL**:
https://generativeai.pub/16-claude-coding-traps-and-the-claude-md-that-fixes-them-e6c344ddf4a4

### Priority Items

- **Intelligent logging standards**: Define INFO/WARN/ERROR criteria -
  inconsistent usage across codebase [4 impact, 2 effort]

### Recommendations

| Status | Concept                  | Rel | Cur | Imp | Eff | Pri | Summary                                                              | Application Examples                                                                                         |
| ------ | ------------------------ | --- | --- | --- | --- | --- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| ⚪     | Intelligent logging      | 5   | 1   | 4   | 3   | 4   | Define when to log at each level - currently ad-hoc and inconsistent | • Document standards in CLAUDE.md<br>• `worker.go` add context<br>• 339 statements but inconsistent severity |
| ✅     | Security best practices  | 5   | 5   | 5   | 3   | 0   | RLS policies, env vars, input validation - already enforced          | Already enforced (very high impact, moderate effort)                                                         |
| ✅     | No placeholders          | 5   | 5   | 4   | 1   | 0   | No YOUR_API_KEY or TODO placeholders - use real config patterns      | Already enforced via CLAUDE.md (high impact, trivial effort)                                                 |
| ✅     | No hardcoded examples    | 4   | 5   | 4   | 1   | 0   | Use variables not example values - prevents prod bugs                | Already enforced (high impact, trivial effort)                                                               |
| ✅     | Evidence-based responses | 4   | 5   | 3   | 1   | 0   | Show actual code when claiming implementation status                 | Already required in CLAUDE.md (moderate impact, trivial effort)                                              |
| ✅     | Preserve requirements    | 5   | 5   | 4   | 1   | 0   | Fix technical bugs not functional requirements                       | Already enforced (high impact, trivial effort)                                                               |
| ✅     | No assumptions           | 4   | 5   | 3   | 1   | 0   | Ask for missing info instead of guessing                             | Already enforced (moderate impact, trivial effort)                                                           |
| ✅     | Question vs code request | 3   | 5   | 3   | 1   | 0   | Answer questions, don't auto-change code                             | Already enforced (moderate impact, trivial effort)                                                           |
| ✅     | Dependency management    | 5   | 5   | 4   | 1   | 0   | Update go.mod when adding imports - automatic via tooling            | Already enforced via go tooling (high impact, trivial effort)                                                |
| ✅     | Clean up code            | 4   | 4   | 3   | 1   | 0   | Remove unused imports, functions, variables                          | Already practised (moderate impact, trivial effort)                                                          |
| ✅     | Capability honesty       | 3   | 5   | 2   | 1   | 0   | Admit limitations instead of faking features                         | Already enforced (low impact, trivial effort)                                                                |

---

## Article 4: Go Latency Reduction

**Source**: `go-latency.md` **URL**:
https://medium.com/@yashbatra11111/we-slashed-our-go-apps-latency-by-80-the-trick-was-wild-f9acba8ed3b8

### Recommendations

| Status | Concept                   | Rel | Cur | Imp | Eff | Pri | Summary                                                      | Application Examples                                                    |
| ------ | ------------------------- | --- | --- | --- | --- | --- | ------------------------------------------------------------ | ----------------------------------------------------------------------- |
|        | Go runtime profiling      | 4   | 2   | 4   | 2   | 4   | Profile GC pauses and scheduler latency before optimising    | • Add GODEBUG=gctrace=1 to staging<br>• Monitor GC pause patterns       |
|        | Cgroup CPU isolation      | 3   | 0   | 3   | 3   | 3   | Dedicate CPU resources via Linux cgroups                     | • Fly.io machine config<br>• Test under load spikes                     |
|        | CPU shares tuning         | 3   | 0   | 3   | 3   | 2   | Prioritise app CPU over system processes                     | • Stage 5+ optimisation<br>• Only if CPU contention observed            |
|        | CFS throttling control    | 2   | 0   | 2   | 4   | 0   | Disable Linux scheduler throttling - very advanced technique | Very advanced - profile first to prove bottleneck                       |
|        | Kernel scheduler analysis | 2   | 0   | 2   | 4   | 0   | Analyse CFS interaction with Go scheduler - research topic   | Stage 6+ research topic - not applicable until scaling to 100+ machines |

---

## Article 5: Go Performance Tips

**Source**: `go-performance.md` **URL**:
https://medium.com/@cleanCompile/10-golang-performance-tips-you-wont-find-in-the-docs-6559665469da

### Priority Items

- **Profile before optimising**: Enable pprof endpoints - make decisions with
  data not guesses [5 impact, 1 effort]
- **Preallocate slices**: Consistent make([]T, 0, cap) usage - small wins in hot
  loops [3 impact, 1 effort]

### Recommendations

| Status | Concept                     | Rel | Cur | Imp | Eff | Pri | Summary                                                                | Application Examples                                                                              |
| ------ | --------------------------- | --- | --- | --- | --- | --- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
|        | sync.Pool for reuse         | 3   | 0   | 4   | 3   | 3   | Object pools for HTTP buffers - reduce GC in high-volume crawling      | • `crawler.go` HTTP buffers<br>• `worker.go` task buffers<br>• `handlers.go` response writers     |
| ✅     | Profile before optimising   | 5   | 1   | 5   | 1   | 5   | Enable pprof HTTP endpoints - optimise based on data not assumptions   | • `/debug/pprof/*` exposed via auth-protected handlers<br>• System admin role required            |
| ✅     | Preallocate slices          | 4   | 4   | 3   | 1   | 3   | Use make([]T, 0, capacity) in loops - avoid reallocation overhead      | • `queue.go:216` batch inserts<br>• `worker.go:115,1262` hot paths covered<br>• 9 instances found |
| 🟠     | Buffered channels           | 4   | 4   | 1   | 1   | 2   | Size channel buffers to reduce goroutine blocking - minor optimisation | • `worker.go:112` notifyCh<br>• Review stopCh if needed                                           |
| 🟠     | Minimise string conversions | 3   | 3   | 2   | 2   | 1   | Cache []byte/string conversions in hot paths - profile first           | • `crawler.go` URL processing<br>• Monitor with pprof                                             |
| ✅     | Minimise goroutines         | 5   | 5   | 5   | 3   | 0   | Worker pool pattern instead of unbounded goroutines - already done     | Already implemented (high impact, moderate effort)                                                |
| ✅     | sync.RWMutex                | 5   | 5   | 4   | 2   | 0   | Read-write lock for read-heavy data - multiple readers one writer      | Already implemented (high impact, low effort)                                                     |
| ✅     | Avoid interface{}           | 3   | 4   | 3   | 1   | 0   | Use concrete types or generics - type safety and performance           | Already good (moderate impact, trivial effort)                                                    |
|        | Reduce JSON overhead        | 2   | 3   | 2   | 3   | 0   | Faster JSON marshalling libraries - must profile to justify            | Not applicable (profile first)                                                                    |
|        | Build tags                  | 1   | 0   | 1   | 2   | 0   | Conditional compilation for platforms - single platform only           | Not applicable (single platform)                                                                  |

---

## Article 6: Microservices Lessons

**Source**: `micro-services.md` **URL**:
https://medium.com/@puneetpm/after-5-years-building-go-microservices-the-5-game-changing-lessons-i-wish-i-knew-earlier-2129929047a3

### Priority Items

- **Expand observability**: Add OpenTelemetry traces + Prometheus metrics - only
  have Sentry errors now [5 impact, 3 effort]

### Recommendations

| Status | Concept                    | Rel | Cur | Imp | Eff | Pri | Summary                                                                  | Application Examples                                                                        |
| ------ | -------------------------- | --- | --- | --- | --- | --- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| ⚪     | Observability first        | 5   | 1   | 5   | 3   | 5   | Add OpenTelemetry traces + Prometheus metrics - comprehensive visibility | • Add OpenTelemetry traces<br>• Prometheus metrics<br>• Only logging + Sentry currently     |
| 🟠     | Error wrapping (%w)        | 5   | 4   | 3   | 1   | 4   | Wrap errors with fmt.Errorf(%w) - preserve error chain for debugging     | • Audit all error returns<br>• `db.go` wrap SQL errors<br>• ~90 instances found via grep    |
| ⚪     | Custom error types         | 4   | 1   | 3   | 3   | 2   | Domain-specific errors with errors.Is/As - type-safe error handling      | • Define domain errors (ErrJobNotFound, ErrTaskLocked)<br>• Currently rely on sql.ErrNoRows |
| ✅     | Structured concurrency     | 5   | 5   | 5   | 3   | 0   | Context + WaitGroup + channels for goroutine management - already done   | Already implemented (very high impact, moderate effort)                                     |
| ✅     | Simplicity over complexity | 5   | 5   | 4   | 2   | 0   | Prefer stdlib over dependencies - keep codebase maintainable             | Already practised (high impact, low effort)                                                 |
| 🟠     | Static binaries            | 5   | 4   | 4   | 2   | 1   | FROM scratch Docker images - minimal attack surface and size             | • `Dockerfile:16` CGO=0 but uses alpine:3.19 base<br>• Not truly static (needs ca-certs)    |
| ✅     | pprof profiling            | 5   | 0   | 4   | 1   | 5   | Built-in CPU/memory profiling - needs full HTTP exposure                 | • `/debug/pprof/*` endpoints available behind system-admin auth                             |
| ✅     | Race detection             | 5   | 5   | 5   | 1   | 0   | go test -race in CI - catch concurrency bugs early                       | Already run in CI (very high impact, trivial effort)                                        |

---

## Article 7: Postgres Connection Pooling

**Source**: `postgres-pool.md` **URL**:
https://medium.com/@Nexumo_/7-postgres-pool-fixes-for-sudden-traffic-spikes-f54d149d1036

### Priority Items

- **Timeout strategy**: Add idle_in_transaction_session_timeout - prevent zombie
  transactions [4 impact, 2 effort]
- **Queue limits & backpressure**: Return 429 with Retry-After when pool full -
  graceful degradation [4 impact, 2 effort]

### Recommendations

| Status | Concept                  | Rel | Cur | Imp | Eff | Pri | Summary                                                                | Application Examples                                                                                |
| ------ | ------------------------ | --- | --- | --- | --- | --- | ---------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| 🟠     | Timeout strategy         | 5   | 2   | 4   | 2   | 5   | Add idle_in_transaction_session_timeout - prevent zombie transactions  | • `db.go:115` has statement_timeout<br>• Missing idle_in_transaction_session_timeout (critical gap) |
| ⚪     | Queue limits             | 5   | 1   | 4   | 3   | 5   | Return 429 with Retry-After when pool exhausted - graceful degradation | • `main.go:227` HTTP limiter exists<br>• Not DB pool-aware<br>• Need pool exhaustion detection      |
| 🟠     | Pool sizing formula      | 5   | 3   | 3   | 1   | 3   | Document 2×vCPU or ¼ max_connections formula - tribal knowledge now    | • `db.go:103,155-156` in code comments only<br>• Not in docs/ folder                                |
| 🟠     | Small transactions       | 5   | 3   | 3   | 2   | 3   | Minimise transaction scope - release locks faster                      | • Batch flushing at `worker.go:1008`<br>• Some long txns remain<br>• Needs profiling                |
|        | Transaction pooling      | 4   | 0   | 4   | 4   | 2   | PgBouncer transaction mode - connection multiplexing                   | Fly.io infra change + Supabase config (Stage 5+)                                                    |
| 🟠     | Prepared statements      | 3   | 3   | 2   | 3   | 1   | Balance caching vs statement churn - profile first                     | Monitor with pprof (no action needed now)                                                           |
| ✅     | App-side concurrency cap | 5   | 5   | 4   | 2   | 0   | Hard limit on concurrent workers - 25 max connections                  | Already implemented (high impact, low effort)                                                       |
|        | Read/write pool split    | 2   | 0   | 3   | 4   | 0   | Separate read/write connection pools - future scaling                  | Stage 6 consideration (needs Supabase Pro)                                                          |

---

## Article 8: Supabase Query Performance

**Source**: `supabase-optimise-db.md` **URL**:
https://supabase.com/docs/guides/troubleshooting/steps-to-improve-query-performance-with-indexes-q8PoC9

### Priority Items

- **index_advisor extension**: Test virtual indexes before creating - zero-cost
  validation [5 impact, 1 effort]
- **Query Performance Advisor**: Built-in Supabase dashboard tool - check weekly
  for suggestions [4 impact, 1 effort]

### Recommendations

| Status | Concept                   | Rel | Cur | Imp | Eff | Pri | Summary                                                        | Application Examples                                                                              |
| ------ | ------------------------- | --- | --- | --- | --- | --- | -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
|        | index_advisor extension   | 5   | 0   | 5   | 1   | 5   | Test virtual indexes before creating - zero-cost validation    | • Enable in Supabase dashboard<br>• Test slow queries<br>• Create indexes with proof              |
|        | Query Performance Advisor | 5   | 1   | 4   | 1   | 5   | Built-in Supabase dashboard tool - automated index suggestions | • Check Supabase dashboard weekly<br>• Review suggestions<br>• Apply high-impact indexes          |
|        | Cache hit rate            | 5   | 0   | 4   | 2   | 4   | Target 99% PostgreSQL cache hits - fundamental health metric   | • Run diagnostic query monthly<br>• Monitor in Supabase Reports<br>• Adjust work_mem              |
|        | Index usage analysis      | 5   | 1   | 4   | 2   | 4   | Find and drop unused indexes - reduce write overhead           | • `supabase inspect db unused-indexes`<br>• Drop unused indexes<br>• Profile with EXPLAIN         |
|        | CONCURRENTLY modifier     | 5   | 0   | 3   | 1   | 3   | Create indexes without blocking writes - production safety     | • Use for all production indexes<br>• Add to migration template<br>• Document in DATABASE.md      |
|        | Grafana metrics           | 4   | 0   | 4   | 3   | 3   | Real-time database monitoring dashboard - visibility           | • Deploy Supabase Grafana (Fly.io)<br>• Track connections, queries, cache<br>• Alert on anomalies |
|        | GIN/GIST indexes          | 2   | 0   | 2   | 2   | 1   | Specialised indexes for JSON/ARRAY columns - not needed yet    | Not currently needed (no complex JSON queries)                                                    |
|        | HNSW indexes              | 0   | 0   | 0   | 2   | 0   | Vector similarity search indexes - not applicable              | Not applicable (no vector/AI features)                                                            |

---

## Article 9: Supabase Performance Habits

**Source**: `supabase-speed.md` **URL**:
https://medium.com/@kaushalsinh73/8-supabase-postgres-habits-for-startup-speed-backends-9acbff48f0aa

### Priority Items

- **pg_stat_statements**: Enable extension - identify slow queries with
  production data [5 impact, 1 effort]
- **Composite indexes**: Index query patterns not columns - task claiming needs
  (job_id, status, claimed_at) [5 impact, 2 effort]

### Recommendations

| Status                                                                  | Concept                | Rel | Cur | Imp | Eff | Pri | Summary                        | Application Examples                                                                                        |
| ----------------------------------------------------------------------- | ---------------------- | --- | --- | --- | --- | --- | ------------------------------ | ----------------------------------------------------------------------------------------------------------- |
|                                                                         | pg_stat_statements     | 5   | 0   | 5   | 1   | 5   | Identify slow queries          | • Enable extension in Supabase<br>• Query top 20 by total_exec_time<br>• Review monthly                     |
| 🟠                                                                      | Composite indexes      | 5   | 2   | 5   | 2   | 4   | Match query patterns           | • `tasks(job_id, status, claimed_at)`<br>• `jobs(user_id, status, created_at)`<br>• Test with index_advisor |
| 🟠                                                                      | Timeout discipline     | 5   | 2   | 4   | 2   | 4   | statement_timeout, idle-in-tx  | • Add idle_in_transaction_session_timeout<br>• Document in DATABASE.md<br>• Already have statement_timeout  |
| ✅                                                                      | Partial indexes        | 4   | 4   | 4   | 2   | 3   | WHERE clauses for sparse data  | • `initial_schema.sql:140` idx_tasks_pending_claim_order EXISTS<br>• `WHERE status = 'pending'` implemented |
| • Page creation now uses DO NOTHING + SELECT to avoid redundant updates |
|                                                                         | Covering indexes       | 4   | 0   | 3   | 2   | 3   | INCLUDE to avoid heap lookups  | • Add INCLUDE (url) to task indexes<br>• Avoid second lookup for hot queries<br>• Profile before adding     |
|                                                                         | Views for joined APIs  | 3   | 0   | 3   | 3   | 2   | Pre-aggregate for UI           | • v_job_summary (tasks count, progress %)<br>• Dashboard endpoints<br>• Stage 3+ feature                    |
|                                                                         | RPC functions          | 3   | 0   | 2   | 3   | 1   | One round trip for complex ops | • Consider for job creation + URL discovery<br>• Reduce round trips<br>• Test vs current approach           |
| ✅                                                                      | RLS as product feature | 5   | 5   | 5   | 3   | 0   | Design policies from day 1     | Already implemented (very high impact, moderate effort)                                                     |
|                                                                         | JSONB with discipline  | 2   | 1   | 2   | 3   | 0   | Generated columns + GIN        | Minimal JSONB usage - not needed yet                                                                        |
|                                                                         | pg_cron + outbox       | 3   | 0   | 3   | 4   | 0   | Reliable background jobs       | Stage 4+ - current worker pool handles async work                                                           |

---

## Article 10: Top 10 Go Libraries

**Source**: `top-10-go-libraries.md` **URL**:
https://blog.stackademic.com/top-10-go-libraries-every-developer-should-know-in-2025-bd4020f98eb9

### Priority Items

- **GoQuery**: jQuery-like HTML parsing - alternative to Colly if static site
  needs arise [3 impact, 2 effort]
- **Cobra CLI**: Build admin CLI tools - future `blue-banded-bee migrate`,
  `seed` commands [2 impact, 3 effort]

### Recommendations

| Status | Concept       | Rel | Cur | Imp | Eff | Pri | Summary                                                                     | Application Examples                                                            |
| ------ | ------------- | --- | --- | --- | --- | --- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| ✅     | Testify       | 5   | 5   | 5   | 1   | 0   | Assertion library with test suites - comprehensive testing framework        | Already using extensively (very high impact, trivial effort)                    |
| ✅     | time package  | 5   | 5   | 4   | 0   | 0   | Standard library time/timezone handling - built-in and sufficient           | Already using stdlib (high impact, zero effort)                                 |
| ✅     | GoQuery       | 3   | 5   | 3   | 2   | 0   | jQuery-style HTML parsing - Colly alternative for static sites              | • `crawler.go:14` imports goquery<br>• `go.mod:8` production dependency         |
|        | Cobra CLI     | 2   | 0   | 2   | 3   | 1   | CLI builder with subcommands - admin tools for migrations and seeding       | • Stage 4+ admin CLI tools<br>• `blue-banded-bee migrate`, `seed`, etc.         |
| ✅     | Gin framework | 2   | 0   | 1   | 4   | 0   | Fast web framework with middleware - stdlib sufficient, high migration cost | Not needed - stdlib sufficient (low impact, high effort to switch)              |
| ✅     | GORM          | 2   | 0   | 1   | 4   | 0   | ORM with migrations and relations - pgx gives better control                | Not needed - pgx provides control (low impact, high effort)                     |
| ✅     | GoDotEnv      | 2   | 5   | 2   | 0   | 0   | Load .env files into environment - Fly.io secrets handle config             | • `main.go:39` godotenv.Load()<br>• `go.mod:14` production dependency           |
|        | mapstructure  | 1   | 0   | 1   | 1   | 0   | Decode maps into structs - JSON unmarshal handles this already              | Not needed - direct JSON unmarshal works fine                                   |
| ✅     | JWT-Go        | 3   | 5   | 3   | 0   | 0   | JWT creation and validation - Supabase Auth handles all JWT ops             | • `auth/middleware.go:13` imports golang-jwt/jwt/v5<br>• `go.mod:11` dependency |
|        | HTTPRouter    | 2   | 0   | 1   | 3   | 0   | Fast HTTP router with path parameters - ServeMux 1.22+ sufficient           | stdlib `net/http` ServeMux 1.22+ sufficient - no benefit to switching           |

---

## Summary: High-Priority Recommendations (Priority ≥ 4)

This table consolidates all recommendations with Priority 4 or 5 from the 10
articles above.

| Article | Concept                   | Pri | Status | Summary                                                                          | Application Examples                                                                                        |
| ------- | ------------------------- | --- | ------ | -------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| 3       | Intelligent logging       | 4   | ⚪     | Define when to log at each level - currently ad-hoc and inconsistent             | • Document standards in CLAUDE.md<br>• 339 statements but inconsistent severity                             |
| 4       | Go runtime profiling      | 4   | ⚪     | Profile GC pauses and scheduler latency before optimising                        | • Add GODEBUG=gctrace=1 to staging<br>• Monitor GC pause patterns                                           |
| 5       | Profile before optimising | 5   | ✅     | Enable pprof HTTP endpoints - optimise based on data not assumptions             | • `/debug/pprof/*` exposed via auth-protected handlers<br>• Requires system admin credentials               |
| 6       | Observability first       | 5   | ⚪     | Add OpenTelemetry traces + Prometheus metrics - comprehensive visibility         | • Add OpenTelemetry traces<br>• Prometheus metrics<br>• Only logging + Sentry currently                     |
| 6       | pprof profiling           | 5   | ✅     | Built-in CPU/memory profiling - needs full HTTP exposure                         | • `/debug/pprof/*` endpoints available behind system-admin auth                                             |
| 6       | Error wrapping (%w)       | 4   | 🟠     | Wrap errors with fmt.Errorf(%w) - preserve error chain for debugging             | • Audit all error returns<br>• ~90 instances found via grep                                                 |
| 7       | Timeout strategy          | 5   | 🟠     | Add idle_in_transaction_session_timeout - prevent zombie transactions            | • `db.go:115` has statement_timeout<br>• Missing idle_in_transaction_session_timeout (critical gap)         |
| 7       | Queue limits              | 5   | ⚪     | Return 429 with Retry-After when pool exhausted - graceful degradation           | • `main.go:227` HTTP limiter exists<br>• Not DB pool-aware<br>• Need pool exhaustion detection              |
| 8       | index_advisor extension   | 5   | ⚪     | Test virtual indexes before creating                                             | • Enable in Supabase dashboard<br>• Test slow queries<br>• Create indexes with proof                        |
| 8       | Query Performance Advisor | 5   | ⚪     | Built-in Supabase dashboard tool - automated index suggestions                   | • Check Supabase dashboard weekly<br>• Review suggestions<br>• Apply high-impact indexes                    |
| 8       | Cache hit rate            | 4   | ⚪     | Target 99% PostgreSQL cache hits - fundamental health metric                     | • Run diagnostic query monthly<br>• Monitor in Supabase Reports<br>• Adjust work_mem                        |
| 8       | Index usage analysis      | 4   | ⚪     | Find and drop unused indexes - reduce write overhead                             | • `supabase inspect db unused-indexes`<br>• Drop unused indexes<br>• Profile with EXPLAIN                   |
| 9       | pg_stat_statements        | 5   | ⚪     | Enable PostgreSQL extension - identify slow queries with production data         | • Enable extension in Supabase<br>• Query top 20 by total_exec_time<br>• Review monthly                     |
| 9       | Composite indexes         | 4   | 🟠     | Index query patterns not columns - some exist, more needed                       | • `tasks(job_id, status, claimed_at)`<br>• `jobs(user_id, status, created_at)`<br>• Test with index_advisor |
| 9       | Timeout discipline        | 4   | 🟠     | Add statement_timeout and idle-in-transaction timeouts - prevent runaway queries | • Add idle_in_transaction_session_timeout<br>• Document in DATABASE.md<br>• Already have statement_timeout  |

**Total High-Priority Items**: 15 (8 Priority-5 must-dos, 7 Priority-4
should-dos)

---

## Evaluation Progress

- [x] Article 1: 5 Go Design Patterns
- [x] Article 2: 6 Go Libraries (2025)
- [x] Article 3: Claude Coding Traps
- [x] Article 4: Go Latency Reduction
- [x] Article 5: Go Performance Tips
- [x] Article 6: Microservices Lessons
- [x] Article 7: Postgres Connection Pooling
- [x] Article 8: Supabase Query Performance
- [x] Article 9: Supabase Performance Habits
- [x] Article 10: Top 10 Go Libraries

---

## Blue Banded Bee Context (For Reference)

### Current Architecture

- **Language**: Go 1.25
- **Database**: PostgreSQL (Supabase)
- **Deployment**: Fly.io with Cloudflare CDN
- **Auth**: Supabase Auth (JWT)
- **Monitoring**: Sentry (errors + performance)
- **Testing**: 350+ tests, 38.9% coverage, testify framework

### Key Components

- Worker pool with concurrent task processing
- FOR UPDATE SKIP LOCKED for lock-free queuing
- Connection pooling (25 max open, 10 idle)
- Goroutine-based concurrency with context
- RESTful API with middleware
- Batch operations for efficiency

### Current Patterns

- Extract + Test + Commit refactoring methodology
- Function size < 50 lines
- Table-driven tests
- Error wrapping with context
- Sentry for critical failures only

### Known Characteristics

- High-concurrency crawling workload
- Burst traffic during job starts
- Database-heavy operations
- External HTTP requests to target sites
- Multi-tenant with RLS policies
