-- Add automatic notification creation when jobs complete
--
-- This updates the update_job_progress() trigger to create notifications
-- when a job transitions to 'completed' or 'failed' status.
-- Notifications are only created for jobs with an organisation_id.
--
-- This replaces the Go-based notification creation, providing a single
-- source of truth in the database trigger.

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
    job_org_id TEXT;
    job_user_id TEXT;
    job_domain_name TEXT;
    duration_secs INTEGER;
BEGIN
    -- Determine which job to update
    IF TG_OP = 'DELETE' THEN
        job_id_to_update = OLD.job_id;
    ELSE
        job_id_to_update = NEW.job_id;
    END IF;

    -- Get the total tasks, current status, and org info for this job
    SELECT j.total_tasks, j.status, j.organisation_id, j.user_id, d.name,
           EXTRACT(EPOCH FROM (COALESCE(j.completed_at, NOW()) - j.started_at))::INTEGER
    INTO total_tasks, current_status, job_org_id, job_user_id, job_domain_name, duration_secs
    FROM jobs j
    JOIN domains d ON j.domain_id = d.id
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
        WHEN current_status IN ('cancelled', 'failed') THEN current_status
        -- Only transition to completed if not already in a terminal state
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

    -- Create notification when job transitions to completed
    -- Only for jobs with an organisation_id
    IF current_status != 'completed' AND new_status = 'completed' AND job_org_id IS NOT NULL THEN
        INSERT INTO notifications (id, organisation_id, user_id, type, title, message, data, created_at)
        VALUES (
            gen_random_uuid(),
            job_org_id,
            job_user_id,
            'job_complete',
            'Job completed: ' || job_domain_name,
            format('%s URLs warmed, %s failed', completed_count, failed_count),
            jsonb_build_object(
                'job_id', job_id_to_update,
                'domain', job_domain_name,
                'completed_tasks', completed_count,
                'failed_tasks', failed_count,
                'duration', COALESCE(duration_secs, 0) || 's'
            ),
            NOW()
        );
    END IF;

    -- Create notification when job transitions to failed
    -- Only for jobs with an organisation_id
    IF current_status NOT IN ('failed', 'cancelled') AND new_status = 'failed' AND job_org_id IS NOT NULL THEN
        INSERT INTO notifications (id, organisation_id, user_id, type, title, message, data, created_at)
        VALUES (
            gen_random_uuid(),
            job_org_id,
            job_user_id,
            'job_failed',
            'Job failed: ' || job_domain_name,
            format('%s URLs completed, %s failed', completed_count, failed_count),
            jsonb_build_object(
                'job_id', job_id_to_update,
                'domain', job_domain_name,
                'completed_tasks', completed_count,
                'failed_tasks', failed_count,
                'error_message', 'Job processing failed'
            ),
            NOW()
        );
    END IF;

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
   Creates notifications when jobs complete or fail.
   Preserves terminal states (cancelled, failed) to prevent race conditions.

   Updated in migration: 20251230054537
   Change: Added automatic notification creation on job completion';
