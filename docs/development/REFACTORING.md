# Refactoring Methodology

## Extract + Test + Commit Pattern

Blue Banded Bee has established a proven methodology for systematically refactoring large functions into focused, testable units.

## When to Refactor

Apply this methodology when encountering:
- **Functions >50 lines** (ideal target: <50 lines)
- **Multiple responsibilities** in a single function
- **Difficult to test** functions due to complexity
- **High cyclomatic complexity** functions

## Systematic Approach

### 1. Analyse Function Structure

**Map Responsibilities:**
```bash
# Example: 216-line function analysis
Lines 50-90:   Authentication logic
Lines 91-130:  Parameter parsing 
Lines 131-190: Query building
Lines 191-216: Response formatting
```

**Identify Boundaries:**
- Clear input/output for each section
- Minimal data dependencies between sections
- Logical groupings of related operations

### 2. Extract Focused Functions

**Create Single-Responsibility Functions:**

```go
// Before: 216-line monster
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
    // All logic mixed together...
}

// After: Clean orchestrator + focused functions
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
    user := h.validateJobAccess(w, r, jobID)
    if user == nil { return }
    
    params := parseTaskQueryParams(r)
    queries := buildTaskQuery(jobID, params)
    tasks, err := formatTasksFromRows(rows)
    
    // Return response...
}
```

**Function Naming Conventions:**
- `validateJobAccess()` - Validation functions
- `parseTaskQueryParams()` - Parsing functions  
- `buildTaskQuery()` - Construction functions
- `formatTasksFromRows()` - Formatting functions
- `handleTaskError()` - Error handling functions

### 3. Create Comprehensive Tests

**Table-Driven Test Pattern:**

```go
func TestParseTaskQueryParams(t *testing.T) {
    tests := []struct {
        name     string
        url      string
        expected TaskQueryParams
    }{
        {
            name: "default_values",
            url:  "/v1/jobs/123/tasks",
            expected: TaskQueryParams{
                Limit:   50,
                Offset:  0,
                Status:  "",
                OrderBy: "t.created_at DESC",
            },
        },
        {
            name: "custom_parameters",
            url:  "/v1/jobs/123/tasks?limit=100&offset=20&status=completed",
            expected: TaskQueryParams{
                Limit:   100,
                Offset:  20,
                Status:  "completed",
                OrderBy: "t.created_at DESC",
            },
        },
        // Edge cases, validation, error conditions...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(http.MethodGet, tt.url, nil)
            result := parseTaskQueryParams(req)
            
            assert.Equal(t, tt.expected.Limit, result.Limit)
            assert.Equal(t, tt.expected.Offset, result.Offset)
            assert.Equal(t, tt.expected.Status, result.Status)
            assert.Equal(t, tt.expected.OrderBy, result.OrderBy)
        })
    }
}
```

**Database Testing with sqlmock:**

```go
func TestCreateCoreTables(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    // Test table creation order and error handling
    mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations").
        WillReturnResult(sqlmock.NewResult(0, 0))
    mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
        WillReturnResult(sqlmock.NewResult(0, 0))

    err = createCoreTables(db)
    assert.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### 4. Commit Each Step

**Atomic Commits:**

```bash
git add internal/api/jobs.go internal/api/parse_task_query_params_test.go
git commit -m "Extract parameter parsing from getJobTasks

Creates parseTaskQueryParams function to handle URL query parameter
parsing and validation. Reduces getJobTasks complexity by ~40 lines.
Adds comprehensive tests for parameter validation and edge cases.
"
```

**Commit Message Pattern:**
- Subject: What was extracted from what
- Body: Brief description of the extracted function
- Impact: Lines reduced and testing added

### 5. Verify Integration

**Comprehensive Verification:**

```bash
# Build verification
go build ./...

# Unit test verification  
go test -v -short ./...

# Integration test verification
go test -v ./...

# Coverage verification
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep "total"
```

## Success Metrics

### Quantitative Results

**Function Complexity Reduction:**
- Target: >50 lines → <50 lines per function
- Achieved: 80% average reduction across 5 functions

**Test Coverage Improvement:**
- Target: 80-90% coverage on core functions
- Achieved: 38.9% total coverage (up from 30%)

**Function Creation:**
- Created: 23 focused, testable functions
- Added: 350+ comprehensive test cases

### Qualitative Benefits

**Maintainability:**
- Clear separation of concerns
- Single responsibility principle
- Easy to understand and modify

**Testability:**
- Each function easily unit tested
- Comprehensive edge case coverage
- Isolated testing of business logic

**Reliability:**
- Zero functional regressions
- Better error handling patterns
- Proper context management

## Proven Results

### Functions Successfully Refactored

1. **`getJobTasks`**: 216 → 56 lines (74% reduction) + 4 functions
2. **`CreateJob`**: 232 → 42 lines (82% reduction) + 4 functions  
3. **`setupJobURLDiscovery`**: 108 → 17 lines (84% reduction) + 2 functions
4. **`setupSchema`**: 216 → 27 lines (87% reduction) + 3 functions
5. **`WarmURL`**: 377 → 68 lines (82% reduction) + 5 functions

### Next Targets Identified

**Remaining monster functions:**
- `processNextTask()` - 204 lines (worker pipeline)
- `processTask()` - 162 lines (task execution)
- `processSitemap()` - 111 lines (sitemap processing)
- `checkForPendingTasks()` - 110 lines (worker coordination)

The methodology is **proven scalable** and ready for continued application.