# Job Scheduling Implementation Plan

## Overview

Add recurring job scheduling allowing jobs to run automatically at 6, 12, 24, or
48 hour intervals.

## Implementation Tasks

### 1. Database Migration

Create `schedulers` table and add `scheduler_id` to `jobs`:

```sql
-- Create schedulers table
CREATE TABLE IF NOT EXISTS schedulers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id INTEGER NOT NULL REFERENCES domains(id),
    organisation_id UUID NOT NULL REFERENCES organisations(id),
    schedule_interval_hours INTEGER NOT NULL CHECK (schedule_interval_hours IN (6, 12, 24, 48)),
    next_run_at TIMESTAMPTZ NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Job configuration template
    concurrency INTEGER NOT NULL DEFAULT 20,
    find_links BOOLEAN NOT NULL DEFAULT TRUE,
    max_pages INTEGER NOT NULL DEFAULT 0,
    include_paths TEXT,
    exclude_paths TEXT,
    required_workers INTEGER NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_domain_org UNIQUE(domain_id, organisation_id)
);

-- Index for scheduler queries
CREATE INDEX IF NOT EXISTS idx_schedulers_next_run
ON schedulers(next_run_at)
WHERE is_enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_schedulers_organisation
ON schedulers(organisation_id);

-- Add scheduler_id to jobs table
ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS scheduler_id UUID REFERENCES schedulers(id);

CREATE INDEX IF NOT EXISTS idx_jobs_scheduler_id
ON jobs(scheduler_id)
WHERE scheduler_id IS NOT NULL;
```

**Note**: When creating jobs from scheduler, set `source_type = 'scheduler'` and
`source_detail = scheduler.id` (same fields used for dashboard/webhook job
creation).

### 2. Database Layer (`internal/db/schedulers.go`)

Create new file with functions:

- `CreateScheduler(ctx, scheduler) error`
- `GetScheduler(ctx, schedulerID) (*Scheduler, error)`
- `ListSchedulers(ctx, organisationID) ([]*Scheduler, error)`
- `UpdateScheduler(ctx, schedulerID, updates) error`
- `DeleteScheduler(ctx, schedulerID) error`
- `GetSchedulersReadyToRun(ctx, limit) ([]*Scheduler, error)` - WHERE
  `is_enabled = TRUE AND next_run_at <= NOW()`
- `UpdateSchedulerNextRun(ctx, schedulerID, nextRun) error`

Define `Scheduler` struct matching table schema.

### 3. API Layer (`internal/api/schedulers.go`)

Create new file with handlers:

- `POST /v1/schedulers` - Create scheduler
- `GET /v1/schedulers` - List schedulers for organisation
- `GET /v1/schedulers/:id` - Get scheduler details
- `PUT /v1/schedulers/:id` - Update scheduler (interval, config, enable/disable)
- `DELETE /v1/schedulers/:id` - Delete scheduler
- `GET /v1/schedulers/:id/jobs` - Get execution history (jobs with
  `scheduler_id = id`)

Request/response types:

- `SchedulerRequest` (domain, schedule_interval_hours, concurrency, find_links,
  etc.)
- `SchedulerResponse` (id, domain, schedule_interval_hours, next_run_at,
  is_enabled, etc.)

Register routes in `SetupRoutes()`.

### 4. Scheduler Service (`cmd/app/main.go`)

Add background goroutine `startJobScheduler()`:

```go
func startJobScheduler(ctx context.Context, wg *sync.WaitGroup, jobsManager *jobs.JobManager, pgDB *db.DB) {
    defer wg.Done()
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            schedulers, _ := pgDB.GetSchedulersReadyToRun(ctx, 50)
            for _, scheduler := range schedulers {
                // Create JobOptions from scheduler
                opts := &jobs.JobOptions{
                    Domain:          domainName, // lookup from domain_id
                    OrganisationID:  &scheduler.OrganisationID,
                    ScheduleIntervalHours: scheduler.ScheduleIntervalHours,
                    Concurrency:     scheduler.Concurrency,
                    FindLinks:       scheduler.FindLinks,
                    MaxPages:        scheduler.MaxPages,
                    SourceType:      stringPtr("scheduler"),
                    SourceDetail:    &scheduler.ID,
                }

                // Create job (standard flow)
                job, err := jobsManager.CreateJob(ctx, opts)
                if err != nil {
                    log.Error().Err(err).Str("scheduler_id", scheduler.ID).Msg("Failed to create scheduled job")
                    continue
                }

                // Link job to scheduler
                pgDB.GetDB().ExecContext(ctx, `UPDATE jobs SET scheduler_id = $1 WHERE id = $2`, scheduler.ID, job.ID)

                // Update scheduler next_run_at
                nextRun := time.Now().UTC().Add(time.Duration(scheduler.ScheduleIntervalHours) * time.Hour)
                pgDB.UpdateSchedulerNextRun(ctx, scheduler.ID, nextRun)
            }
        }
    }
}
```

Add to `main()`:

```go
backgroundWG.Add(1)
go startJobScheduler(appCtx, &backgroundWG, jobsManager, pgDB)
```

**Note**: Jobs created by scheduler are automatically picked up by existing
`StartTaskMonitor()` (runs every 30s), which adds them to the worker pool. No
changes needed to worker pool.

### 5. Job Manager (`internal/jobs/manager.go`)

Update `CreateJob()` to accept optional `scheduler_id` in `JobOptions` and set
it on job record during `setupJobDatabase()`.

## Testing

- Unit tests for scheduler CRUD operations
- Integration test: create scheduler → wait → verify job created with
  `scheduler_id`
- Test scheduler service picks up ready schedulers

## Notes

- Scheduler service creates jobs using standard `CreateJob()` flow
- Existing worker pool processes scheduled jobs identically to manual jobs
- Each scheduled execution creates a new job (new UUID) linked via
  `scheduler_id`
