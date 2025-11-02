# Domain Rate Limiter Enhancement

## Goals

- Prevent repeated 429 responses by spacing requests per domain
- Persist adaptive backoff so future jobs honour learned delays
- Adjust job concurrency dynamically based on current backoff

## Reference Behaviour (Current)

- Robots.txt crawl delay stored in `domains.crawl_delay_seconds`
- Each worker calls `applyCrawlDelay(task)` → per-worker sleep only
- `handleTaskError` retries blocking errors (403/429/503) twice with exponential
  backoff sleep (1s, 2s) then fails
- No shared throttle state, no persistent adjustment, no concurrency change

## Requirements (per user request)

1. Shared domain limiter:
   - Respect max(robots delay, learned delay)
   - On 429 streak: increase delay by +1s each time (linear growth) up to cap
     (default 60s)
   - On sustained success: after N (e.g. 20) consecutive successes, probe
     reducing delay by 1s; if 429 recurs immediately, revert and record floor
   - Track `error_streak`, `success_streak`, `adaptive_delay`, `delay_floor`,
     `next_available` (in-memory), `last_success`
2. Persistence:
   - Add domain columns `adaptive_delay_seconds` (default 0) and
     `adaptive_delay_floor_seconds` (default 0) to store learned baseline delays
     between runs and the minimum safe delay discovered
   - Short-term backoff window remains in memory and resets on restart
3. Retry policy (blocking errors only):
   - Allow up to 3 retries for 429/403/503 using limiter-provided `retryAfter`
   - Optional safety: cancel job after large streak (e.g. 20 consecutive 429s
     while delay ≥ 60s) — configurable, default off
   - New requirement: if a job accrues N consecutive task failures (configurable
     via `BBB_JOB_FAILURE_THRESHOLD`, default 20), mark the job failed to avoid
     wasting resources. The streak resets on the next successful task and is
     ignored once the job leaves the worker pool.
4. Concurrency reduction:
   - Base concurrency from job
   - For every +5s increase over baseline delay, reduce effective concurrency by
     1 (min 1)
   - Implement via `jobConcurrencyOverrides`
5. Configuration:
   - Base delay default (e.g. 500ms)
   - Max adaptive delay (60s)
   - Max retries (3)
   - Cancel streak threshold (20 @ delay ≥ 60s, optional)
6. Logging & Metrics:
   - Info log when throttling changes (domain, new delay, error streak)
   - Metric counters for 429s, retries, cancellations, concurrency overrides

## Implementation Outline

1. **Schema Migration**
   - Add columns to `domains` table for learned baseline delay and floor
     ```sql
     ALTER TABLE domains
       ADD COLUMN adaptive_delay_seconds INTEGER NOT NULL DEFAULT 0,
       ADD COLUMN adaptive_delay_floor_seconds INTEGER NOT NULL DEFAULT 0;
     ```
   - short-term backoff (`next_allowed_at`) remains in-memory only

2. **DomainLimiter struct (new file)**
   - In-memory map: domain → struct { baseDelay, adaptiveDelay, delayFloor,
     errorStreak, successStreak, backoffUntil (in-memory), lastPersisted }
   - Methods:
     - `Wait(ctx, jobID, domain, baseDelay)` → sleep until allowed; adjust
       concurrency override
     - `RecordSuccess(domain)` → increment success streak, probe lower delay
       when threshold reached (respecting floor), persist if new delay
       established
     - `Record429(domain)` → increment error streak, increase adaptiveDelay by
       +1s (capped), update delay floor when probing fails, set backoffUntil
     - `Persist(domain)` → debounce writes of adaptiveDelay and delay floor to
       DB

3. **WorkerPool Integration**
   - Hold pointer to DomainLimiter
   - In `processTask`, replace `applyCrawlDelay` with `limiter.Wait`
   - On success / error call `RecordSuccess` / `Record429`

4. **Retry Logic Update**
   - `handleTaskError` (blocking branch only — 429/403/503):
     - On 429: call `limiter.Record429` to get `retryAfter`
     - Requeue task for retry up to 3 times using limiter timing (no per-worker
       sleep)
     - Optional safety (configurable, default off): cancel job after large
       streak (e.g. 20 consecutive 429s while delay ≥ 60s)

5. **Dynamic Concurrency**
   - DomainLimiter returns recommended concurrency reduction for job
   - WorkerPool maintains `jobConcurrencyOverrides` map; when override changes,
     update internal structures
   - Ensure override is restored when adaptive delay drops below thresholds

6. **Persistence Hooks**
   - Extend schema to store both `adaptive_delay_seconds` (current baseline) and
     `adaptive_delay_floor_seconds` (minimum safe delay)
   - On job add (AddJob), load these values to seed limiter
   - Periodic goroutine (or on change) flushes adaptive delay & floor to DB (not
     backoffUntil)

7. **Testing**
   - Unit tests for DomainLimiter transitions
   - Integration tests with mocked crawler returning 429 to ensure delay
     increments & concurrency reduces
