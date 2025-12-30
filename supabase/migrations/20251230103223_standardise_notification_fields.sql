-- Standardise notification fields for consistent multi-channel delivery
--
-- New structure:
--   subject  - Main heading (e.g., "✅ Job completed: example.com")
--   preview  - Short summary for previews/toasts (e.g., "150 URLs warmed in 2m 30s")
--   message  - Full details if needed (optional)
--   link     - URL to view details (e.g., "/jobs/abc-123")

-- Rename title to subject
ALTER TABLE notifications RENAME COLUMN title TO subject;

-- Rename message to preview (short summary)
ALTER TABLE notifications RENAME COLUMN message TO preview;

-- Add message column for full details (optional)
ALTER TABLE notifications ADD COLUMN message TEXT;

-- Add link column for navigation
ALTER TABLE notifications ADD COLUMN link TEXT;

-- Update the job status notification trigger to use new fields
CREATE OR REPLACE FUNCTION notify_job_status_change()
RETURNS TRIGGER AS $$
DECLARE
    job_domain_name TEXT;
    duration_secs INTEGER;
    duration_text TEXT;
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

    -- Format duration as human-readable
    IF duration_secs < 60 THEN
        duration_text := duration_secs || 's';
    ELSIF duration_secs < 3600 THEN
        duration_text := (duration_secs / 60) || 'm ' || (duration_secs % 60) || 's';
    ELSE
        duration_text := (duration_secs / 3600) || 'h ' || ((duration_secs % 3600) / 60) || 'm';
    END IF;

    -- Create notification when job transitions to completed
    IF OLD.status != 'completed' AND NEW.status = 'completed' THEN
        notification_id := gen_random_uuid();
        INSERT INTO notifications (id, organisation_id, user_id, type, subject, preview, link, data, created_at)
        VALUES (
            notification_id,
            NEW.organisation_id,
            NEW.user_id,
            'job_complete',
            '✅ Job completed: ' || COALESCE(job_domain_name, 'Unknown'),
            format('%s URLs warmed in %s', NEW.completed_tasks, duration_text),
            '/jobs/' || NEW.id,
            jsonb_build_object(
                'job_id', NEW.id,
                'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks,
                'failed_tasks', NEW.failed_tasks,
                'duration', duration_text
            ),
            NOW()
        );
        PERFORM pg_notify('new_notification', notification_id::text);
    END IF;

    -- Create notification when job transitions to failed
    IF OLD.status NOT IN ('failed', 'cancelled') AND NEW.status = 'failed' THEN
        notification_id := gen_random_uuid();
        INSERT INTO notifications (id, organisation_id, user_id, type, subject, preview, link, data, created_at)
        VALUES (
            notification_id,
            NEW.organisation_id,
            NEW.user_id,
            'job_failed',
            '❌ Job failed: ' || COALESCE(job_domain_name, 'Unknown'),
            format('%s URLs completed, %s failed', NEW.completed_tasks, NEW.failed_tasks),
            '/jobs/' || NEW.id,
            jsonb_build_object(
                'job_id', NEW.id,
                'domain', COALESCE(job_domain_name, 'Unknown'),
                'completed_tasks', NEW.completed_tasks,
                'failed_tasks', NEW.failed_tasks,
                'error_message', 'Job processing failed'
            ),
            NOW()
        );
        PERFORM pg_notify('new_notification', notification_id::text);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION notify_job_status_change() IS
  'Creates notifications when job status transitions to completed or failed.
   Uses standardised fields: subject, preview, link.

   Updated in migration: 20251230103223';
