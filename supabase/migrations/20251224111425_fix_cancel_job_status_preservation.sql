-- Fix: Preserve terminal job states (cancelled, failed) in progress trigger
--
-- Problem: The update_job_progress trigger unconditionally sets job status
-- to 'completed' when progress reaches 100%, overwriting 'cancelled' status.
--
-- Race condition scenario:
-- 1. User cancels job -> status = 'cancelled', pending tasks -> 'skipped'
-- 2. Running tasks continue to complete (cannot be stopped mid-flight)
-- 3. Task status change fires trigger
-- 4. Trigger calculates progress = 100% (all non-skipped tasks done)
-- 5. BUG: Trigger sets status = 'completed', losing the 'cancelled' state
--
-- Fix: Preserve terminal states before checking progress percentage

CREATE OR REPLACE FUNCTION update_job_progress()
RETURNS TRIGGER AS $$
DECLARE
    job_id_to_update TEXT;
    total_tasks INTEGER;
    completed_count INTEGER;
    failed_count INTEGER;
    skipped_count INTEGER;
    current_status TEXT;
    new_progress REAL;
BEGIN
    -- Determine which job to update
    IF TG_OP = 'DELETE' THEN
        job_id_to_update = OLD.job_id;
    ELSE
        job_id_to_update = NEW.job_id;
    END IF;

    -- Get the total tasks and current status for this job
    SELECT j.total_tasks, j.status INTO total_tasks, current_status
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
    -- CRITICAL FIX: Preserve terminal states (cancelled, failed) - they should never be overwritten
    UPDATE jobs
    SET
        completed_tasks = completed_count,
        failed_tasks = failed_count,
        skipped_tasks = skipped_count,
        progress = new_progress,
        status = CASE
            -- Preserve terminal states - these are final and should not change
            WHEN current_status IN ('cancelled', 'failed') THEN current_status
            -- Only transition to completed if not already in a terminal state
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

-- Add updated comment
COMMENT ON FUNCTION update_job_progress() IS
  'Updates job progress counters when task status changes.
   Preserves terminal states (cancelled, failed) to prevent race conditions.

   Fixed in migration: 20251224111425
   Issue: Cancel job race condition - trigger overwrote cancelled status';
