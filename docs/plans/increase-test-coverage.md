# Implementation Plan: Increase Test Coverage

**Objective:** Incrementally increase test coverage for the `internal/jobs` package using a Supabase branch database for simplified integration testing.

This plan focuses on creating simple, practical integration tests that verify core functionality with minimal complexity.

**Note:** This integration testing approach complements the unit testing strategy in `unit-testing-with-testify.md`:
- **Integration tests (this plan)**: Test database operations and basic flows with real DB
- **Unit tests (other plan)**: Test complex business logic with mocks
- Together they provide comprehensive coverage

---

## Overall Approach

- **Complexity:** Low to Medium. By using a Supabase branch database, we eliminate the need for mocking and complex test setup.
- **Method:** We will use a Supabase branch database that mirrors production. For each test:
  1. Use the branch database with existing schema and stored procedures
  2. Execute the function being tested
  3. Query the database to verify the expected state changes
  4. Reset branch data between test runs if needed
- **Dependencies:** No mocking required - we'll use real HTTP requests to test domains when needed

---

## Supabase Branch Setup Instructions

### 1. Create a Development Branch

1. Log into your Supabase project dashboard
2. Navigate to the "Branches" section
3. Click "Create branch" and name it `test-branch` or similar
4. Wait for the branch to be provisioned (usually 1-2 minutes)
5. Copy the branch connection string from the Settings → Database page

### 2. Configure Test Environment

Create a `.env.test` file in your project root:
```bash
TEST_DATABASE_URL=<your-branch-connection-string>
```

### 3. Test Execution

Run tests with the test environment:
```bash
# Option 1: Use the test script
./scripts/test-db.sh

# Option 2: Manual command
set -a; source .env; source .env.test; set +a
export DATABASE_URL="$TEST_DATABASE_URL"
go test ./internal/jobs/... -v
```

---

## Simplified Test Plan

### Phase 1: Basic Tests (In Progress)

| Test Function | Est. Lines | Complexity | Purpose | Status |
| :------------ | :--------- | :--------- | :------ | :----- |
| **`TestGetJob`** | 10-15 lines | **Low** | Verify basic read operations work | ✅ Complete (42 lines) |
| **`TestCreateJob`** | 20-30 lines | **Low** | Test job creation without sitemap complexity | 🔴 Next |
| **`TestCancelJob`** | 20-30 lines | **Low** | Test state transitions | ⚪ Pending |
| **`TestProcessSitemapFallback`** | 30-40 lines | **Medium** | Test your new fallback feature | ⚪ Pending |

**Phase 1 Total:** ~80-115 lines (much simpler than original 350-470)

### Phase 2: Extended Tests (Future)

| Test Function | Est. Lines | Complexity | Purpose |
| :------------ | :--------- | :--------- | :------ |
| **`TestEnqueueJobURLs`** | 30-40 lines | **Medium** | Test duplicate detection |
| **`TestProcessTask`** | 40-50 lines | **Medium** | Test single task processing with real HTTP |
| **`TestWorkerPool_Basic`** | 30-40 lines | **Medium** | Basic worker pool functionality |

### Test Implementation Pattern

```go
func TestGetJob(t *testing.T) {
    // 1. Connect to test database
    db, err := db.InitFromEnv()
    require.NoError(t, err)
    
    // 2. Create test data
    jobID := createTestJob(t, db, "test.example.com")
    
    // 3. Execute function
    jm := jobs.NewJobManager(db, nil, nil)
    job, err := jm.GetJob(context.Background(), jobID)
    
    // 4. Assert results
    require.NoError(t, err)
    assert.Equal(t, jobID, job.ID)
    assert.Equal(t, "test.example.com", job.DomainName)
}
```

---

## Benefits of This Approach

1. **No Mocking Required** - Use real crawler for actual HTTP requests
2. **Real Database Features** - All triggers, stored procedures work as in production
3. **Simple Setup** - Just set TEST_DATABASE_URL and run
4. **Incremental Progress** - Start with 100 lines instead of 500
5. **Production-Like** - Tests run against exact same schema as production

---

## Summary

- **Phase 1:** 80-115 lines of simple tests (30 minutes to implement)
- **Phase 2:** Additional 100-130 lines when needed
- **Total Time:** Phase 1 can be completed in under an hour

This approach provides immediate value with minimal complexity, allowing for incremental improvement of test coverage.
