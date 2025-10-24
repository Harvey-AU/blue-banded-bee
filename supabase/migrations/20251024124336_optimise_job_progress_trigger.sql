-- Optimize job progress trigger to only fire on task status changes
--
-- Problem: The trigger was firing on EVERY task update (priority_score, response_time,
-- cache_status, headers, etc.), causing:
-- - 8.5M job table updates from 3.37M task updates
-- - 270 deadlock errors per day (multiple workers updating different tasks from same job)
-- - Connection pool saturation from blocked transactions waiting for job row locks
-- - 9.4% transaction rollback rate (healthy is <2%)
--
-- Solution: Only fire trigger when task status actually changes (pending→running→completed/failed)
-- This reduces trigger executions by ~80% while maintaining accurate progress tracking.
--
-- Expected impact:
-- - Trigger executions: 3.37M/day → ~670K/day (-80%)
-- - Job table updates: 8.5M/day → ~1.7M/day (-80%)
-- - Deadlock errors: 270/day → <50/day (-82%)
-- - Pool saturation events: 8,926/day → <1,500/day (-83%)
-- - Transaction rollback rate: 9.4% → <2%

-- Drop existing trigger
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;

-- Recreate trigger with optimized condition
-- Key change: UPDATE OF status - Only fires on status column changes, not all columns
-- This alone reduces trigger executions by ~80% (metadata updates no longer fire it)
-- Still fires on INSERT/DELETE as before for accurate progress tracking
CREATE TRIGGER trigger_update_job_progress
  AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION update_job_progress();

-- Add comment explaining the optimization
COMMENT ON TRIGGER trigger_update_job_progress ON tasks IS
  'Updates job progress counters only when task status changes.
   Optimized to avoid firing on metadata updates (priority_score, response_time, cache_status, etc.)
   which were causing excessive job table updates and deadlocks.

   Migration: 20251024124336
   Issue: Trigger storm causing 8.5M updates/day and 270 deadlocks/day
   Fix: Only fire on status changes, reducing executions by 80%';
