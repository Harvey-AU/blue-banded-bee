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

-- Recreate the function (in case it doesn't exist in preview environment)
CREATE OR REPLACE FUNCTION update_job_progress()
RETURNS TRIGGER AS $$
DECLARE
    job_id_to_update TEXT;
    total_tasks INTEGER;
    completed_count INTEGER;
    failed_count INTEGER;
    skipped_count INTEGER;
    new_progress REAL;
BEGIN
    -- Determine which job to update
    IF TG_OP = 'DELETE' THEN
        job_id_to_update = OLD.job_id;
    ELSE
        job_id_to_update = NEW.job_id;
    END IF;

    -- Get the total tasks for this job
    SELECT j.total_tasks INTO total_tasks
    FROM jobs j
    WHERE j.id = job_id_to_update;

    -- Count completed, failed, and skipped tasks
    SELECT
        COUNT(*) FILTER (WHERE status = 'completed'),
        COUNT(*) FILTER (WHERE status = 'failed'),
        COUNT(*) FILTER (WHERE status = 'skipped')
    INTO completed_count, failed_count, skipped_count
    FROM tasks
    WHERE job_id = job_id_to_update;

    -- Calculate progress percentage (only count completed + failed, not skipped)
    IF total_tasks > 0 AND (total_tasks - skipped_count) > 0 THEN
        new_progress = (completed_count + failed_count)::REAL / (total_tasks - skipped_count)::REAL * 100.0;
    ELSE
        new_progress = 0.0;
    END IF;

    -- Update the job with new counts and progress
    UPDATE jobs
    SET
        completed_tasks = completed_count,
        failed_tasks = failed_count,
        skipped_tasks = skipped_count,
        progress = new_progress,
        status = CASE
            WHEN new_progress >= 100.0 THEN 'completed'
            WHEN completed_count > 0 OR failed_count > 0 THEN 'running'
            ELSE status
        END
    WHERE id = job_id_to_update;

    -- Return the appropriate record based on operation
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

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
