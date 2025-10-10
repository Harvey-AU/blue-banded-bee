# Supabase Query Performance Optimisation Plan

## Overview

Analysis of query performance data reveals opportunities to significantly
improve database performance. The top 3 queries consume 79.7% of total database
time.

## Immediate Actions

### 3. Optimise Connection Pool Settings

Update `internal/db/db.go` for 10x jobs with 1000x pages workload:

```go
// Current: MaxIdleConns: 25, MaxOpenConns: 75
// Recommended:
MaxIdleConns: 30          // Keep more connections ready
MaxOpenConns: 50          // Stay under Supabase limits (60-100 per project)
MaxLifetime: 15 * time.Minute  // Rotate connections more frequently
```

### 5. Review High-Frequency Updates

#### Current Performance Bottlenecks:

1. **Task status updates (31.9% of total time)**
   - 266,565 calls
   - Simple UPDATE setting status and started_at
2. **Task completion updates (26.4% of total time)**
   - 37,207 calls
   - Updates 25 columns (all performance metrics needed)
3. **Job counter updates (21.4% of total time)**
   - 90,382 calls
   - Can be optimised with database triggers above

### 6. Clean Up Legacy Queries

Remove references to non-existent columns (`url`, `depth`) in old queries that
appear in the performance logs but would fail if executed.

### 7. Monitor Pages Table Performance

The `INSERT INTO pages` query has been called 11.4 million times with ON
CONFLICT handling. This may be resolved by recent duplicate handling fixes, but
should be monitored.

## Implementation Priority

1. **Immediate**:
   - Create all indexes using CONCURRENTLY option
   - Run ANALYZE on all tables
2. **This Week**:
   - Update connection pool settings in code
   - Implement database triggers for job counter updates
3. **Next Sprint**:
   - Monitor performance improvements
   - Consider further optimisations based on new metrics
4. **Ongoing**:
   - Monitor query performance after each change
   - Watch Supabase connection limits

## Expected Outcomes

- 50%+ reduction in stuck job cleanup query time
- 20-30% reduction in overall database load through batching
- Better scalability as data volume grows
- Reduced database connection overhead

## Monitoring

After implementing each change:

1. Monitor query performance metrics in Supabase dashboard
2. Track application response times
3. Watch for any new slow queries that emerge
4. Validate that worker performance improves
