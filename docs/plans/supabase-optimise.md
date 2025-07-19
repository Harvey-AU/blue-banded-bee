# Supabase Query Performance Optimisation Plan

## Overview
Analysis of query performance data reveals opportunities to significantly improve database performance. The top 3 queries consume 79.7% of total database time.

## Immediate Actions

### 1. Add Index on `started_at` (HIGH PRIORITY)
Supabase recommends this index which will improve query performance by **53.61%**:

```sql
-- Use CONCURRENTLY to avoid locking the table during creation
CREATE INDEX CONCURRENTLY idx_tasks_started_at ON public.tasks USING btree (started_at);
```

**Impact:**
- Reduces query cost from 27,733 to 12,864
- Affects stuck job cleanup queries
- Will become more critical as the tasks table grows

**Note:** If you get a timeout error, use the CONCURRENTLY option above which allows the index to be built without blocking writes.

### 2. Create Additional Performance Indexes

Run these indexes to optimise specific query patterns:

```sql
-- For task queue queries (GetNextTask pattern)
CREATE INDEX CONCURRENTLY idx_tasks_queue ON public.tasks (job_id, status, priority_score DESC, created_at ASC);

-- For job completion checks
CREATE INDEX CONCURRENTLY idx_jobs_status_completion ON public.jobs (status) WHERE status = 'running';

-- For priority update queries
CREATE INDEX CONCURRENTLY idx_tasks_job_priority ON public.tasks (job_id, priority_score);

-- Optional: For pending tasks if most are completed
CREATE INDEX CONCURRENTLY idx_tasks_pending ON public.tasks (status, created_at) WHERE status = 'pending';

-- Run ANALYZE after creating indexes
ANALYZE tasks;
ANALYZE jobs;
ANALYZE pages;
```

### 3. Optimise Connection Pool Settings

Update `internal/db/db.go` for 10x jobs with 1000x pages workload:

```go
// Current: MaxIdleConns: 25, MaxOpenConns: 75
// Recommended:
MaxIdleConns: 30          // Keep more connections ready
MaxOpenConns: 50          // Stay under Supabase limits (60-100 per project)
MaxLifetime: 15 * time.Minute  // Rotate connections more frequently
```

### 4. Database-Side Batch Updates for Job Counters

Instead of code changes, implement PostgreSQL trigger for automatic job stats:

```sql
-- Create function to update job statistics
CREATE OR REPLACE FUNCTION update_job_stats() RETURNS TRIGGER AS $$
BEGIN
    -- Update job stats in batches
    UPDATE jobs 
    SET total_tasks = (SELECT COUNT(*) FROM tasks WHERE job_id = NEW.job_id),
        completed_tasks = (SELECT COUNT(*) FROM tasks WHERE job_id = NEW.job_id AND status = 'completed'),
        failed_tasks = (SELECT COUNT(*) FROM tasks WHERE job_id = NEW.job_id AND status = 'failed')
    WHERE id = NEW.job_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to run periodically or after significant changes
-- Note: Adjust trigger conditions based on your needs
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
Remove references to non-existent columns (`url`, `depth`) in old queries that appear in the performance logs but would fail if executed.

### 7. Monitor Pages Table Performance
The `INSERT INTO pages` query has been called 11.4 million times with ON CONFLICT handling. This may be resolved by recent duplicate handling fixes, but should be monitored.

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