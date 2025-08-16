# Writing Tests

## Testing Philosophy

Blue Banded Bee follows a **focused, testable function** approach established through systematic refactoring. Each function should be small, have a single responsibility, and be thoroughly tested.

## Test Patterns

### Focused Unit Tests

With refactored functions, unit testing becomes straightforward:

```go
func TestValidateCrawlRequest(t *testing.T) {
    tests := []struct {
        name        string
        targetURL   string
        expectError bool
    }{
        {
            name:        "valid_https_url",
            targetURL:   "https://example.com",
            expectError: false,
        },
        {
            name:        "invalid_url_format",
            targetURL:   "not-a-url",
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parsedURL, err := validateCrawlRequest(context.Background(), tt.targetURL)
            
            if tt.expectError {
                assert.Error(t, err)
                assert.Nil(t, parsedURL)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, parsedURL)
            }
        })
    }
}
```

### Database Testing with Mocks

For database operations, use sqlmock for unit testing:

```go
func TestCreateCoreTables(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    // Expect table creation in dependency order
    mock.ExpectExec("CREATE TABLE IF NOT EXISTS organisations").
        WillReturnResult(sqlmock.NewResult(0, 0))
    mock.ExpectExec("CREATE TABLE IF NOT EXISTS users").
        WillReturnResult(sqlmock.NewResult(0, 0))

    err = createCoreTables(db)
    assert.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### Integration Tests

For full system testing with real database:

```go
func TestCreateJob(t *testing.T) {
    database := setupTest(t)  // Loads .env.test
    defer database.Close()
    
    ctx := context.Background()
    jm := NewJobManager(database.GetDB(), dbQueue, nil, nil)
    
    job, err := jm.CreateJob(ctx, &JobOptions{
        Domain: "test.example.com",
    })
    
    require.NoError(t, err)
    assert.Equal(t, "test.example.com", job.Domain)
    
    // Cleanup
    _, err = database.GetDB().ExecContext(ctx, 
        "DELETE FROM jobs WHERE id = $1", job.ID)
    require.NoError(t, err)
}
```

## Test Helpers

### Database Setup

Use the standard helper in `manager_test.go`:

```go
func setupTest(t *testing.T) *db.DB {
    t.Helper()
    testutil.LoadTestEnv(t)
    
    database, err := db.InitFromEnv()
    require.NoError(t, err, "Failed to connect to test database")
    return database
}
```

### HTTP Testing

For API endpoints:

```go
func TestHealthEndpoint(t *testing.T) {
    req := httptest.NewRequest("GET", "/health", nil)
    w := httptest.NewRecorder()
    
    handler := NewHandler(nil, nil, nil)
    handler.HealthCheck(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

## Function Refactoring for Testability

### Extract + Test + Commit Methodology

When encountering large functions (>50 lines), apply systematic refactoring:

1. **Analyse Structure**: Identify distinct responsibilities within the function
2. **Extract Functions**: Pull out focused, single-responsibility functions  
3. **Create Tests**: Write comprehensive tests for each extracted function
4. **Commit Changes**: Commit each extraction separately for safety
5. **Verify Integration**: Ensure original function still works correctly

### Example Refactoring Pattern

**Before (216 lines):**
```go
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
    // 50 lines of auth logic
    // 40 lines of parameter parsing  
    // 60 lines of query building
    // 66 lines of response formatting
}
```

**After (56 lines):**
```go
func (h *Handler) getJobTasks(w http.ResponseWriter, r *http.Request, jobID string) {
    user := h.validateJobAccess(w, r, jobID)
    if user == nil { return }
    
    params := parseTaskQueryParams(r)
    queries := buildTaskQuery(jobID, params)
    
    // Execute queries...
    tasks, err := formatTasksFromRows(rows)
    if err != nil { return }
    
    // Return response...
}
```

**Benefits:**
- Each function <50 lines and easily testable
- Single responsibility principle
- Comprehensive test coverage possible
- Future changes isolated to specific functions

### Testing Extracted Functions

Each extracted function should have comprehensive tests:

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
        // More test cases...
    }
    // Table-driven test implementation...
}
```

## Best Practices

1. **Function Size**: Keep functions under 50 lines where possible
2. **Single Responsibility**: Each function should do one thing well
3. **Testability**: Design functions to be easily testable
4. **Error Handling**: Return simple errors, let callers handle complex responses
5. **Context Handling**: Use appropriate contexts (request vs background)
6. **Cleanup**: Always clean up test data
7. **Isolation**: Each test should be independent
8. **Naming**: Use descriptive test names describing the scenario
9. **Assertions**: Use testify's require/assert consistently
10. **Table-Driven Tests**: Use for multiple similar scenarios

## Running Tests

```bash
# Run all tests
go test ./...

# Run unit tests only
go test -tags unit ./...

# Run specific test
go test -v -run TestCreateJob ./internal/jobs

# Run with race detector
go test -race ./...
```