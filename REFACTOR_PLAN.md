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

### Step 2: Extract Auth/Validation ⏳
- [ ] Create `validateJobAccess(ctx context.Context, h *Handler, userClaims *auth.UserClaims, jobID string) (*db.User, error)`
- [ ] Move lines ~521-549 (auth and job ownership validation)
- [ ] Test: Verify auth logic works
- [ ] Commit: "Extract job access validation from getJobTasks"

### Step 3: Extract Query Building ⏳
- [ ] Create `buildTaskQuery(jobID, status, sort string) (query string, args []interface{})`
- [ ] Move query construction logic (estimate lines ~570-650)
- [ ] Test: Verify query results match
- [ ] Commit: "Extract query building from getJobTasks"

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
**Status**: Planning ⏳ | **Next**: Step 1 - Extract Parameter Parsing