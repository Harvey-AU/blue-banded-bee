# Rearchitecture & Code Clean-up

## Purpose

This refactoring aims to eliminate duplicate code, standardize naming conventions, and clarify responsibilities between packages. The key improvements are:

1. Single implementation for database operations
2. Elimination of global state with proper dependency injection
3. More consistent and predictable function naming
4. Clear separation of concerns between packages

## Target Architecture

internal/
├── common/
│ │ - queue.go
│ │ - DbQueue struct (core transaction queue)
│ └── Execute (schedules DB operations)
│
├── db/
│ ├── db.go
│ │ - InitFromEnv (creates DB connection)
│ │ - setupSchema (creates tables)
│ │ - MOVED: serialize (previously: in jobs/db.go)
│ │ - MOVED: retryDB (previously: in jobs/db.go)
│ │
│ ├── queue.go
│ │ - DbQueue implementation
│ │ - EnqueueURLs (single implementation)
│ │ - GetNextTask (previously: GetNextPendingTask)
│ │ - UpdateTaskStatus (previously: multiple functions: UpdateTaskStatusTx, CompleteTask, FailTask)
│ │ - UpdateJobProgress
│ └── MOVED: CleanupStuckJobs (previously: in queue_helpers.go)
│
└── jobs/
│ ├── REMOVE: db.go (functions moved to other files)
│ │
│ ├── manager.go
│ │ - CreateJob
│ │ - StartJob
│ │ - CancelJob
│ │ - GetJobStatus
│ │ - MOVED: GetJob (previously: in db.go)
│ │ - processSitemap
│ │ - UPDATED: All methods to use dbQueue instead of direct SQL
│ │
│ ├── REMOVE: queue_helpers.go (functions moved elsewhere)
│ │
│ ├── types.go
│ │ - Job struct
│ │ - Task struct
│ │ - JobOptions struct
│ │ - Constants for job/task statuses
│ │
│ ├── worker.go
│ │
│ │ - WorkerPool management
│ │ - processTask
│ │ - REMOVE: EnqueueURLs (use db/queue.go version instead)
│ │ - UPDATED: processNextTask (previously: used GetNextPendingTaskTx)
│ │ - UPDATED: task status updates (previously: used UpdateTaskStatusTx)
│ │ - UPDATED: Replace ExecuteInQueue with direct dbQueue.Execute
│ └── UPDATED: Worker initialization to receive dbQueue (previously: used global SetDBInstance)

## Step-by-Step Implementation Guide

### Phase 1: Prepare DB Package

[x] 1.1. Move serialize function from jobs/db.go to db/db.go
[x] 1.2. Move retryDB function from jobs/db.go to db/db.go
[x] 1.3. Standardize GetNextTask in db/queue.go (rename from GetNextPendingTask)
[x] 1.4. Create unified UpdateTaskStatus in db/queue.go that handles all task status changes
[x] 1.5. Move CleanupStuckJobs from queue_helpers.go to db/queue.go

### Phase 2: Update Jobs Manager

[x] 2.1. Move GetJob function from db.go to manager.go
[x] 2.2. Update JobManager to accept DbQueue in constructor
[x] 2.3. Update all JobManager methods to use dbQueue instead of direct SQL
[ ] 2.4. Test that job creation and management still work

### Phase 3: Update Worker

[ ] 3.1. Add dbQueue field to WorkerPool struct
[ ] 3.2. Update WorkerPool constructor to accept dbQueue
[ ] 3.3. Replace EnqueueURLs calls with dbQueue.EnqueueURLs
[ ] 3.4. Update processNextTask to use dbQueue.GetNextTask
[ ] 3.5. Update task status updates to use dbQueue.UpdateTaskStatus
[ ] 3.6. Replace ExecuteInQueue calls with direct dbQueue.Execute

### Phase 4: Remove Redundant Code

[ ] 4.1. Remove jobs/db.go after confirming all functions are moved
[ ] 4.2. Remove jobs/queue_helpers.go after all functionality is moved
[ ] 4.3. Remove SetDBInstance and related global state

### Phase 5: Testing

[ ] 5.1. Test job creation from API
[ ] 5.2. Test job status updates
[ ] 5.3. Test task processing
[ ] 5.4. Test complete flow with find_links=true

Each change should be made one at a time and tested before proceeding to the next step.
