# Task Prioritization System

## Overview

This document outlines the design and implementation plan for adding task prioritization to Blue Banded Bee's job queue system. Task prioritization will allow more important tasks to be processed before less important ones, improving overall system responsiveness for high-priority operations.

## Current System

Currently, our system processes tasks in a FIFO (First-In-First-Out) order based on creation time:

```sql
ORDER BY created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

This means all tasks are treated equally regardless of their importance or the job they belong to.

## Proposed Changes

### 1. Database Changes

We need to extend the `tasks` table with a priority field:

```sql
ALTER TABLE tasks ADD COLUMN priority INTEGER NOT NULL DEFAULT 5;
```

The priority field will use a numeric scale where:
- Lower numbers = higher priority (1 = highest priority)
- Default priority = 5 (medium priority)
- Range: 1-10

### 2. Task Queue Modifications

Update the GetNextTask query to consider priority:

```sql
SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url 
FROM tasks 
WHERE status = 'pending'
AND (job_id = $1 OR $1 = '')
ORDER BY priority ASC, created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

This ensures that higher priority tasks (lower priority numbers) are processed first, with creation time as a secondary sorting criterion.

### 3. API Changes

Extend the JobOptions and related interfaces to accept a priority parameter:

```go
type JobOptions struct {
    // Existing fields
    Domain         string
    Concurrency    int
    FindLinks      bool
    UseSitemap     bool
    MaxPages       int
    IncludePaths   []string
    ExcludePaths   []string
    RequiredWorkers int
    
    // New field
    Priority       int // Default: 5
}
```

### 4. Job Manager Changes

Update the EnqueueJobURLs method to pass the job's priority to EnqueueURLs:

```go
func (jm *JobManager) EnqueueJobURLs(ctx context.Context, jobID string, pageIDs []int, paths []string, sourceType string, sourceURL string, priority int) error {
    // ...existing code...
    
    // Use the filtered lists to enqueue only new pages
    err := jm.dbQueue.EnqueueURLs(ctx, jobID, filteredPageIDs, filteredPaths, sourceType, sourceURL, priority)
    
    // ...existing code...
}
```

### 5. Worker Pool Changes

Modify the worker pool's task retrieval logic to be priority-aware:

```go
func (wp *WorkerPool) processNextTask(ctx context.Context) error {
    // Get tasks based on priority
    task, err := wp.dbQueue.GetNextTask(ctx, "") // Empty job ID to get tasks from any job
    
    // ...existing processing logic...
}
```

## Implementation Plan

1. **Database Migration**
   - Add the priority column to the tasks table
   - Update existing indexes to include priority
   
2. **Interface Updates**
   - Modify the DbQueueProvider interface to include priority
   - Update all implementations of EnqueueURLs
   
3. **Task Processing**
   - Update the task selection query to consider priority
   - Modify the worker pool to handle prioritized tasks
   
4. **API Endpoints**
   - Update job creation endpoint to accept priority
   - Add new endpoints to modify job/task priorities
   
5. **Testing**
   - Test priority-based task selection
   - Verify high-priority tasks are processed before low-priority ones
   - Test under load to ensure priority system works correctly

## Priority Assignment Guidelines

### Default Priority (5)
- Standard website cache warming tasks
- Regular scheduled crawls

### High Priority (1-4)
- Priority 1: Critical system tasks (e.g., database cleanup)
- Priority 2: User-initiated urgent crawls
- Priority 3: Tasks from premium-tier customers
- Priority 4: Time-sensitive crawls (e.g., after a website update)

### Low Priority (6-10)
- Priority 6-7: Background/discovery tasks
- Priority 8-9: Deep crawl tasks that aren't time-sensitive
- Priority 10: Lowest priority maintenance tasks

## Benefits

1. **Improved User Experience**: Critical tasks processed faster
2. **Service Level Differentiation**: Higher priorities for premium customers
3. **System Efficiency**: Important tasks don't get stuck behind less important ones
4. **Resource Management**: Better allocation of worker resources

## Technical Considerations

1. **Database Load**: Priority ordering adds minimal overhead to queries
2. **Starvation Prevention**: Need to ensure low-priority tasks eventually run
3. **Concurrency**: Priority system works well with our row-level locking approach
4. **Monitoring**: Need to add metrics to track task queue by priority