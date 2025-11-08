# Queue Stability Work – November 2025

Date: 2025‑11‑08  
Owner: Infra team

## Summary

We finished the queue overhaul sparked by the 10× load test:

- Added `jobs.pending_tasks` / `jobs.waiting_tasks` counters + trigger to keep
  them accurate.
- `EnqueueURLs` now locks the job row and uses those counters when deciding how
  many tasks can be marked `pending`.
- Worker guardrail (rebalance + idle monitor) reads the counters instead of
  scanning `tasks`, logging global queue depth and demoting overflow pending
  rows every 5 minutes.
- Updated CHANGELOG + tests to match the new SQL.
- Cleaned up research docs to keep only outstanding actions (waiting backlog,
  worker scaling, resource analysis, recommendations).

All tests pass (`go test ./...`) and the queue no longer needs heavy `COUNT(*)`
queries.
