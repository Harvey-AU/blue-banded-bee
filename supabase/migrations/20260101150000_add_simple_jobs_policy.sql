-- Add a simplified RLS policy for job creators.
-- This helps Realtime performance by avoiding the join to the users/organisations table
-- for the common case where a user is just viewing their own jobs.
-- This DOES NOT replace the organisation-based policy, but adds an alternative path.

CREATE POLICY "users_own_jobs_simple" ON jobs
FOR SELECT
USING (
    user_id = auth.uid()::text
);
