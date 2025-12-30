-- Create a trigger on jobs table for status change notifications
--
-- This handles notifications when jobs transition to terminal states
-- (completed, failed) regardless of whether the change came from
-- the task progress trigger or from direct Go code updates.
--
-- Also fixes update_job_progress() to:
-- 1. Preserve 'completed' as a terminal state
-- 2. Remove notification logic (now handled by this trigger)

-- Create notification trigger function for job status changes
CREATE OR REPLACE FUNCTION notify_job_status_change()
RETURNS TRIGGER AS $$
DECLARE
    job_domain_name TEXT;
    duration_secs INTEGER;
    notification_id UUID;
BEGIN
    -- Only process if status actually changed to a terminal state
    IF OLD.status = NEW.status THEN
        RETURN NEW;
    END IF;

    -- Only create notifications for jobs with an organisation_id
    IF NEW.organisation_id IS NULL THEN
        RETURN NEW;
    END IF;

    -- Get domain name and calculate duration
    SELECT d.name INTO job_domain_name
    FROM domains d
    WHERE d.id = NEW.domain_id;

    duration_secs := EXTRACT(EPOCH FROM (COALESCE(NEW.completed_at, NOW()) - NEW.started_at))::INTEGER;

    -- Create notification when job transitions to completed
    IF OLD.status != 'completed' AND NEW.status = 'completed' THEN
        notification_id := gen_random_uuid();
        INSERT INTO notifications (id, organisation_id, user_id, type, title, message, data, created_at)
        VALUES (
            notification_id,
            NEW.organisation_id,
            NEW.user_id,
            'job_complete',
            'Job completed: ' || COALESCE(job_domain_name, 'Unknown'),
            format('%s URLs warmed, %s failed', NEW.completed_tasks, NEW.failed_tasks),
            jsonb_build_object(
                'job_id', NEW.id,
                'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks,
                'failed_tasks', NEW.failed_tasks,
                'duration', COALESCE(duration_secs, 0) || 's'
            ),
            NOW()
        );
        -- Signal Go service for real-time delivery
        PERFORM pg_notify('new_notification', notification_id::text);
    END IF;

    -- Create notification when job transitions to failed
    IF OLD.status NOT IN ('failed', 'cancelled') AND NEW.status = 'failed' THEN
        notification_id := gen_random_uuid();
        INSERT INTO notifications (id, organisation_id, user_id, type, title, message, data, created_at)
        VALUES (
            notification_id,
            NEW.organisation_id,
            NEW.user_id,
            'job_failed',
            'Job failed: ' || COALESCE(job_domain_name, 'Unknown'),
            format('%s URLs completed, %s failed', NEW.completed_tasks, NEW.failed_tasks),
            jsonb_build_object(
                'job_id', NEW.id,
                'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks,
                'failed_tasks', NEW.failed_tasks,
                'error_message', 'Job processing failed'
            ),
            NOW()
        );
        -- Signal Go service for real-time delivery
        PERFORM pg_notify('new_notification', notification_id::text);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger on jobs table
CREATE TRIGGER on_job_status_change
    AFTER UPDATE OF status ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION notify_job_status_change();

-- Update comment
COMMENT ON FUNCTION notify_job_status_change() IS
  'Creates notifications when job status transitions to completed or failed.
   Fires on any status change to jobs table, whether from task progress
   trigger or direct Go code updates.

   Created in migration: 20251230063341';


-- Now update the task trigger to:
-- 1. Preserve 'completed' as a terminal state (prevents regression)
-- 2. Remove notification logic (now handled by notify_job_status_change)

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
    new_status TEXT;
BEGIN
    -- Determine which job to update
    IF TG_OP = 'DELETE' THEN
        job_id_to_update = OLD.job_id;
    ELSE
        job_id_to_update = NEW.job_id;
    END IF;

    -- Get the total tasks and current status for this job
    SELECT j.total_tasks, j.status
    INTO total_tasks, current_status
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

    -- Determine the new status
    new_status = CASE
        -- Preserve terminal states - these are final and should not change
        WHEN current_status IN ('cancelled', 'failed', 'completed') THEN current_status
        -- Transition to completed when all tasks are processed
        WHEN new_progress >= 100.0 THEN 'completed'
        WHEN completed_count > 0 OR failed_count > 0 THEN 'running'
        ELSE current_status
    END;

    -- Update the job with new counts and progress
    UPDATE jobs
    SET
        completed_tasks = completed_count,
        failed_tasks = failed_count,
        skipped_tasks = skipped_count,
        progress = new_progress,
        status = new_status
    WHERE id = job_id_to_update;

    -- Return the appropriate record based on operation
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Update comment
COMMENT ON FUNCTION update_job_progress() IS
  'Updates job progress counters when task status changes.
   Preserves terminal states (cancelled, failed, completed) to prevent regression.
   Notifications are handled by notify_job_status_change() trigger on jobs table.

   Updated in migration: 20251230063341
   Changes:
   - Added completed to terminal states
   - Removed notification logic (moved to jobs table trigger)';
