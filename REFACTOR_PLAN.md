# Blue Banded Bee Refactor Plan

## Proven Methodology

The **Extract + Test + Commit** pattern has been successfully demonstrated:

✅ **Incremental approach** - Small, safe steps with immediate validation
✅ **Test-driven refactoring** - Test each extracted function comprehensively  
✅ **Clean commits** - Each step separately committed and reversible
✅ **Preserve functionality** - Zero regressions, all existing tests pass
✅ **Quality over speed** - Thorough understanding before changes

### Success Pattern Established

**Previous Achievements:**
- **`getJobTasks`**: 216 → 56 lines (74% reduction) with 4 focused functions
- **`CreateJob`**: 232 → 42 lines (82% reduction) with 4 focused functions
- **145+ test cases** added with comprehensive coverage
- **Zero functional regressions** across entire codebase

## CURRENT STATE ANALYSIS

**Large Files Remaining:**
- `internal/jobs/worker.go` - **1561 lines, 28 functions**
- `internal/jobs/manager.go` - **1017 lines, 23 functions** 
- `internal/api/jobs.go` - **801 lines, 14 functions**
- `internal/db/db.go` - **750 lines, 14 functions**

## IMMEDIATE PRIORITY: Fix Recent Extraction Issues

### Problem Identified
The `setupJobURLDiscovery()` function we just extracted is **108 lines** - another monster function! 
This violates the single responsibility principle and needs further breakdown.

### Step 1: Break Down setupJobURLDiscovery (108 lines) ✅

**COMPLETED**: Successfully refactored 108-line function

**Functions Created:**
1. ✅ **`validateRootURLAccess()`** - Robots.txt checking and crawl delay (58 lines)
2. ✅ **`createManualRootTask()`** - Database operations for root URL (42 lines)  
3. ✅ **`setupJobURLDiscovery()`** - Clean orchestrator (17 lines)

**Results:**
- **108 → 17 lines** (84% reduction)
- **2 focused, testable functions** created
- **35+ test cases** added with comprehensive coverage
- **Zero functional regressions**

### Step 2: Break Down processSitemap (111 lines) ⏳

**Current function structure (lines 907-1017):**
- Sitemap discovery (~20 lines)
- URL filtering and validation (~30 lines)  
- Database operations (~40 lines)
- Error handling (~21 lines)

**Target extraction:**

1. **`discoverSitemapURLs()`** - Sitemap discovery and parsing (20 lines)
2. **`filterAndValidateURLs()`** - URL filtering against robots.txt (30 lines)
3. **`enqueueSitemapTasks()`** - Database operations for sitemap URLs (40 lines)
4. **`processSitemap()`** - Clean orchestrator (~15 lines)

## HIGH-IMPACT MONSTER FUNCTIONS (Next Targets)

### CRITICAL PRIORITY: Core Worker Logic

1. **`internal/jobs/worker.go:processNextTask()` - 204 lines** 🔥
   - **Risk**: Core task processing pipeline
   - **Impact**: Very High - worker reliability depends on this
   - **Target**: Break into 5 functions (claim, execute, update, error handling, orchestrate)

2. **`internal/jobs/worker.go:processTask()` - 162 lines** 🔥  
   - **Risk**: Individual task execution logic
   - **Impact**: Very High - determines crawl success/failure
   - **Target**: Break into 4 functions (validate, crawl, analyse, orchestrate)

### HIGH PRIORITY: Database Schema

3. **`internal/db/db.go:setupSchema()` - 216 lines** ⚠️
   - **Risk**: Schema management complexity
   - **Impact**: Medium - affects all database operations
   - **Target**: Break into 4 functions (tables, indexes, policies, orchestrate)

### MEDIUM PRIORITY: Worker Management

4. **`internal/jobs/worker.go:checkForPendingTasks()` - 110 lines**
   - **Impact**: Medium - affects scaling and performance
   - **Target**: Break into 3 functions (identify, scale, cleanup)

5. **`internal/jobs/worker.go:AddJob()` - 91 lines**
   - **Impact**: Medium - affects job startup
   - **Target**: Break into 3 functions (validate, assign, configure)

## EXECUTION PLAN

### Immediate (This Session)
1. **Fix `setupJobURLDiscovery`** - Break 108 lines into 4 focused functions
2. **Fix `processSitemap`** - Break 111 lines into 4 focused functions
3. **Test extensively** - Follow proven pattern
4. **Verify no regressions** - Full test suite

### Next Session Priorities
1. **`processNextTask()`** (204 lines) - Highest impact target
2. **`processTask()`** (162 lines) - Critical task execution
3. **`setupSchema()`** (216 lines) - Database foundation

### Success Metrics
- **Target**: Each function <50 lines
- **Coverage**: 80-90% on extracted functions
- **Quality**: Comprehensive test suites with edge cases
- **Reliability**: Zero functional regressions

## File Structure Targets

**Long-term goals:**
- `internal/jobs/manager.go`: 1017 → ~400 lines
- `internal/jobs/worker.go`: 1561 → ~600 lines  
- `internal/db/db.go`: 750 → ~300 lines

**Strategy**: Continue the proven **Extract + Test + Commit** methodology that has delivered exceptional results.

---
**Status**: Ready to continue breaking down monster functions | **Next**: Fix setupJobURLDiscovery (108 lines)