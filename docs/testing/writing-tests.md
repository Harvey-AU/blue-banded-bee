# Writing Tests

## Test Patterns

### Integration Tests

Most tests use real database connections:

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

### Unit Tests with Mocks

For testing business logic without database:

```go
//go:build unit

func TestProcessTask_Unit(t *testing.T) {
    mockCrawler := &mocks.MockCrawler{
        WarmURLFunc: func(ctx context.Context, url string) (*CrawlResult, error) {
            return &CrawlResult{StatusCode: 200}, nil
        },
    }
    
    // Test with mock
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

## Best Practices

1. **Cleanup**: Always clean up test data
2. **Isolation**: Each test should be independent
3. **Naming**: Use descriptive test names
4. **Assertions**: Use testify's require/assert
5. **Context**: Pass context.Background() for database operations

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