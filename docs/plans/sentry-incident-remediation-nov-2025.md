# Sentry Incident Remediation – November 2025

Date: 2025‑11‑14  
Owner: Platform team

## Overview

This document captures the five production issues surfaced in Sentry over the
last week. For each issue we outline the core problem, current impact, and the
technical plan required to remediate or consciously accept the risk. All actions
assume production environment unless otherwise stated.

## BLUE-BANDED-BEE-40 – Cloudflare Turnstile Error 106010

**Core problem**  
Client-side Turnstile challenges occasionally fail with error `106010`, which is
Cloudflare’s “unknown session / replayed token” failure. Impact is user
login/verify actions aborting on `/dashboard` without retry logic, affecting 2
unique users in the last 24 hours and 8 users lifetime.

**Technical solution**

- **Improve token lifecycle** – Ensure the front-end always calls
  `turnstile.reset()` after form submission or when the widget receives
  cross-tab messages, preventing multiple submissions with stale tokens.
- **Handle 106010 gracefully** – Wrap the Turnstile promise with retry/backoff
  (max 2 attempts) and bubble a friendly error state instead of throwing.
- **Instrument** – Add structured logging of widget status + token age to
  confirm whether failures disappear after the reset logic ships.

## BLUE-BANDED-BEE-42 – Persistent Stuck Tasks Warning

**Core problem**  
Queue monitor emits “Found 1 stuck tasks across 1 jobs” for job
`0bf5a3fc-4cae…`. The warning fires ~100×/day (718 events in last week) because
`worker.monitorStuckTasks` never sees the `pending` task age decrease; the task
is permanently `running` but no worker owns it.

**Technical solution**

- **Detect orphaned workers** – Extend `worker_pool` heartbeat to mark stale
  workers and requeue their tasks if `updated_at` exceeds 2× lease interval.
- **Task auto-requeue** – Add SQL that moves tasks from `running` → `pending`
  when the owning worker is missing, and increment a `requeued` counter to avoid
  infinite loops.
- **Operational cleanup** – Add an admin job to clear the sample stuck task and
  verify the monitor returns to zero.

## BLUE-BANDED-BEE-3Y – Job Removed from Pool While Running

**Core problem**  
Companion info-level log states that job `0bf5a3fc-4cae…` disappeared from the
worker pool “without completion”. This is a symptom of the same stuck-task
condition, but it also indicates we delete job rows prematurely when their tasks
are still `running`.

**Technical solution**

- **Tighten job lifecycle** – Update the job garbage-collection routine to
  verify `pending + running + retrying = 0` before removing pool membership.
- **Add foreign-key constraints** – Prevent worker/job deletion if dependent
  tasks exist; instead transition tasks → `failed` with reason `job_removed`.
- **Alerting** – Downgrade this log to warning only when auto-requeue logic
  triggers more than N times/hour, so it complements Issue 42.

## BLUE-BANDED-BEE-6C – Transaction Already Committed/Rolled Back

**Core problem**  
`(*DbQueue).executeOnce` tries to `Commit()` after a helper already committed or
rolled back (e.g. once `CreatePageRecords` experiences an error). The double
commit raises `sql: transaction has already been committed or rolled back` and
results in lost task progress because we exit before marking the task done.

**Technical solution**

- **Single owner for transactions** – Refactor queue workers so that `Execute()`
  returns early when the task handler reports it already committed (add
  `ErrTxnClosed` sentinel). Only the worker is allowed to call
  `Commit/Rollback`.
- **Defer guard** – Add
  `defer func(){ if tx.Status()==Active { tx.Rollback() }}` blocks to guarantee
  cleanup on all paths.
- **Testing** – Create regression tests in `internal/db/queue_test.go` that run
  a handler which commits manually and confirm the worker doesn’t double commit.

## BLUE-BANDED-BEE-7G – PG Timeout in `CreatePageRecords`

**Core problem**  
New high-priority issue (4 events) where `CreatePageRecords` exceeds the context
deadline while inserting discovered links. Indicates slow transactions once the
queue falls behind, likely due to long-running scans from Issue 42/3Y keeping DB
connections pinned.

**Technical solution**

- **Shorter DB work units** – Batch inserts inside `CreatePageRecords` to ≤250
  rows and flush per batch so the transaction keeps under the 5 s worker
  deadline.
- **Context tuning** – Plumb a longer database timeout independent of the worker
  lease so expensive pages can complete without tripping the global queue timer.
- **Backpressure** – When stuck-task monitor reports active incidents as in
  Issue 42, temporarily rate-limit page discovery jobs to keep DB load below the
  timeout threshold.

## Next Steps

1. Prioritize the queue reliability issues (BLUE-BANDED-BEE-42 & 3Y) since they
   cascade into the DB failures (6C, 7G).
2. Schedule a small frontend hotfix for the Turnstile error (40) with at least
   one canary deploy before rolling to all dashboards.
3. Track completion in Sentry by linking commits and marking issues resolved
   only after 48 hours of no recurrences.
