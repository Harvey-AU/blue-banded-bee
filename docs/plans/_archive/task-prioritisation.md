# Task Prioritization Implementation Plan

## Overview

Implement a priority-based task processing system using a PostgreSQL view that ranks pages by their importance in the site hierarchy, replacing the current FIFO (First In, First Out) approach.

## Requirements (from Roadmap.md)

- Give pages ranking by number of pages (e.g., 1700 page site, highest rank is 1700)
- All pages start at 0 rank, except homepage which is max (1700)
- Give pages higher score based on if it's in the header or homepage
- Each time a page is linked from another page, assign more rank
- Job-level task prioritisation options

## Core Design Principles

1. **Minimal Schema Changes**: Only add `priority_score` to pages table
2. **PostgreSQL-Driven Logic**: Use views and functions for ordering
3. **Percentage-Based Scoring**: Normalise scores as percentages (0.0 to 1.0)
4. **No Complex Tracking**: Leverage existing `source_url` and `source_type` fields

## Database Schema Changes

### 1. Pages Table Enhancement

```sql
-- Add priority_score column to pages table (stored as percentage 0.0-1.0)
ALTER TABLE pages 
ADD COLUMN priority_score NUMERIC(4,3) DEFAULT 0.000;

-- Add index for efficient priority ordering
CREATE INDEX idx_pages_priority ON pages(domain_id, priority_score DESC);
```

### 2. Jobs Table Enhancement

```sql
-- Add prioritization mode and total pages count
ALTER TABLE jobs
ADD COLUMN prioritization_mode TEXT DEFAULT 'hierarchy',
ADD COLUMN total_pages INTEGER DEFAULT 0;

-- Prioritization modes:
-- 'hierarchy' - Use page hierarchy scoring (default)
-- 'fifo' - First in, first out (current behaviour)
```

## PostgreSQL View for Task Acquisition

### Priority-Ordered Task View

```sql
CREATE OR REPLACE VIEW prioritized_pending_tasks AS
WITH job_config AS (
    SELECT DISTINCT 
        j.id as job_id,
        j.prioritization_mode,
        j.total_pages
    FROM jobs j
)
SELECT 
    t.id, 
    t.job_id, 
    t.page_id, 
    p.path, 
    t.created_at, 
    t.retry_count, 
    t.source_type, 
    t.source_url,
    -- Calculate effective priority based on job mode
    CASE 
        WHEN jc.prioritization_mode = 'fifo' THEN 0
        ELSE COALESCE(p.priority_score, 0.000)
    END as effective_priority
FROM tasks t
JOIN pages p ON t.page_id = p.id
JOIN job_config jc ON t.job_id = jc.job_id
WHERE t.status = 'pending';
```

## PostgreSQL Functions for Priority Calculation

### 1. Update Page Priority Function

```sql
CREATE OR REPLACE FUNCTION update_page_priority(
    p_page_id INTEGER,
    p_source_type TEXT,
    p_source_page_id INTEGER DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    v_domain_id INTEGER;
    v_total_pages INTEGER;
    v_path TEXT;
    v_source_priority NUMERIC(4,3);
    v_new_priority NUMERIC(4,3);
BEGIN
    -- Get page details
    SELECT domain_id, path INTO v_domain_id, v_path
    FROM pages
    WHERE id = p_page_id;
    
    -- Get total pages for this domain from any active job
    SELECT COALESCE(MAX(j.total_pages), 0) INTO v_total_pages
    FROM jobs j
    JOIN domains d ON j.domain_id = d.id
    WHERE d.id = v_domain_id
    AND j.status IN ('pending', 'running');
    
    -- Skip if no active jobs (total_pages = 0)
    IF v_total_pages = 0 THEN
        RETURN;
    END IF;
    
    -- Calculate priority based on source
    IF v_path = '/' THEN
        -- Homepage gets maximum priority (1.0)
        v_new_priority := 1.000;
    ELSIF p_source_type = 'sitemap' THEN
        -- Sitemap pages get moderate priority (0.5)
        v_new_priority := 0.500;
    ELSIF p_source_type = 'link' AND p_source_page_id IS NOT NULL THEN
        -- Get source page priority
        SELECT COALESCE(priority_score, 0.000) INTO v_source_priority
        FROM pages
        WHERE id = p_source_page_id;
        
        -- Linked pages inherit 80% of source page priority
        v_new_priority := v_source_priority * 0.8;
        
        -- Ensure minimum priority for linked pages
        IF v_new_priority < 0.100 THEN
            v_new_priority := 0.100;
        END IF;
    ELSE
        -- Default priority
        v_new_priority := 0.100;
    END IF;
    
    -- Update the page priority (only increase, never decrease)
    UPDATE pages
    SET priority_score = GREATEST(priority_score, v_new_priority)
    WHERE id = p_page_id;
END;
$$ LANGUAGE plpgsql;
```

