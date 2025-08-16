# Blue Banded Bee Refactor Plan

## Proven Methodology

The **Extract + Test + Commit** pattern has been successfully demonstrated:

✅ **Incremental approach** - Small, safe steps with immediate validation
✅ **Test-driven refactoring** - Test each extracted function comprehensively  
✅ **Clean commits** - Each step separately committed and reversible
✅ **Preserve functionality** - Zero regressions, all existing tests pass
✅ **Quality over speed** - Thorough understanding before changes

### Success Pattern Established

**Completed Achievements This Session:**
- **`getJobTasks`**: 216 → 56 lines (74% reduction) with 4 focused functions ✅
- **`CreateJob`**: 232 → 42 lines (82% reduction) with 4 focused functions ✅
- **`setupJobURLDiscovery`**: 108 → 17 lines (84% reduction) with 2 focused functions ✅
- **`validateCrawlRequest`**: Extracted from WarmURL with comprehensive tests ✅
- **220+ test cases** added with comprehensive coverage
- **Zero functional regressions** across entire codebase

## CURRENT PRIORITY TARGET

### Target: setupSchema() - 216 Lines 💀

**Why setupSchema() is the optimal next target:**
- ✅ **High impact, lower complexity** - Database foundation affects everything
- ✅ **Clear boundaries** - Tables, indexes, triggers, policies are distinct
- ✅ **Coverage opportunity** - DB package at only 30.5% coverage
- ✅ **Easier testing** - SQL operations are more predictable than HTTP/async logic
- ✅ **Foundation first** - Schema stability benefits all other work

## COMPREHENSIVE MONSTER FUNCTION ANALYSIS

**Current Large Files Status:**
- `internal/jobs/worker.go` - **1561 lines, 28 functions** (11 monsters >50 lines)
- `internal/jobs/manager.go` - **1031 lines, 25 functions** (8 monsters >50 lines)  
- `internal/api/jobs.go` - **801 lines, 14 functions** (7 monsters >50 lines)
- `internal/db/db.go` - **750 lines, 14 functions** (7 monsters >50 lines)
- `internal/crawler/crawler.go` - **669 lines, 10 functions** (2 monsters >50 lines)

## MONSTER FUNCTIONS BY PRIORITY

### 💀 **EXTREME PRIORITY (>200 lines) - System-Critical**

1. **`internal/crawler/crawler.go:WarmURL()` - 385 lines** 💀💀💀
   - **THE BIGGEST MONSTER** in entire codebase
   - **Risk**: Core URL crawling logic
   - **Impact**: EXTREME - determines all crawl success/failure
   - **Testing impact**: Would unlock massive coverage gains
   - **Complexity**: HTTP, response analysis, link extraction, cache validation

2. **`internal/db/db.go:setupSchema()` - 216 lines** 💀💀
   - **Risk**: Database foundation setup
   - **Impact**: HIGH - affects all database operations
   - **Complexity**: Table creation, indexes, triggers, policies

3. **`internal/jobs/worker.go:processNextTask()` - 204 lines** 💀💀
   - **Risk**: Worker task processing pipeline  
   - **Impact**: EXTREME - worker reliability depends on this
   - **Complexity**: Task claiming, execution, state management

### 🔥 **CRITICAL PRIORITY (100-199 lines) - High Business Impact**

4. **`internal/jobs/worker.go:processTask()` - 162 lines** 🔥🔥
   - **Risk**: Individual task execution
   - **Impact**: EXTREME - determines crawl results
   - **Complexity**: HTTP handling, cache validation, metrics

5. **`internal/jobs/manager.go:processSitemap()` - 111 lines** 🔥🔥
   - **Risk**: Sitemap processing logic
   - **Impact**: HIGH - URL discovery reliability
   - **Complexity**: Sitemap parsing, URL filtering, database operations

6. **`internal/jobs/worker.go:checkForPendingTasks()` - 110 lines** 🔥🔥
   - **Risk**: Worker coordination
   - **Impact**: HIGH - scaling and performance
   - **Complexity**: Job queue management, worker scaling

### ⚠️ **HIGH PRIORITY (50-99 lines) - Moderate Impact**

**25 additional functions** between 50-99 lines requiring attention:

