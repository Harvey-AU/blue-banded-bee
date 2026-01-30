# Optimisations

A few key optimisations to infra

## Memory leaks:

- DomainLimiter.domains map grows forever — every unique domain adds an entry
  with rate limit state, robots rules, and job tracking. No cleanup mechanism
  exists. Over 6 months with 1,000+ domains, this accumulates indefinitely.
- Job-related caches (jobInfoCache, jobPerformance, jobFailureCounters) only
  clean up via explicit RemoveJob() calls. Jobs that stall, error out, or are
  abandoned without proper cleanup leave entries permanently in memory.
- Impact: Estimated 200-400MB unreclaimable memory in long-running deployments.
  Gradual degradation rather than sudden failure — harder to detect until memory
  pressure causes issues.

## Observability gaps:

- No visibility into queue depth — can't see how many tasks are pending/waiting
  per job. Workers scale reactively but operators have no early warning of
  backlogs forming.
- No cache size metrics — len(domains) and job cache sizes aren't exposed.
  Impossible to correlate memory growth with business activity or detect leaks
  before they matter.
- Impact: Blind to scaling issues until auto-scaling triggers or failures occur.
  Incident diagnosis relies on logs rather than dashboards — slower MTTR.

## What's needed:

- TTL-based eviction for domain limiter — evict domains with no activity >1 hour
  via periodic cleanup goroutine. Prevents unbounded growth while preserving
  active rate limit state.
- Metrics exposing cache sizes — publish domain_limiter_domains_total,
  job_cache_entries_total to Prometheus. Enables trending, capacity planning,
  and leak detection.
- Queue depth metrics — record pending/waiting/processing counts per job.
  Surface in dashboards for real-time visibility into throughput and
  bottlenecks.
- Alerting thresholds — trigger warnings when domain cache exceeds 10,000
  entries or queue depth grows faster than processing rate.
