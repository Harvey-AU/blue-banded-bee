-- Fix the update_job_counters function to properly handle INSERT operations
-- The previous migration accidentally removed INSERT handling, causing total_tasks not to update
-- when new URLs are discovered during crawling

CREATE OR REPLACE FUNCTION update_job_counters()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- New task created - increment total_tasks and source-specific counters
        UPDATE jobs
        SET total_tasks = total_tasks + 1,
            sitemap_tasks = CASE
                WHEN NEW.source_type = 'sitemap' THEN sitemap_tasks + 1
                ELSE sitemap_tasks
            END,
            found_tasks = CASE
                WHEN NEW.source_type != 'sitemap' OR NEW.source_type IS NULL THEN found_tasks + 1
                ELSE found_tasks
            END
        WHERE id = NEW.job_id;

    ELSIF TG_OP = 'UPDATE' THEN
        -- Task status changed - update counters
        UPDATE jobs
        SET
            completed_tasks = (
                SELECT COUNT(*) FROM tasks
                WHERE job_id = NEW.job_id AND status = 'completed'
            ),
            failed_tasks = (
                SELECT COUNT(*) FROM tasks
                WHERE job_id = NEW.job_id AND status = 'failed'
            ),
            skipped_tasks = (
                SELECT COUNT(*) FROM tasks
                WHERE job_id = NEW.job_id AND status = 'skipped'
            ),
            -- Update progress calculation
            progress = CASE
                WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                    ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
                ELSE 0.0
            END,
            -- Update timestamps when job starts/completes - using UTC
            started_at = CASE
                WHEN started_at IS NULL AND NEW.status = 'running' THEN NOW() AT TIME ZONE 'UTC'
                ELSE started_at
            END,
            completed_at = CASE
                WHEN NEW.status IN ('completed', 'failed') AND
                     completed_tasks + failed_tasks + skipped_tasks >= total_tasks THEN NOW() AT TIME ZONE 'UTC'
                ELSE completed_at
            END
        WHERE id = NEW.job_id;

    ELSIF TG_OP = 'DELETE' THEN
        -- Task deleted - decrement counters
        UPDATE jobs
        SET total_tasks = GREATEST(0, total_tasks - 1),
            completed_tasks = CASE
                WHEN OLD.status = 'completed' THEN GREATEST(0, completed_tasks - 1)
                ELSE completed_tasks
            END,
            failed_tasks = CASE
                WHEN OLD.status = 'failed' THEN GREATEST(0, failed_tasks - 1)
                ELSE failed_tasks
            END,
            skipped_tasks = CASE
                WHEN OLD.status = 'skipped' THEN GREATEST(0, skipped_tasks - 1)
                ELSE skipped_tasks
            END,
            sitemap_tasks = CASE
                WHEN OLD.source_type = 'sitemap' THEN GREATEST(0, sitemap_tasks - 1)
                ELSE sitemap_tasks
            END,
            found_tasks = CASE
                WHEN OLD.source_type != 'sitemap' OR OLD.source_type IS NULL THEN GREATEST(0, found_tasks - 1)
                ELSE found_tasks
            END,
            -- Recalculate progress
            progress = CASE
                WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                    ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
                ELSE 0.0
            END
        WHERE id = OLD.job_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Ensure the trigger is properly set up
DROP TRIGGER IF EXISTS trigger_update_job_counters ON tasks;
CREATE TRIGGER trigger_update_job_counters
    AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_job_counters();