**Worker Management (internal/jobs/worker.go):**
- `AddJob()` - 91 lines
- `recoverRunningJobs()` - 85 lines  
- `evaluateJobPerformance()` - 77 lines
- `flushBatches()` - 74 lines
- `worker()` - 61 lines
- `NewWorkerPool()` - 61 lines
- `listenForNotifications()` - 62 lines
- `recoverStaleTasks()` - 57 lines

**Job Management (internal/jobs/manager.go):**
- `GetJob()` - 70 lines
- `CancelJob()` - 69 lines
- `enqueueURLsForJob()` - 65 lines
- `StartJob()` - 59 lines
- `validateRootURLAccess()` - 59 lines (just created!)
- `EnqueueJobURLs()` - 57 lines
- `discoverAndParseSitemaps()` - 57 lines

**API Handlers (internal/api/jobs.go):**
- `updateJob()` - 80 lines
- `formatTasksFromRows()` - 72 lines (just created!)
- `listJobs()` - 69 lines
- `createJob()` - 69 lines
- `getJob()` - 63 lines
- `parseTaskQueryParams()` - 62 lines (just created!)
- `getJobTasks()` - 56 lines (already refactored!)

## setupSchema() REFACTOR PLAN

### Function Analysis (internal/db/db.go:setupSchema - 216 lines)

**Current structure identification:**
- Table creation statements (~80 lines)
- Index creation for performance (~60 lines)  
- Row Level Security setup (~40 lines)
- Trigger creation (~36 lines)

**Target breakdown:**
1. **`createCoreTables()`** - Main table creation (~80 lines)
2. **`createPerformanceIndexes()`** - Index creation (~60 lines)
3. **`setupRowLevelSecurity()`** - RLS policies (~40 lines)
4. **`createDatabaseTriggers()`** - Trigger setup (~36 lines)
5. **`setupSchema()`** - Clean orchestrator (~15 lines)

### Execution Steps

#### Step 1: Extract Table Creation ⏳
- [ ] Analyse table creation section (lines ~241-320)
- [ ] Create `createCoreTables(db *sql.DB) error` function
- [ ] Add comprehensive tests for table creation
- [ ] Verify build and existing tests pass
- [ ] Commit: "Extract table creation from setupSchema"

#### Step 2: Extract Index Creation ⏳
- [ ] Analyse index creation section (lines ~321-380)
- [ ] Create `createPerformanceIndexes(db *sql.DB) error` function
- [ ] Add tests for index creation and constraints
- [ ] Verify build and existing tests pass
- [ ] Commit: "Extract index creation from setupSchema"

#### Step 3: Extract RLS Setup ⏳
- [ ] Analyse RLS section (lines ~381-420)
- [ ] Create `setupRowLevelSecurity(db *sql.DB) error` function
- [ ] Add tests for security policy creation
- [ ] Verify build and existing tests pass
- [ ] Commit: "Extract RLS setup from setupSchema"

#### Step 4: Extract Trigger Creation ⏳
- [ ] Analyse trigger section (lines ~421-456)
- [ ] Create `createDatabaseTriggers(db *sql.DB) error` function
- [ ] Add tests for trigger functionality
- [ ] Verify build and existing tests pass
- [ ] Commit: "Extract trigger creation from setupSchema"

#### Step 5: Simplify Schema Orchestrator ⏳
- [ ] Rewrite `setupSchema()` to coordinate extracted functions
- [ ] Target: ~15 lines of clean orchestration
- [ ] Add integration test for full schema setup
- [ ] Verify build and existing tests pass
- [ ] Commit: "Simplify setupSchema to orchestrator"

### Success Criteria
- **216 → ~15 lines** (93% reduction)
- **4 focused, testable functions** created
- **Database operations fully tested** with proper mocking
- **Zero regressions** in schema creation
- **Improved DB package coverage** from 30.5% baseline

### Next Targets (After setupSchema)
1. **`processNextTask()`** (204 lines) - Worker pipeline critical path
2. **`processTask()`** (162 lines) - Task execution logic
3. **Continue WarmURL breakdown** - 7 more functions needed
4. **`processSitemap()`** (111 lines) - Sitemap processing

---
**Status**: Ready to tackle database foundation | **Next**: Extract table creation from setupSchema