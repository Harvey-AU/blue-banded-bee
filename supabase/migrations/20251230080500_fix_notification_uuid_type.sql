-- Fix UUID type mismatch in notify_job_status_change function
-- The notification_id was declared as TEXT but notifications.id is UUID

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
