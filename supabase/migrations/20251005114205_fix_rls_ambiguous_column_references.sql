-- Fix ambiguous column references in RLS policies
-- This fixes the "ERROR: column reference 'id' is ambiguous (SQLSTATE 42702)" errors
-- by qualifying column names with their table names

-- Drop and recreate organisations RLS policy
DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
CREATE POLICY "Users can access own organisation" ON organisations
FOR ALL USING (
    id IN (
        SELECT organisation_id FROM users WHERE users.id = auth.uid()
    )
);

-- Drop and recreate jobs RLS policy
DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
CREATE POLICY "Organisation members can access jobs" ON jobs
FOR ALL USING (
    organisation_id IN (
        SELECT organisation_id FROM users WHERE users.id = auth.uid()
    )
);

-- Drop and recreate tasks RLS policy
DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
CREATE POLICY "Organisation members can access tasks" ON tasks
FOR ALL USING (
    job_id IN (
        SELECT jobs.id FROM jobs WHERE organisation_id IN (
            SELECT organisation_id FROM users WHERE users.id = auth.uid()
        )
    )
);
