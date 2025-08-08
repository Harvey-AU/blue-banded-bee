# Testing Style Guide

## Overview

This guide documents the testing conventions and patterns used in the Blue Banded Bee project to ensure consistent, maintainable, and comprehensive test coverage.

## Testing Frameworks

- **Test Framework**: Standard Go testing package
- **Assertions**: [testify](https://github.com/stretchr/testify) - `assert` and `require`
- **HTTP Testing**: `net/http/httptest`
- **Mocking**: Custom mocks in `internal/mocks` (no external mocking framework)

## Test Organisation

### File Naming
- Test files must end with `_test.go`
- Integration test files may use `_integration_test.go` suffix
- Mock implementations go in `internal/mocks/`

### Package Structure
```
internal/
  api/
    handlers.go
    handlers_test.go     # Unit tests
    errors.go
    errors_test.go       # Unit tests
  jobs/
    manager.go
    manager_test.go      # Integration tests (uses real DB)
    worker.go
    worker_test.go       # Unit tests
```

## Test Patterns

### 1. Table-Driven Tests (Preferred)

**Always use table-driven tests for testing multiple scenarios:**

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input returns expected output",
            input:    "test",
            expected: "TEST",
            wantErr:  false,
        },
        {
            name:     "empty input returns error",
            input:    "",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 2. Test Naming Conventions

**Test names should be descriptive and follow these patterns:**

- Use underscores for readability in subtest names
- Describe the scenario and expected outcome
- Start with the condition, end with the expectation

Good examples:
- `"user_with_valid_token_should_be_authorised"`
- `"nil_input_should_return_error"`
- `"empty_array_should_return_zero_count"`
- `"concurrent_requests_should_not_cause_race_condition"`

### 3. HTTP Handler Testing

```go
func TestHTTPHandler(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        path           string
        body           interface{}
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "valid_request_returns_success",
            method:         http.MethodPost,
            path:           "/api/endpoint",
            body:           map[string]string{"key": "value"},
            expectedStatus: http.StatusOK,
            expectedBody:   `{"status":"success"}`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Prepare request
            var bodyReader io.Reader
            if tt.body != nil {
                bodyBytes, _ := json.Marshal(tt.body)
                bodyReader = bytes.NewReader(bodyBytes)
            }
            
            req := httptest.NewRequest(tt.method, tt.path, bodyReader)
            rec := httptest.NewRecorder()
            
            // Add context if needed
            ctx := context.WithValue(req.Context(), requestIDKey, "test-123")
            req = req.WithContext(ctx)
            
            // Call handler
            handler(rec, req)
            
            // Assert
            assert.Equal(t, tt.expectedStatus, rec.Code)
            if tt.expectedBody != "" {
                assert.JSONEq(t, tt.expectedBody, rec.Body.String())
            }
        })
    }
}
```

### 4. Edge Case Coverage

**Always test these edge cases:**

- Nil inputs
- Empty strings/slices/maps
- Invalid types (when using interface{})
- Boundary values
- Unicode and special characters
- Concurrent access (where applicable)
- Very large inputs

Example:
```go
func TestEdgeCases(t *testing.T) {
    t.Run("nil_input", func(t *testing.T) {
        result := Function(nil)
        assert.Equal(t, expectedDefault, result)
    })
    
    t.Run("empty_string", func(t *testing.T) {
        result := Function("")
        assert.Equal(t, expectedEmpty, result)
    })
    
    t.Run("unicode_characters", func(t *testing.T) {
        inputs := []string{
            "Hello 世界",
            "Test\n\t\r",
            "Emoji 🔥",
            "Zero\x00Byte",
        }
        for _, input := range inputs {
            result := Function(input)
            assert.NotPanics(t, func() { _ = result })
        }
    })
}
```

### 5. Context Handling

```go
func TestWithContext(t *testing.T) {
    ctx := context.Background()
    
    t.Run("respects_context_cancellation", func(t *testing.T) {
        ctx, cancel := context.WithCancel(ctx)
        cancel() // Cancel immediately
        
        err := FunctionWithContext(ctx)
        assert.ErrorIs(t, err, context.Canceled)
    })
    
    t.Run("respects_context_timeout", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
        defer cancel()
        
        time.Sleep(2 * time.Millisecond)
        err := FunctionWithContext(ctx)
        assert.ErrorIs(t, err, context.DeadlineExceeded)
    })
}
```

## Assertions

### Use `require` vs `assert`

- **`require`**: Use when the test cannot continue if the assertion fails
- **`assert`**: Use for non-critical assertions where the test can continue

```go
func TestExample(t *testing.T) {
    result, err := GetImportantData()
    require.NoError(t, err)  // Stop test if error occurs
    require.NotNil(t, result) // Stop test if result is nil
    
    assert.Equal(t, "expected", result.Field1) // Continue even if this fails
    assert.True(t, result.IsValid)            // Continue even if this fails
}
```

### Assertion Messages

Add descriptive messages to assertions when the failure reason might not be obvious:

```go
assert.Equal(t, expected, actual, 
    "Function(%v) returned %v, expected %v", input, actual, expected)
```

## Test Helpers

### Use `t.Helper()`

Mark helper functions with `t.Helper()` so test failures report the correct line:

```go
func setupTestServer(t *testing.T) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
}
```

### Cleanup with `t.Cleanup()` or `defer`

```go
func TestWithCleanup(t *testing.T) {
    resource := createResource(t)
    t.Cleanup(func() {
        resource.Close()
    })
    // or
    defer resource.Close()
    
    // Test code here
}
```

## Integration Tests

### Database Tests

```go
//go:build integration

func TestDatabaseOperation(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    db := setupTestDatabase(t)
    defer db.Close()
    
    ctx := context.Background()
    
    // Perform test
    err := db.CreateRecord(ctx, record)
    require.NoError(t, err)
    
    // Clean up
    t.Cleanup(func() {
        db.DeleteRecord(ctx, record.ID)
    })
}
```

## Performance Testing

### Benchmarks

```go
func BenchmarkFunction(b *testing.B) {
    // Setup code (not timed)
    data := prepareData()
    
    b.ResetTimer() // Start timing here
    
    for i := 0; i < b.N; i++ {
        Function(data)
    }
}

func BenchmarkParallel(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            Function()
        }
    })
}
```

### Race Condition Testing

```go
func TestConcurrentAccess(t *testing.T) {
    resource := NewResource()
    
    done := make(chan bool, 10)
    for i := 0; i < 10; i++ {
        go func(id int) {
            defer func() { done <- true }()
            
            // Concurrent operations
            resource.Write(id)
            resource.Read()
        }(i)
    }
    
    // Wait for all goroutines
    for i := 0; i < 10; i++ {
        <-done
    }
    
    // Verify state
    assert.Equal(t, expectedState, resource.State())
}
```

Run with: `go test -race ./...`

## Test Output

### Use `t.Log()` for Debugging

```go
t.Logf("Debug: received value = %+v", value)
```

### Warnings vs Failures

```go
if result.Performance < expected {
    t.Logf("Warning: Performance degraded: %v < %v", result.Performance, expected)
    // Don't fail the test for performance warnings in CI
}

if result.Critical != expected {
    t.Errorf("Critical check failed: got %v, want %v", result.Critical, expected)
    // This will fail the test
}
```

## Mock Patterns

### Interface-Based Mocks

```go
// In production code
type Crawler interface {
    WarmURL(ctx context.Context, url string) (*Result, error)
}

// In test code
type MockCrawler struct {
    WarmURLFunc func(ctx context.Context, url string) (*Result, error)
    Calls       []string // Track calls for verification
}

func (m *MockCrawler) WarmURL(ctx context.Context, url string) (*Result, error) {
    m.Calls = append(m.Calls, url)
    if m.WarmURLFunc != nil {
        return m.WarmURLFunc(ctx, url)
    }
    return &Result{Status: 200}, nil
}
```

## Coverage Goals

- **Target**: Minimum 80% coverage for critical paths
- **Focus Areas**:
  - Error handling paths
  - Business logic
  - API handlers
  - Data validation
- **Exclude from Coverage**:
  - Generated code
  - Simple getters/setters
  - Panic recovery code
  - Main function bootstrapping

## CI/CD Considerations

### Skip Flaky Tests in CI

```go
func TestFlakyExternal(t *testing.T) {
    if os.Getenv("CI") != "" {
        t.Skip("Skipping flaky external test in CI")
    }
    // Test that relies on external services
}
```

### Environment-Specific Tests

```go
func TestProductionOnly(t *testing.T) {
    if os.Getenv("ENVIRONMENT") != "production" {
        t.Skip("This test only runs in production environment")
    }
    // Production-specific test
}
```

## Common Pitfalls to Avoid

1. **Don't test implementation details** - Test behaviour, not implementation
2. **Don't use real external services** - Mock external dependencies
3. **Don't share state between tests** - Each test should be independent
4. **Don't ignore error returns** - Always check errors, even in tests
5. **Don't hardcode timeouts** - Use configurable timeouts for CI compatibility
6. **Don't forget cleanup** - Always clean up resources and test data
7. **Don't test the testing framework** - Focus on your code, not testify assertions

## Test Review Checklist

Before submitting tests for review:

- [ ] All tests pass locally
- [ ] Tests are independent and can run in any order
- [ ] Edge cases are covered
- [ ] Error paths are tested
- [ ] Test names are descriptive
- [ ] No hardcoded values that might break in different environments
- [ ] Cleanup is properly handled
- [ ] Race conditions are considered (run with `-race`)
- [ ] Documentation is updated if adding new test patterns