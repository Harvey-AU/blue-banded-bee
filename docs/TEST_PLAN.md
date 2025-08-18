# Testing Guide

## Testing Patterns

### **Interface-Based Testing**
```go
// For API endpoints using dependency injection
mockDB := new(MockDBClient)
mockJobsManager := new(MockJobManager)
handler := NewHandler(mockDB, mockJobsManager)

mockDB.On("GetOrCreateUser", userID, email, nil).Return(user, nil)
mockJobsManager.On("CreateJob", ctx, opts).Return(job, nil)
```

### **Sqlmock Testing**
```go
// For functions with direct SQL queries
mockSQL, mock, err := sqlmock.New()
defer mockSQL.Close()

mock.ExpectQuery(`SELECT.*FROM jobs WHERE id = \$1`).
    WithArgs(jobID).WillReturnRows(rows)
```

### **Extract + Test + Commit**
```go
// For refactoring large functions
func extractedFunction(params) result {
    // Single responsibility logic
}

func TestExtractedFunction(t *testing.T) {
    // Table-driven tests with edge cases
}
```

## File Structure

- `test_mocks.go` - Shared mocks and utilities
- `*_test.go` - Focused tests per endpoint/function group
- Use sqlmock for direct SQL query testing
- Use interface mocks for dependency injection testing

## Coverage Targets

- **New functions**: Aim for 80%+ coverage
- **Existing functions**: Improve opportunistically during feature work
- **Integration tests**: Focus on user workflows