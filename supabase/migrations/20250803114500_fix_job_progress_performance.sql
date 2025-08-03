-- Fix O(n²) job progress performance issue by using incremental counters
-- Instead of counting all tasks on every update, we increment/decrement counters

-- First, ensure all job counters are accurate before switching to incremental updates
-- This will only run if the jobs table exists and has data
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'jobs') THEN
        UPDATE jobs j
        SET 
            total_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = j.id), 0),
            completed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = j.id AND status = 'completed'), 0),
            failed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = j.id AND status = 'failed'), 0),
            skipped_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = j.id AND status = 'skipped'), 0);
    END IF;
END $$;

-- Drop the old inefficient trigger
DROP TRIGGER IF EXISTS trigger_update_job_progress ON tasks;
DROP FUNCTION IF EXISTS update_job_progress();

-- Create the new O(1) incremental counter function
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
        
    ELSIF TG_OP = 'UPDATE' AND OLD.status != NEW.status THEN
        -- Status changed - update status counters and progress
        UPDATE jobs 
        SET completed_tasks = completed_tasks + 
                CASE WHEN NEW.status = 'completed' AND OLD.status != 'completed' THEN 1 
                     WHEN OLD.status = 'completed' AND NEW.status != 'completed' THEN -1 
                     ELSE 0 END,
            failed_tasks = failed_tasks + 
                CASE WHEN NEW.status = 'failed' AND OLD.status != 'failed' THEN 1 
                     WHEN OLD.status = 'failed' AND NEW.status != 'failed' THEN -1 
                     ELSE 0 END,
            skipped_tasks = skipped_tasks + 
                CASE WHEN NEW.status = 'skipped' AND OLD.status != 'skipped' THEN 1 
                     WHEN OLD.status = 'skipped' AND NEW.status != 'skipped' THEN -1 
                     ELSE 0 END,
            progress = CASE 
                WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                    ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
                ELSE 0.0 
            END,
            -- Update timestamps when job starts/completes
            started_at = CASE 
                WHEN started_at IS NULL AND NEW.status = 'running' THEN NOW()
                ELSE started_at
            END,
            completed_at = CASE 
                WHEN NEW.status IN ('completed', 'failed') AND 
                     completed_tasks + failed_tasks + skipped_tasks >= total_tasks THEN NOW()
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
            END
        WHERE id = OLD.job_id;
    END IF;
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Create the new efficient trigger only if tasks table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tasks') THEN
        CREATE TRIGGER trigger_update_job_counters
            AFTER INSERT OR UPDATE OF status OR DELETE ON tasks
            FOR EACH ROW
            EXECUTE FUNCTION update_job_counters();
    END IF;
END $$;

-- Drop the existing function if it exists (may have different parameter names)
DROP FUNCTION IF EXISTS recalculate_job_stats(TEXT);

-- Create the missing recalculate function that the Go code expects
CREATE OR REPLACE FUNCTION recalculate_job_stats(p_job_id TEXT) 
RETURNS void AS $$
BEGIN
    UPDATE jobs 
    SET total_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id), 0),
        completed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'completed'), 0),
        failed_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'failed'), 0),
        skipped_tasks = COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = p_job_id AND status = 'skipped'), 0),
        progress = CASE 
            WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
            ELSE 0.0 
        END
    WHERE id = p_job_id;
END;
$$ LANGUAGE plpgsql;

-- Add index to support the incremental updates efficiently
CREATE INDEX IF NOT EXISTS idx_jobs_id ON jobs(id);