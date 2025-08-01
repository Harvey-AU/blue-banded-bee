-- Fix update_job_progress trigger to recalculate total_tasks
-- This prevents the issue where completed_tasks can exceed total_tasks in the dashboard

-- Drop the existing trigger first
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;

-- Create the improved update_job_progress function
CREATE OR REPLACE FUNCTION update_job_progress()
RETURNS TRIGGER AS $$
DECLARE
    job_id_to_update TEXT;
    total_count INTEGER;
    sitemap_count INTEGER;
    found_count INTEGER;
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
    
    -- Count all task types and statuses in a single query
    -- This ensures total_tasks is always accurate
    SELECT 
        COUNT(*) as total,
        COUNT(*) FILTER (WHERE source_type = 'sitemap') as sitemap,
        COUNT(*) FILTER (WHERE source_type != 'sitemap' OR source_type IS NULL) as found,
        COUNT(*) FILTER (WHERE status = 'completed') as completed,
        COUNT(*) FILTER (WHERE status = 'failed') as failed,
        COUNT(*) FILTER (WHERE status = 'skipped') as skipped
    INTO total_count, sitemap_count, found_count, completed_count, failed_count, skipped_count
    FROM tasks
    WHERE job_id = job_id_to_update;
    
    -- Calculate progress percentage (only count completed + failed, not skipped)
    IF total_count > 0 AND (total_count - skipped_count) > 0 THEN
        new_progress = (completed_count + failed_count)::REAL / (total_count - skipped_count)::REAL * 100.0;
    ELSE
        new_progress = 0.0;
    END IF;
    
    -- Update the job with recalculated counts and progress
    -- Key fix: total_tasks is now recalculated from actual task count
    UPDATE jobs
    SET 
        total_tasks = total_count,
        sitemap_tasks = sitemap_count,
        found_tasks = found_count,
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

-- Recreate the trigger with the improved function
CREATE TRIGGER trigger_update_job_progress
    AFTER INSERT OR UPDATE OR DELETE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_job_progress();

-- Add comment for documentation
COMMENT ON FUNCTION update_job_progress() IS 'Updates job progress and recalculates all task counts including total_tasks to prevent stale data';