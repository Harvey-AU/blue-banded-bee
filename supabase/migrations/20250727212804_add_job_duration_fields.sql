-- Add calculated duration fields to jobs table
-- These fields automatically calculate job duration and average time per task

-- Add duration_seconds as a generated column
ALTER TABLE jobs 
ADD COLUMN IF NOT EXISTS duration_seconds INTEGER GENERATED ALWAYS AS (
    CASE 
        WHEN started_at IS NOT NULL AND completed_at IS NOT NULL 
        THEN EXTRACT(EPOCH FROM (completed_at - started_at))::INTEGER
        ELSE NULL
    END
) STORED;

-- Add avg_time_per_task_seconds as a generated column
ALTER TABLE jobs 
ADD COLUMN IF NOT EXISTS avg_time_per_task_seconds NUMERIC GENERATED ALWAYS AS (
    CASE 
        WHEN started_at IS NOT NULL 
        AND completed_at IS NOT NULL 
        AND completed_tasks > 0
        THEN EXTRACT(EPOCH FROM (completed_at - started_at)) / completed_tasks
        ELSE NULL
    END
) STORED;

-- Add indexes for performance when querying by these fields
CREATE INDEX IF NOT EXISTS idx_jobs_duration ON jobs(duration_seconds) 
WHERE duration_seconds IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_avg_time ON jobs(avg_time_per_task_seconds) 
WHERE avg_time_per_task_seconds IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN jobs.duration_seconds IS 'Total job duration in seconds (calculated from started_at to completed_at)';
COMMENT ON COLUMN jobs.avg_time_per_task_seconds IS 'Average time per completed task in seconds';