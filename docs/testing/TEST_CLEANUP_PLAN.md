# Test Cleanup Plan

## Executive Summary

The test suite was rapidly created over 48-72 hours to add coverage to an existing production application. This resulted in multiple conflicting approaches, broken tests, and architectural misunderstandings. This document provides a cleanup strategy to establish a coherent, maintainable test suite.

## Critical Issues Found

### 1. Broken Tests That Never Worked

- **File**: `internal/jobs/integration_test.go`
- **Problem**: Calls non-existent methods like `database.CreateJob()`, `database.CreateTask()`
- **Root Cause**: Tests written assuming DB has business logic methods that don't exist
- **Status**: These tests have NEVER successfully run
- **Action**: DELETE or completely rewrite using JobManager

### 2. Duplicate Test Functions

- **Example**: `TestRecoverStaleTasks` exists in both:
  - `internal/jobs/integration_test.go` (broken)
  - `internal/jobs/worker_advanced_test.go` (working)
- **Action**: Remove duplicates, keep working versions

### 3. Test File Sprawl

Current state (10+ test files for one package):

```
internal/jobs/
├── connection_test.go
├── constants_test.go
├── integration_test.go (BROKEN)
├── job_manager_unit_test.go
├── manager_mock_test.go
├── manager_test.go
├── types_test.go
├── worker_advanced_test.go
├── worker_lifecycle_test.go
└── worker_test.go
```

### 4. Mixed Testing Paradigms

Three conflicting approaches:

1. **Direct DB testing** - Assumes DB has business methods (WRONG)
2. **Manager testing** - Tests through JobManager (CORRECT)
3. **Mock testing** - Uses mocked dependencies (CORRECT for units)

### 5. Cross‑cutting Test Smells (to fix during cleanup)

- Conflicting expectations: a health DB test still expects a panic on `/health/db` when DB is nil. Current behaviour returns 503 Service Unavailable. Update the test to assert 503 and error payload (no panic).
- Brittle version assertions: some tests hard‑code `"0.4.0"`. Now `Version` is a variable (overridable via ldflags). Assert against `api.Version` or non‑empty string, not a literal.
- Incomplete DB mocks: handlers now depend on a `DBClient` interface. Ensure mocks implement all required methods (`GetOrCreateUser`, `GetJobStats`, `GetJobActivity`, `GetUserByWebhookToken`) or introduce a dedicated `MockDBClient`.
- Unsafe assertions in goroutines: avoid using `t.*` from goroutines. Collect results via channels and assert in the parent goroutine or use subtests with `t.Parallel()`.
- DB health test strategy: prefer `sqlmock` to exercise ping success/failure deterministically; avoid hitting a real DB in unit tests.
- DSN/statement_timeout tests: extract DSN augmentation into a small pure helper and unit test it directly to avoid side effects.

## Recommended Clean Structure

### Target Organisation

```
internal/jobs/
├── manager_test.go              # JobManager unit + integration tests
├── worker_test.go               # WorkerPool unit + integration tests
├── queue_test.go                # DbQueue tests (if needed)
├── types_test.go                # Type/serialisation tests (keep existing)
└── testhelpers_test.go          # Shared test utilities
```

### Delete These Files

```bash
# Broken/redundant files to remove
internal/jobs/integration_test.go        # Fundamentally broken
internal/jobs/manager_mock_test.go       # Merge useful parts into manager_test.go
internal/jobs/worker_advanced_test.go    # Merge into worker_test.go
internal/jobs/worker_lifecycle_test.go   # Merge into worker_test.go
internal/jobs/job_manager_unit_test.go   # Merge into manager_test.go
internal/jobs/connection_test.go         # If trivial, merge or delete
internal/jobs/constants_test.go          # If trivial, merge or delete
```

## Cleanup Actions

### Phase 1: Remove Broken Tests (Immediate)

```bash
# 1. Delete the broken integration test file
rm internal/jobs/integration_test.go

# 2. Check for any other files with non-existent method calls
grep -r "database.CreateJob\|database.CreateTask\|database.UpdateTaskStatus" --include="*_test.go"

# 3. Fix conflicting health DB expectations (panic vs 503)
grep -R "DatabaseHealthCheck\(.*\)\s*\n[\s\S]*assert\.Panics" internal/api --include "*_test.go" || true
# Replace panic expectations with 503 + error body assertions

# 4. Replace hard-coded version literals with the Version variable
grep -R "\"0\.4\.0\"" internal/api --include "*_test.go" || true
# Assert against api.Version or non-empty version instead of a fixed literal
```

### Phase 2: Consolidate Working Tests (Next Session)

#### Consolidate Worker Tests

Merge these files into single `worker_test.go`:

- `worker_advanced_test.go`
- `worker_lifecycle_test.go`
- Current `worker_test.go`

Keep the best test from each, removing duplicates.

#### Consolidate Manager Tests

Merge these files into single `manager_test.go`:

- `job_manager_unit_test.go`
- `manager_mock_test.go`
- Current `manager_test.go`

