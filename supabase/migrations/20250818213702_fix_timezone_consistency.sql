-- Fix timezone consistency for database triggers
-- Problem: Database triggers were using NOW() (server timezone) while Go code uses UTC
-- Solution: Change all NOW() calls to NOW() AT TIME ZONE 'UTC' for consistent UTC timestamps

-- Update the set_job_started_at trigger function
CREATE OR REPLACE FUNCTION set_job_started_at()
RETURNS TRIGGER AS $$
BEGIN
  -- Only set started_at if it's currently NULL and completed_tasks > 0
  IF NEW.completed_tasks > 0 AND (TG_OP = 'INSERT' OR OLD.started_at IS NULL) AND NEW.started_at IS NULL THEN
    NEW.started_at = NOW() AT TIME ZONE 'UTC';
  END IF;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Update the set_job_completed_at trigger function  
CREATE OR REPLACE FUNCTION set_job_completed_at()
RETURNS TRIGGER AS $$
BEGIN
  -- Set completed_at when progress reaches 100% and it's not already set
  IF NEW.progress >= 100.0 AND (TG_OP = 'INSERT' OR OLD.completed_at IS NULL) AND NEW.completed_at IS NULL THEN
    NEW.completed_at = NOW() AT TIME ZONE 'UTC';
  END IF;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Update the update_job_counters trigger function
CREATE OR REPLACE FUNCTION update_job_counters()
RETURNS TRIGGER AS $$
BEGIN
    -- Task inserted/updated - update job counters
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
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
            -- Recalculate progress
            progress = CASE 
                WHEN total_tasks > 0 AND (total_tasks - skipped_tasks) > 0 THEN
                    ((completed_tasks + failed_tasks)::REAL / (total_tasks - skipped_tasks)::REAL) * 100.0
                ELSE 0.0 
            END
        WHERE id = OLD.job_id;
    END IF;
    
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;