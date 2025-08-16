# CreateJob Function Refactor Plan

## Current Analysis
- **File**: `internal/jobs/manager.go`
- **Function**: `CreateJob()` (lines 53-284, 232 lines)
- **Problem**: Massive function handling multiple responsibilities

## Function Structure (Identified Sections)

### Section 1: Existing Job Check & Cancellation (lines ~60-103)
- Checks for existing active jobs for same domain/org
- Cancels existing job if found
- **Extract to**: `handleExistingJobs(ctx, domain, orgID)`

### Section 2: Job Object Creation (lines ~105-128) 
- Creates new Job struct with options
- Sets initial status and defaults
- **Extract to**: `createJobObject(options)`

### Section 3: Domain & Database Setup (lines ~130-180)
- Get/create domain record
- Database transaction setup
- **Extract to**: `setupJobDatabase(ctx, job)`

### Section 4: URL Discovery (lines ~180-240)
- Sitemap discovery if enabled
- URL enumeration and validation
- **Extract to**: `discoverJobURLs(ctx, job, options)`

### Section 5: Worker Pool Integration (lines ~240-284)
- Add job to worker pool
- Configure worker requirements
- **Extract to**: `configureJobWorkers(job, options)`

## Refactor Steps

### Step 1: Extract Existing Job Handling ⏳
- [ ] Create `handleExistingJobs(ctx, domain, orgID) error`
- [ ] Test: Mock database scenarios (existing job, no job, query errors)

### Step 2: Extract Job Object Creation ⏳  
- [ ] Create `createJobObject(options) *Job`
- [ ] Test: All field mappings, defaults, edge cases

### Step 3: Extract Database Setup ⏳
- [ ] Create `setupJobDatabase(ctx, job) error`
- [ ] Test: Domain creation, transaction handling

### Step 4: Extract URL Discovery ⏳
- [ ] Create `discoverJobURLs(ctx, job, options) error`
- [ ] Test: Sitemap vs manual URL discovery

### Step 5: Extract Worker Configuration ⏳
- [ ] Create `configureJobWorkers(job, options) error`
- [ ] Test: Worker pool integration

### Step 6: Simplify CreateJob ⏳
- [ ] Orchestrate all extracted functions
- [ ] Target: ~20-30 lines
- [ ] Test: Full integration test

---
**Status**: Analysis Complete | **Next**: Step 1 - Extract Existing Job Handling