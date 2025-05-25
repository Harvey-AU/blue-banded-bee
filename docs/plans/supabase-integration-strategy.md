# Supabase Advanced Integration Strategy

This document outlines a strategic approach to leverage Supabase capabilities beyond basic authentication and database storage, optimising the Blue Banded Bee application architecture.

## Overview

While we're already using Supabase for PostgreSQL database and authentication, its platform offers additional features that could enhance our application performance, monitoring capabilities, and scalability with minimal refactoring of our core Go codebase.

## Integration Areas

### 1. Database Functions & Triggers

Move critical database operations from Go code to PostgreSQL functions and triggers:

- **Task acquisition with row-level locking**
  ```sql
  CREATE FUNCTION get_next_pending_task(job_id_param TEXT)
  RETURNS TABLE (id TEXT, job_id TEXT, url TEXT, status TEXT) AS $$
  BEGIN
    RETURN QUERY
    UPDATE tasks
    SET status = 'running', started_at = NOW()
    WHERE id = (
      SELECT id FROM tasks
      WHERE status = 'pending' AND job_id = job_id_param
      ORDER BY created_at ASC
      FOR UPDATE SKIP LOCKED
      LIMIT 1
    )
    RETURNING tasks.id, tasks.job_id, tasks.url, tasks.status;
  END;
  $$ LANGUAGE plpgsql;
  ```

- **Automated job progress updates**
  ```sql
  CREATE TRIGGER update_job_progress_trigger
  AFTER UPDATE OF status ON tasks
  FOR EACH ROW
  EXECUTE FUNCTION update_job_progress();
  ```

- **URL enqueuing function**
  ```sql
  CREATE FUNCTION enqueue_urls(job_id_param TEXT, urls TEXT[])
  RETURNS INTEGER AS $$
    -- Implementation details
  $$ LANGUAGE plpgsql;
  ```

### 2. Supabase Realtime for Monitoring

Implement real-time monitoring using Supabase Realtime:

- **Job status tracking**
  ```typescript
  const jobSubscription = supabase
    .channel('job-updates')
    .on(
      'postgres_changes',
      { event: 'UPDATE', table: 'jobs', filter: `id=eq.${jobId}` },
      (payload) => updateJobProgressUI(payload.new)
    )
    .subscribe()
  ```

- **Task status updates**
  ```typescript
  const taskSubscription = supabase
    .channel('task-updates')
    .on(
      'postgres_changes',
      { event: '*', table: 'tasks', filter: `job_id=eq.${jobId}` },
      (payload) => updateTaskListUI(payload.new)
    )
    .subscribe()
  ```

### 3. Row Level Security for Multi-tenant Usage

Configure Row Level Security policies for secure multi-tenant usage:

```sql
-- Basic policy for user data isolation
CREATE POLICY "Users can only access their own jobs"
ON jobs
FOR ALL
USING (auth.uid() = user_id);

-- Policy for subscription-based limits
CREATE POLICY "Users can create jobs based on subscription limits"
ON jobs
FOR INSERT
USING (
  (SELECT count(*) FROM jobs WHERE user_id = auth.uid() AND 
   created_at > now() - interval '30 days') < 
  (SELECT job_limit FROM subscription_plans WHERE 
   plan_id = (SELECT plan_id FROM user_subscriptions WHERE user_id = auth.uid()))
);
```

### 4. Selective Edge Functions

Implement Edge Functions for specific operations:

- **Webhook handlers** for Webflow integration
- **Scheduled maintenance tasks**
- **Lightweight API endpoints** for dashboard data

Example Edge Function for handling a Webflow publish event:
```typescript
// webflow-publish-webhook.ts
export default async function webflowPublishHandler(req) {
  const { site_id, published_at, signature } = await req.json()
  // Verify webhook signature
  // Create new job in database
  // Return success response
}
```

## Implementation Approach

### Phase 1: Database Optimisation (1-2 weeks)

1. Identify key database operations suitable for migration to functions
2. Implement and test database functions for core operations:
   - Task acquisition
   - Job progress tracking
   - URL enqueuing

### Phase 2: Monitoring Enhancement (1 week)

1. Implement Supabase Realtime for job monitoring
2. Create real-time dashboard components
3. Set up WebSocket-based status updates

### Phase 3: Security & Multi-tenancy (1 week)

1. Implement Row Level Security policies
2. Configure subscription-based usage limits
3. Set up secure tenant isolation

### Phase 4: Integration Points (2 weeks)

1. Create Edge Functions for webhook handlers
2. Implement scheduled maintenance tasks
3. Set up integration with Webflow

## Benefits

- **Reduced application code complexity**
- **Improved database performance** through native functions
- **Real-time monitoring** capabilities
- **Enhanced security** with database-level access controls
- **Better scalability** through serverless components
- **Reduced operational costs** with selective serverless architecture

## Considerations

- Maintain the Go-based worker pool for CPU-intensive crawling
- Use database functions for coordination and state management
- Keep critical business logic in Go code for maintainability
- Use Supabase features selectively where they provide clear benefits

This integration strategy allows us to leverage Supabase's additional capabilities while preserving our existing architecture, providing incremental improvements without a complete rewrite.