# Worker Scaling and Priority System

## Overview

This plan outlines implementing dynamic worker scaling, job priority tiers, and domain protection for the Blue Banded Bee cache warming service. The goal is to provide premium speed tiers while maintaining responsible crawling practices and system efficiency.

## 1. Dynamic Worker Scaling

### Current State
- Fixed 5 workers processing all jobs equally
- No scaling based on workload

### Proposed Changes
- **Base worker count:** 3 workers
- **Scaling rule:** +3 workers per active job
- **Maximum workers:** 25 (to avoid database connection pool issues)
- **Scaling behaviour:** Scale up when jobs added, scale down when jobs complete

### Examples
- 1 job: 3 workers
- 3 jobs: 9 workers
- 5 jobs: 15 workers
- 8+ jobs: 25 workers (capped)

## 2. Job Priority System

### Four-Tier Priority Model

| Tier   | Worker Allocation | Behaviour |
|--------|------------------|-----------|
| Ultra  | 100%            | Blocks all other tiers until complete |
| High   | 70%             | Premium/large jobs |
| Medium | 20%             | Standard paid jobs |
| Low    | 10%             | Free tier jobs |

### Fair Share Implementation
- Each tier gets guaranteed minimum allocation
- Unused capacity from higher tiers shared proportionally with lower tiers
- Example: If no Ultra/High jobs, Medium gets 77% (20% + 70%*0.22), Low gets 23%

### Use Cases
- **Ultra:** Emergency cache warming, critical deployments
- **High:** Premium plans, large enterprise jobs
- **Medium:** Standard paid plans
- **Low:** Free tier, small jobs

## 3. Domain Protection

### Simple Domain Cooldown
- **Method:** In-memory per-worker domain tracking
- **Cooldown period:** 200ms between requests to same domain per worker
- **Implementation:** Zero database overhead, automatic rate limiting

```go
// Per worker, in-memory only
lastDomainRequest := map[string]time.Time{}

// In task selection loop
if time.Since(lastDomainRequest[domain]) < 200ms {
    continue // Try next job
}
lastDomainRequest[domain] = time.Now()
```

### Benefits
- Maximum 5 requests/second per worker per domain
- Natural distribution across domains
- Prevents accidental DDoS of target sites
- Scales automatically with worker count
- Maintains good relationship with target sites

## 4. Implementation Benefits

### Resource Efficiency
- Workers scale with actual workload (3-25 range)
- No waste during low-activity periods
- Efficient use of Fly.io resources

### Site-Friendly Crawling
- Automatic rate limiting through domain cooldown
- Respects target site capacity
- Maintains good crawler citizenship

### User Experience
- Predictable performance tiers
- Premium users get meaningful speed improvements
- Free tier users still get reasonable service

### Technical Benefits
- Minimal database overhead for domain protection
- Simple scaling logic using existing worker pool architecture
- Backwards compatible with current job system

## 5. Resource Impact Analysis

### Memory Usage
- Base: ~300KB for 3 workers
- Scaled: ~2.5MB for 25 workers
- Domain maps: Negligible (~50KB per worker)

### Fly.io Scaling
- 10K pages: Current instance handles easily
- 50K pages: May need 512MB instance (+$3/month)
- 100K+ pages: Need 1GB instance (+$10/month)

### Database Impact
- No additional queries for domain protection
- Existing worker pool queries handle scaling
- Connection pool (25 max) adequate for 25 workers

## 6. Implementation Priority

### Phase 1: Core Scaling
1. Dynamic worker scaling (3 + 3*jobs, max 25)
2. Domain cooldown protection
3. Basic testing and monitoring

### Phase 2: Priority System
1. Add priority field to JobOptions
2. Implement worker allocation logic
3. Add priority selection to job creation UI

### Phase 3: Optimization
1. Monitoring and metrics for priority effectiveness
2. Fine-tune worker scaling ratios
3. Optimize domain cooldown timing if needed

## 7. Risk Mitigation

### Technical Risks
- **Database connection exhaustion:** Mitigated by 25 worker cap
- **Memory usage:** Minimal impact, monitored via metrics
- **Domain blocking:** Prevented by 200ms cooldown

### Business Risks
- **Free tier degradation:** Guaranteed 10% minimum allocation
- **Premium tier expectations:** Clear tier definitions and SLAs
- **Resource costs:** Predictable scaling with job volume

## 8. Success Metrics

### Performance Metrics
- Premium jobs complete 2-4x faster than current system
- Standard jobs maintain reasonable completion times
- No increase in target site blocking/rate limiting

### Resource Metrics
- Worker utilisation >80% during peak periods
- Memory usage stays within instance limits
- Database connection pool utilisation <90%

### User Experience Metrics
- Premium tier adoption rates
- User satisfaction with job completion times
- Reduction in support tickets about slow processing