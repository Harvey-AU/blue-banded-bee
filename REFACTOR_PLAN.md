# getJobTasks Refactor Plan

## Current State
- **File**: `internal/api/jobs.go`
- **Function**: `getJobTasks` (lines 520-735, 216 lines)
- **Current coverage**: 3.6%
- **Problem**: Massive function doing too many things

## Target State
Break into 5 focused functions:
1. `parseTaskQueryParams()` - Parse limit/offset/status/sort params
2. `validateJobAccess()` - Auth checks and job ownership 
3. `buildTaskQuery()` - Build database query with filters/sorting
4. `formatTasksResponse()` - Convert DB results to API response
5. `getJobTasks()` - Orchestrate the above (~15-20 lines)

## Refactor Steps

### Step 1: Extract Parameter Parsing ✅
- [x] Create `parseTaskQueryParams(r *http.Request) TaskQueryParams`
- [x] Move lines 551-601 (parameter parsing logic)
- [x] Test: Verify existing functionality works
- [x] Commit: "Extract parameter parsing from getJobTasks"

### Step 2: Extract Auth/Validation ✅
- [x] Create `validateJobAccess(w http.ResponseWriter, r *http.Request, jobID string) *db.User`
- [x] Move lines ~521-549 (auth and job ownership validation)
- [x] Test: Auth context validation tests
- [x] Commit: "Extract job access validation from getJobTasks"

### Step 3: Extract Query Building ✅
- [x] Create `buildTaskQuery(jobID string, params TaskQueryParams) TaskQueryBuilder`
- [x] Move query construction logic (~30 lines)
- [x] Test: SQL generation and injection safety tests
- [x] Commit: "Extract query building from getJobTasks"

### Step 4: Extract Response Formatting ✅
- [x] Create `formatTasksFromRows(rows *sql.Rows) ([]TaskResponse, error)`
- [x] Move response building logic (~67 lines)
- [x] Test: Comprehensive database row formatting tests
- [x] Commit: "Extract response formatting from getJobTasks"

### Step 5: Final Function Cleanup ✅
- [x] getJobTasks now orchestrates all extracted functions
- [x] Reduced from 216 lines to 56 lines (~74% reduction)
- [x] Clean, focused orchestration logic
- [x] All existing functionality preserved

## Success Criteria
- [x] All existing functionality preserved
- [x] No breaking changes to API contract
- [x] All tests pass
- [x] 4 focused functions instead of 1 monster (+ 1 orchestrator)
- [x] Each function <72 lines (much more manageable)
- [x] Ready for comprehensive unit testing
- [x] Reduced getJobTasks from 216 lines to 56 lines (74% reduction)

## Risk Mitigation
- Test after each step
- Small, atomic commits
- Preserve exact functionality
- Can rollback any single step if needed

---
**Status**: ✅ COMPLETED - getJobTasks refactored successfully

## NEXT HIGH-IMPACT REFACTOR OPPORTUNITIES

### Monster Functions Identified (>100 lines)

#### CRITICAL PRIORITY: Core Business Logic

1. **`internal/jobs/manager.go:CreateJob()` - 232 lines** 🔥
   - **Risk**: Core job creation logic in one massive function
   - **Impact**: High - central to business operations
   - **Complexity**: Very High - handles validation, URL discovery, database setup
   - **Suggested breakup**:
     - `validateJobOptions()` - Input validation and defaults
     - `discoverJobURLs()` - Sitemap/crawling URL discovery  
     - `setupJobDatabase()` - Create job record and initial pages
     - `configureJobWorkers()` - Worker pool setup
     - `CreateJob()` - Orchestrate the above

2. **`internal/jobs/worker.go:processNextTask()` - 204 lines** 🔥
   - **Risk**: Core task processing pipeline
   - **Impact**: High - worker reliability depends on this
   - **Complexity**: Very High - error handling, state management, crawling
   - **Suggested breakup**:
     - `claimNextTask()` - Database task claiming logic
     - `executeTaskCrawl()` - URL crawling and response handling
     - `updateTaskResults()` - Database result storage
     - `handleTaskErrors()` - Error classification and retry logic
     - `processNextTask()` - Orchestrate the above

