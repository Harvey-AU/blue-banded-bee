# Unit Testing Strategy with Testify

**Status:** ACTIVE - Complements the integration testing approach in `increase-test-coverage.md`

**Approach:** This plan covers unit tests for complex business logic using mocks, while `increase-test-coverage.md` covers integration tests with real database operations. Both are needed for comprehensive coverage.

## Overview

This document outlines the plan to improve test coverage in Blue Banded Bee by introducing the testify framework for unit testing with mocks. The current codebase is under-tested, particularly for complex business logic that involves multiple dependencies.

## Current State

- **Test Coverage**: Limited, mostly simple table-driven tests
- **Mocking**: None - making it difficult to test complex functions in isolation
- **Dependencies**: Standard library only
- **Complex Logic**: Untested due to database, crawler, and external dependencies

## Priority Tests to Implement

### 1. JobManager.processSitemap (Sitemap Fallback)

**File**: `internal/jobs/manager.go:542-804`

**Why Priority**:
- Recently added fallback logic needs verification
- Complex branching: sitemap found vs fallback to root page
- Critical for job creation flow

**Test Cases**:
1. **Sitemap found with URLs** - verify normal processing
2. **Empty sitemap** - verify fallback to root page creation
3. **Sitemap discovery error** - verify error handling
4. **Domain ID retrieval failure** - verify transaction rollback

**Dependencies to Mock**:
- `DbQueueProvider` - database operations
- `crawler.Crawler` - sitemap discovery and parsing

**Key Assertions**:
- When no sitemap URLs found, root page task created with priority 1.0
- Source type set to "fallback" vs "sitemap"
- Job statistics recalculated after task creation

### 2. JobManager.CreateJob

**File**: `internal/jobs/manager.go:52-226`

**Why Priority**:
- Core business logic with multiple paths
- Handles duplicate job detection and cancellation
- Transaction management critical for data integrity

**Test Cases**:
1. **New job creation** - verify successful creation
2. **Duplicate job exists** - verify old job cancelled, new created
3. **Domain creation failure** - verify transaction rollback
4. **Sitemap vs manual job** - verify different initialization paths

**Dependencies to Mock**:
- `DbQueueProvider` - database operations
- `WorkerPool` - job lifecycle management
- Direct `*sql.DB` queries for duplicate detection

**Key Assertions**:
- Existing active jobs cancelled before creating new
- Domain created or retrieved correctly
- Job inserted with correct initial state
- Sitemap processing triggered asynchronously when enabled

### 3. WorkerPool.processTask

**File**: `internal/jobs/worker.go:954-1077`

**Why Priority**:
- Core crawling logic with link discovery
- Complex priority calculation for discovered links
- Performance metrics collection

**Test Cases**:
1. **Successful crawl with links** - verify link discovery and priority assignment
2. **Crawl failure** - verify error handling and retry logic
3. **Homepage link prioritisation** - verify header/footer/body link priorities
4. **Non-homepage link processing** - verify only body links processed

**Dependencies to Mock**:
- `crawler.Crawler` - URL warming and link discovery
- `DbQueueProvider` - task updates and link enqueueing
- `JobManager` - for EnqueueJobURLs callback

**Key Assertions**:
- Header links get priority 1.0 to 0.991 (homepage only)
- Footer links get priority 0.990 to 0.981 (homepage only)
- Body links get parent priority × 0.9
- Discovered links filtered for same domain
- Task marked complete with correct metrics

### 4. API Handler - createJob

**File**: `internal/api/jobs.go`

**Why Priority**:
- User-facing endpoint needing robust error handling
- Authentication and authorisation logic
- Input validation and response formatting

**Test Cases**:
1. **Valid job creation** - verify successful response
2. **Missing authentication** - verify 401 response
3. **Invalid domain** - verify 400 response
4. **Database error** - verify 500 response and error format

**Dependencies to Mock**:
- `JobManager` - job creation
- `db.DB` - user lookup
- Auth middleware context

**Key Assertions**:
- Authentication properly validated
- Organisation ID correctly passed
- Response format matches API contract
- Error responses include request ID

## Implementation Approach

### 1. Directory Structure
```
internal/
├── mocks/
│   ├── db_queue.go      # DbQueueProvider mock
│   ├── crawler.go       # Crawler mock
│   └── worker_pool.go   # WorkerPool mock
├── jobs/
│   ├── manager_test.go  # Unit tests with mocks
│   └── worker_test.go   # Unit tests with mocks
└── api/
    └── jobs_test.go     # API handler tests
```

### 2. Mock Generation

Using testify's mock package:
```go
type MockDbQueue struct {
    mock.Mock
}

func (m *MockDbQueue) Execute(ctx context.Context, fn func(*sql.Tx) error) error {
    args := m.Called(ctx, fn)
    return args.Error(0)
}

func (m *MockDbQueue) EnqueueURLs(ctx context.Context, jobID string, pages []db.Page, sourceType string, sourceURL string) error {
    args := m.Called(ctx, jobID, pages, sourceType, sourceURL)
    return args.Error(0)
}
```

### 3. Test Pattern

```go
func TestJobManager_ProcessSitemap_Fallback(t *testing.T) {
    // Arrange
    ctx := context.Background()
    mockQueue := new(MockDbQueue)
    mockCrawler := new(MockCrawler)
    
    jm := &JobManager{
        dbQueue: mockQueue,
        processedPages: make(map[string]map[int]bool),
    }
    
    // Set expectations
    mockCrawler.On("DiscoverSitemaps", ctx, "example.com").Return([]string{}, nil)
    mockQueue.On("Execute", ctx, mock.AnythingOfType("func(*sql.Tx) error")).Return(nil)
    mockQueue.On("EnqueueURLs", ctx, "job-123", mock.MatchedBy(func(pages []db.Page) bool {
        return len(pages) == 1 && pages[0].Path == "/" && pages[0].Priority == 1.0
    }), "fallback", "https://example.com").Return(nil)
    
    // Act
    jm.processSitemap(ctx, "job-123", "example.com", nil, nil)
    
    // Assert
    mockCrawler.AssertExpectations(t)
    mockQueue.AssertExpectations(t)
}
```

### 4. Integration with Existing Tests

- Keep existing simple tests using standard library
- Add `//go:build unit` build tag for unit tests
- Add `//go:build integration` for integration tests
- CI pipeline runs both: `go test -tags=unit,integration ./...`

## Benefits

1. **Isolation**: Test complex logic without external dependencies
2. **Speed**: Unit tests run in milliseconds vs seconds for integration tests
3. **Coverage**: Can test error paths that are hard to trigger
4. **Confidence**: Refactoring is safer with comprehensive tests
5. **Documentation**: Tests serve as usage examples

## Success Metrics

- Increase test coverage from current ~30% to 70%+
- All critical business logic has unit tests
- Test execution time under 5 seconds for unit tests
- No flaky tests due to external dependencies

## Next Steps

1. Add testify dependency: `go get github.com/stretchr/testify`
2. Create mock implementations in `internal/mocks/`
3. Implement priority tests in order listed above
4. Add build tags to separate unit and integration tests
5. Update CI pipeline to run both test suites
6. Document testing patterns for team consistency