### 2. Batch Update Priorities After Sitemap Processing

```sql
CREATE OR REPLACE FUNCTION update_job_page_priorities(p_job_id TEXT)
RETURNS VOID AS $$
DECLARE
    v_domain_id INTEGER;
    v_total_pages INTEGER;
BEGIN
    -- Get job details
    SELECT domain_id, total_pages INTO v_domain_id, v_total_pages
    FROM jobs
    WHERE id = p_job_id;
    
    -- Update homepage priority
    UPDATE pages
    SET priority_score = 1.000
    WHERE domain_id = v_domain_id
    AND path = '/';
    
    -- Update sitemap page priorities
    UPDATE pages p
    SET priority_score = 0.500
    FROM tasks t
    WHERE t.page_id = p.id
    AND t.job_id = p_job_id
    AND t.source_type = 'sitemap'
    AND p.path != '/'
    AND p.priority_score < 0.500;
END;
$$ LANGUAGE plpgsql;
```

### 3. Trigger to Update Priority on Task Creation

```sql
CREATE OR REPLACE FUNCTION task_created_update_priority()
RETURNS TRIGGER AS $$
DECLARE
    v_source_page_id INTEGER;
BEGIN
    -- Only process for link-discovered pages
    IF NEW.source_type = 'link' AND NEW.source_url IS NOT NULL THEN
        -- Extract source page ID from source_url
        SELECT p.id INTO v_source_page_id
        FROM pages p
        JOIN domains d ON p.domain_id = d.id
        WHERE d.name || p.path = NEW.source_url
        LIMIT 1;
        
        -- Update priority for the new task's page
        IF v_source_page_id IS NOT NULL THEN
            PERFORM update_page_priority(
                NEW.page_id, 
                NEW.source_type, 
                v_source_page_id
            );
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger
CREATE TRIGGER trigger_task_priority_update
AFTER INSERT ON tasks
FOR EACH ROW
EXECUTE FUNCTION task_created_update_priority();
```

## Task Acquisition Query Changes

### Current Query (FIFO)
```sql
SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url 
FROM tasks 
WHERE status = 'pending'
AND job_id = $1
ORDER BY created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

### New Query (View-based with Priority)
```sql
SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url
FROM prioritized_pending_tasks
WHERE job_id = $1
ORDER BY effective_priority DESC, created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

## Implementation Steps

### Phase 1: Database Schema Updates
1. Add `priority_score` column to pages table
2. Add `prioritization_mode` and `total_pages` to jobs table
3. Create the prioritized_pending_tasks view
4. Create PostgreSQL functions
5. Create trigger for automatic priority updates

### Phase 2: Update Go Code
1. Update job creation to set `total_pages` after sitemap processing
2. Modify GetNextTask to use the view when prioritization_mode = 'hierarchy'
3. No changes needed to EnqueueURLs function

### Phase 3: Homepage Detection
1. When creating page records, detect path = '/' and set priority_score = 1.0
2. Call `update_job_page_priorities` after sitemap processing completes

## Simplified Priority Rules

1. **Homepage (/)**: 100% (1.000)
2. **Sitemap pages**: 50% (0.500)
3. **Pages linked from homepage**: 80% (0.800)
4. **Pages linked from other pages**: 80% of source page priority
5. **Minimum priority**: 10% (0.100)

## Benefits

1. **Simple Implementation**: Minimal schema changes, view-based ordering
2. **PostgreSQL-Driven**: All logic in database, minimal Go code changes
3. **Percentage-Based**: Easy to understand and reason about
4. **No Complex Tracking**: Uses existing source_url field
5. **Backwards Compatible**: FIFO mode still available
6. **Performance**: Single indexed column for ordering

## Performance Considerations

1. **Index Usage**: Priority queries use idx_pages_priority
2. **View Performance**: Simple view with minimal joins
3. **Trigger Overhead**: Minimal - only updates on task creation
4. **No Additional Queries**: Priority calculated during normal operations

## Testing Strategy

1. Create test job with known sitemap
2. Verify homepage gets 1.0 priority
3. Verify sitemap pages get 0.5 priority
4. Test link discovery propagates priority correctly
5. Compare processing order: FIFO vs hierarchy
6. Performance test with large site (1000+ pages)