3. **`internal/jobs/worker.go:processTask()` - 162 lines** 🔥  
   - **Risk**: Individual task execution logic
   - **Impact**: High - determines crawl success/failure
   - **Complexity**: High - HTTP handling, cache validation, metrics
   - **Suggested breakup**:
     - `validateTaskURL()` - URL validation and robots.txt check
     - `executeCrawlRequest()` - HTTP request execution
     - `analyseResponse()` - Response analysis and metrics
     - `recordTaskMetrics()` - Performance tracking
     - `processTask()` - Orchestrate the above

#### HIGH PRIORITY: Database Operations

4. **`internal/db/db.go:setupSchema()` - 216 lines** ⚠️
   - **Risk**: Schema management complexity
   - **Impact**: Medium - affects all database operations  
   - **Complexity**: Medium - mostly SQL statements
   - **Suggested breakup**:
     - `createCoreTables()` - Main table creation
     - `createIndexes()` - Performance indexes
     - `setupRLSPolicies()` - Security policies (already exists)
     - `createTriggers()` - Database triggers
     - `setupSchema()` - Orchestrate the above

#### MEDIUM PRIORITY: Worker Management

5. **`internal/jobs/worker.go:checkForPendingTasks()` - 110 lines**
   - **Risk**: Worker coordination complexity
   - **Impact**: Medium - affects scaling and performance
   - **Complexity**: Medium - job queue management
   - **Suggested breakup**:
     - `identifyPendingJobs()` - Find jobs needing workers
     - `scaleWorkerPools()` - Adjust worker counts
     - `cleanupCompletedJobs()` - Remove finished jobs
     - `checkForPendingTasks()` - Orchestrate the above

6. **`internal/jobs/manager.go:processSitemap()` - 102 lines**
   - **Risk**: Sitemap processing complexity
   - **Impact**: Medium - affects URL discovery
   - **Complexity**: Medium - XML parsing and URL handling

7. **`internal/jobs/worker.go:AddJob()` - 91 lines**
   - **Risk**: Job initialization complexity
   - **Impact**: Medium - affects job startup
   - **Complexity**: Medium - worker assignment and setup

### Refactoring Strategy Recommendations

#### For Immediate Testing Wins (Next Session):
1. **Start with `CreateJob()`** - Highest business impact, most testable when broken down
2. **Focus on pure logic functions first** - validation, URL processing, data transformation
3. **Test each extraction** - following the successful getJobTasks pattern

#### For Database Functions:
1. **`setupSchema()`** - Break into logical groups, test schema creation
2. **Queue operations** - Already identified in TEST_PLAN.md

#### For Worker Functions:
1. **`processNextTask()`** and `processTask()`** - Critical for system reliability
2. **Break down error handling** - Currently mixed with business logic
3. **Extract metrics/logging** - Currently embedded throughout

### Success Pattern Established

The **getJobTasks refactor demonstrates the winning approach**:
✅ **Extract + Test + Commit** in small steps
✅ **Preserve functionality** while improving structure  
✅ **Comprehensive test coverage** for each extracted function
✅ **74% size reduction** with better maintainability

**Apply this same pattern** to the monster functions above for maximum impact on testing and maintainability.

## RECOMMENDED NEXT ACTION

**Immediate priority: `CreateJob()` function (232 lines)**

This is the **highest impact** refactor target because:
- ✅ **Core business logic** - job creation is central to the entire system
- ✅ **High test value** - breaking it down will enable comprehensive unit testing
- ✅ **Clear boundaries** - validation, URL discovery, database setup are separate concerns
- ✅ **Proven pattern** - we've successfully demonstrated the extract+test+commit approach

**Expected outcome**: 
- Reduce 232-line function to ~30-line orchestrator
- Add 80-90% test coverage for job creation logic
- Eliminate major source of production bugs
- Make future job creation features much easier to implement

**Estimated effort**: 2-3 hours following the established pattern