#### Strengthen DB Mocking and Deterministic DB Health Tests

- Ensure your API test mocks implement the full `DBClient` surface (`GetDB`, `GetOrCreateUser`, `GetJobStats`, `GetJobActivity`, `GetUserByWebhookToken`).
- Add `sqlmock` tests for `/health/db` covering both healthy and unhealthy ping paths (no real DB dependency for units).

### Phase 3: Establish Clear Patterns

#### Unit Test Pattern

```go
//go:build !integration

func TestSomething(t *testing.T) {
    // Use mocks for ALL external dependencies
    mockDB := &MockDB{}
    mockCrawler := &MockCrawler{}

    // Test business logic only
}
```

Concurrency pattern for unit tests that require goroutines:

```go
results := make(chan error, n)
for i := 0; i < n; i++ {
  go func() {
    // do work, push error/status to channel; DO NOT call t.* here
    results <- nil
  }()
}
for i := 0; i < n; i++ {
  err := <-results
  if err != nil { t.Errorf("unexpected error: %v", err) }
}
```

#### Integration Test Pattern

```go
//go:build integration

func TestSomethingIntegration(t *testing.T) {
    // Use real database
    database := setupTest(t)

    // Mock only external HTTP services
    mockCrawler := &MockCrawler{}

    // Test with real DB operations
}
```

DSN augmentation testing pattern (pure unit):

```go
// Extract DSN augmentation into a small helper and unit test it directly.
// e.g., dsn := augmentStatementTimeout("postgresql://...")
// assert.Contains(t, dsn, "statement_timeout=")
```

### Phase 4: Fix Build Tags

Current inconsistency:

- Some files use `//go:build integration`
- Some files use `// +build integration`
- Some files have no tags

Standard:

- Unit tests: No build tag (run by default)
- Integration tests: `//go:build integration` only

### Phase 5: CI and Script Hardening (Optional but recommended)

- In `run-tests.sh`, enable strict mode to fail fast:
  - Add `set -euo pipefail` near the top of the script.
- Consider exposing `Version` via ldflags in CI builds to decouple tests from literals.
- Keep timing‑based assertions permissive (no strict upper bounds) to reduce flakiness in CI.

## Test Coverage Reality Check

### Current True State

- **Claimed**: 33.8% overall
- **Reality**: Lower, as broken tests inflate the count
- **Working coverage**: ~25% (excluding broken tests)

### Realistic Targets

After cleanup:

- **Unit tests**: 60% coverage (focus on business logic)
- **Integration tests**: 20% coverage (critical paths only)
- **Combined effective**: 70% coverage

Quality over raw coverage:

- Prefer meaningful unit coverage on business logic (JobManager transitions, queue operations, error paths) over breadth with superficial assertions.
- Keep a small, reliable set of integration tests for critical paths (job creation, enqueue, cancellation) behind the `integration` tag.

## Immediate Action Items

### Do Now

1. ✅ Delete `internal/jobs/integration_test.go` - it's fundamentally broken
2. ✅ Run tests to ensure nothing breaks: `go test -v ./internal/jobs/...`
3. ✅ Count actual working test files and functions

### Next Session

1. Consolidate worker tests into single file
2. Consolidate manager tests into single file
3. Ensure all tests actually run and pass
4. Update TEST_PLAN.md with reality
5. Complete DB health unit tests with `sqlmock` and remove any remaining brittle goroutine assertions

### Future

1. Add missing unit tests for uncovered business logic
2. Add focused integration tests for critical user paths
3. Remove test duplication and establish clear ownership
4. Extract and unit test DSN augmentation; verify statement_timeout handling consistently across URL and key=value formats

## Key Principles Going Forward

### DO

- ✅ Test through the proper interfaces (JobManager, WorkerPool)
- ✅ Use mocks for unit tests
- ✅ Use real DB only for integration tests
- ✅ Keep test files parallel to source files
- ✅ Run tests regularly to catch breaks early

### DON'T

- ❌ Test DB methods that don't exist
- ❌ Create multiple test files for one source file
- ❌ Mix unit and integration tests without build tags
- ❌ Leave broken tests in the codebase
- ❌ Create tests without running them

## Success Metrics

### Short Term (This Week)

- [ ] Zero broken tests in codebase
- [ ] All duplicate tests removed
- [ ] Test files reduced from 10+ to 5-6
- [ ] All tests actually run and pass

### Medium Term (This Month)

- [ ] Clear unit vs integration separation
- [ ] 50%+ coverage on business logic
- [ ] CI runs tests on every commit
- [ ] No test has been broken for >24 hours

## Summary

The current test suite is a **rapid prototype that needs refactoring**, not a fundamental architectural problem. The application architecture is sound; the tests just need to align with how the application actually works rather than how we imagined it might work during the rapid test creation phase.

**Primary issue**: Tests were written against imagined interfaces rather than actual implementation.

**Solution**: Delete broken tests, consolidate working tests, establish clear patterns.

**Timeline**: 2-3 hours of cleanup work to achieve a maintainable state.
