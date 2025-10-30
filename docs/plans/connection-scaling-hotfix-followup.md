# Connection Scaling Hotfix – Follow-up Plan

Date: 30 October 2025  
Owner: Blue Banded Bee backend team

## Context

Production load tests exposed a critical failure mode: when multiple large jobs
run concurrently the Supabase connection pool saturates. Workers stall, the
stuck-job watchdog fires, and jobs are marked failed after 30 minutes with no
task progress. Immediate mitigations were applied:

- Reduced `DB_QUEUE_MAX_CONCURRENCY` default to 12 to keep task-claim
  transactions well below pool capacity.
- Lowered production worker count to 30 (from 50) so the pool stays saturated
  without exhausting connections.

This plan captures everything still outstanding to stabilise and scale
connection handling beyond the hotfix.

## Immediate Validation (now)

1. **Redeploy with new limits**
   - Confirm `DB_QUEUE_MAX_CONCURRENCY=12` and `jobWorkers=30` in the running
     stack.
2. **Load-test replay**
   - Use `scripts/load-test-simple.sh` with ≤10 concurrent jobs.
   - Watch `pg_stat_activity` and application logs for `ErrPoolSaturated`, 429s,
     or stuck-job warnings.
3. **Dashboard verification**
   - Ensure previously failing domains complete without triggering the 30-minute
     timeout.

## Short-Term Hardening (next sprint)

1. **Expose pool metrics**
   - Emit OpenTelemetry gauges for pool usage, wait count, and queue saturation.
   - Build Grafana alerts for sustained utilisation >80%.
2. **Surface job failure reasons**
   - Display `jobs.error_message` in the dashboard so operators see “no progress
     for 30 minutes” versus WAF/403 failures.
3. **Active job guardrail**
   - Cap the number of running jobs based on pool headroom (e.g. 12 concurrent
     task slots → 3–4 jobs at a time).
   - Implement admission control in `JobManager.StartJob`.

## Medium-Term Scalability (Stage 6 roadmap)

1. **Burst-protected connection classes**
   - Create separate Supabase roles/DSNs for interactive versus batch workloads.
   - Route non-interactive jobs through a lower-capacity pool to protect the
     main app.
2. **Read replica routing**
   - Introduce replicas and split read-heavy APIs off the primary to reduce pool
     contention.
3. **Tenant-level quotas**
   - Enforce per-tenant pool limits so one payload cannot starve others during
     large crawls.
4. **Serverless proxy front**
   - Add a managed Postgres proxy in front of future edge/serverless workloads
     to prevent storm spikes.

## Rollback Plan

- `DB_QUEUE_MAX_CONCURRENCY` is environment-driven; revert via env override if
  necessary.
- Worker count is configurable via redeploy; set `jobWorkers` back to 50 only
  after Supabase pool scaling or the above hardening is complete.

## References

- `cmd/app/main.go:437` – Worker pool size configuration.
- `internal/db/queue.go:45` – Queue concurrency guard.
- Roadmap Stage 6 tasks: connection scaling items added 30 Oct 2025.
