-- Optimise RLS policies to prevent row-by-row function evaluation
-- Wraps auth.uid() with (SELECT ...) to cache result per query instead of per row
-- Context: Supabase linter detected InitPlan issue causing 10,000x overhead
-- Reference: https://supabase.com/docs/guides/database/postgres/row-level-security#call-functions-with-select

-- 1. Users table: Users can access own data
DROP POLICY IF EXISTS "Users can access own data" ON users;
CREATE POLICY "Users can access own data"
ON users FOR ALL
USING (id = (SELECT auth.uid()));

-- 2. Organisations table: Users can access own organisation
DROP POLICY IF EXISTS "Users can access own organisation" ON organisations;
CREATE POLICY "Users can access own organisation"
ON organisations FOR ALL
USING (id = (
  SELECT organisation_id
  FROM users
  WHERE id = (SELECT auth.uid())
));

-- 3. Jobs table: Organisation members can access jobs
DROP POLICY IF EXISTS "Organisation members can access jobs" ON jobs;
CREATE POLICY "Organisation members can access jobs"
ON jobs FOR ALL
USING (organisation_id = (
  SELECT organisation_id
  FROM users
  WHERE id = (SELECT auth.uid())
));

-- 4. Tasks table: Organisation members can access tasks
DROP POLICY IF EXISTS "Organisation members can access tasks" ON tasks;
CREATE POLICY "Organisation members can access tasks"
ON tasks FOR ALL
USING (
  EXISTS (
    SELECT 1
    FROM jobs
    WHERE jobs.id = tasks.job_id
      AND jobs.organisation_id = (
        SELECT organisation_id
        FROM users
        WHERE id = (SELECT auth.uid())
      )
  )
);

-- 5. Domains table: Allow INSERT without existing job, restrict SELECT to owned jobs
-- Split USING (read) and WITH CHECK (write) to allow workers to create domains
-- while maintaining tenant isolation on reads
DROP POLICY IF EXISTS "Organisation members can access domains" ON domains;
DROP POLICY IF EXISTS "Users can read domains via jobs" ON domains;
DROP POLICY IF EXISTS "Authenticated users can create domains" ON domains;
DROP POLICY IF EXISTS "Users can update domains via jobs" ON domains;

-- Allow reading domains that have jobs in user's organisation
CREATE POLICY "Users can read domains via jobs"
ON domains FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM jobs
    WHERE jobs.domain_id = domains.id
      AND jobs.organisation_id = (
        SELECT organisation_id
        FROM users
        WHERE id = (SELECT auth.uid())
      )
  )
);

-- Allow any authenticated user to create domains (checked at job level)
-- Workers need to create domains before jobs exist
CREATE POLICY "Authenticated users can create domains"
ON domains FOR INSERT
WITH CHECK (auth.role() = 'authenticated');

-- NO UPDATE POLICY: Domains are shared resources
-- Service role only can update to prevent cross-tenant data corruption

-- 6. Pages table: Similar split for tenant isolation while allowing worker inserts
DROP POLICY IF EXISTS "Organisation members can access pages" ON pages;
DROP POLICY IF EXISTS "Users can read pages via jobs" ON pages;
DROP POLICY IF EXISTS "Authenticated users can create pages" ON pages;
DROP POLICY IF EXISTS "Users can update pages via jobs" ON pages;

-- Allow reading pages that have jobs in user's organisation
CREATE POLICY "Users can read pages via jobs"
ON pages FOR SELECT
USING (
  EXISTS (
    SELECT 1
    FROM jobs
    WHERE jobs.domain_id = pages.domain_id
      AND jobs.organisation_id = (
        SELECT organisation_id
        FROM users
        WHERE id = (SELECT auth.uid())
      )
  )
);

-- Allow any authenticated user to create pages (checked at job level)
-- Workers discover and create pages during crawling
CREATE POLICY "Authenticated users can create pages"
ON pages FOR INSERT
WITH CHECK (auth.role() = 'authenticated');

-- NO UPDATE POLICY: Pages are shared resources
-- Service role only can update to prevent cross-tenant data corruption

COMMENT ON POLICY "Users can access own data" ON users
IS 'Optimised: auth.uid() wrapped in SELECT to prevent per-row evaluation';

COMMENT ON POLICY "Organisation members can access jobs" ON jobs
IS 'Optimised: auth.uid() wrapped in SELECT to prevent per-row evaluation';

COMMENT ON POLICY "Organisation members can access tasks" ON tasks
IS 'Optimised: auth.uid() wrapped in SELECT to prevent per-row evaluation';
