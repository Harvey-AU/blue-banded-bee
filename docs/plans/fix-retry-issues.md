# Batch Manager & Worker Pool Updates

## Overview

Here's the full rundown of what changed and why:

## Batch Manager Hardening

Every batch flush path now runs under a fresh **30s context**. That covers:

- The routine timer-based flush
- The MaxBatchSize flush
- The shutdown flush
- Both poison-pill fallbacks

If PostgreSQL stalls (locks, timeouts, etc.), the batch goroutine gets released
after 30s instead of wedging the entire worker pool.

**Shutdown behaviour** mirrors the live path: it retries with the bounded
context, and if all retries fail, individual updates each get their own timeout
so we can still isolate corruption without hanging.

## DbQueue Context Propagation

Added `ExecuteWithContext` that mirrors the original retry/back-off logic but
passes the context with any injected timeout straight into the callback.

### Key Details

- **Semaphore slots** are released immediately after each attempt, so retries
  can continue
- Failures call `waitForRetry`, reusing the existing exponential backoff and
  respecting cancellation
- `executeOnceWithContext` is a companion to `executeOnce`, passing the bounded
  context into every `ExecContext`/`QueryContext`
- Existing call sites that need the new semantics (notably
  `DecrementRunningTasks`) now go through `ExecuteWithContext`, so freeing job
  capacity and promoting waiting tasks cannot block forever
- All DbQueue mocks in the tests were extended to implement the new method, so
  unit tests still compile and behave as before

## Worker Resiliency

`DecrementRunningTasks` uses `ExecuteWithContext`, meaning:

- The decrement + promotion happens inside a bounded **30s window**
- Any pool pressure triggers the same retries/backoffs as other transactional
  operations

## Logging Quality-of-Life

Added a `--log-level` CLI flag that overrides `LOG_LEVEL`. Production operators
can toggle debug logs without touching env vars or redeploying.

## Testing

Ran `go test ./...` (includes API, auth, cache, crawler, db, jobs, mocks, util
packages).

## Summary

Together, the queue no longer has an unbounded SQL path, workers release
resources even if PostgreSQL glitches, and we can turn on debug logging from the
command line when investigating live issues.
