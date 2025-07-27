# Testing Documentation

## Overview

Blue Banded Bee uses a comprehensive testing approach combining integration tests with a real Supabase test database and automated CI/CD pipeline through GitHub Actions.

## Test Infrastructure

### Database Setup

- **Test Database**: Dedicated Supabase branch for testing
- **Local Config**: `.env.test` file with `TEST_DATABASE_URL`
- **CI Config**: GitHub Actions secret with Supabase pooler URL

### Key Components

1. **Test Runner**: `run-tests.sh` - Loads test environment and runs all tests
2. **Test Utilities**: `internal/testutil` - Handles test environment setup
3. **CI Pipeline**: `.github/workflows/fly-deploy.yml` - Automated testing and deployment
4. **Coverage Tracking**: Codecov integration for test coverage reporting

## Running Tests

### Local Development

```bash
# Run all tests with test database
./run-tests.sh

# Run specific package tests
go test -v ./internal/jobs

# Run with coverage report
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -v ./internal/jobs -run TestDatabaseConnection
```

### Continuous Integration

The CI pipeline automatically runs on every push:

1. **Test Execution**: All tests run with coverage reporting
2. **JUnit Reporting**: Test results converted to JUnit XML
3. **Coverage Upload**: Results sent to Codecov
4. **Deployment**: Auto-deploy to Fly.io on success (main branch)

## Test Categories

### Integration Tests

Real database tests using Supabase test branch:

- **Job Management**: Creating, updating, cancelling jobs
- **Task Processing**: Queue operations, priority handling
- **Database Operations**: Transactions, constraints, triggers
- **Worker Pool**: Concurrent task processing

Example:
```go
func TestCreateJob(t *testing.T) {
    database := setupTest(t)  // Connects to test database
    defer database.Close()
    
    // Test with real database operations
}
```

### Unit Tests

Isolated business logic tests:

- **URL Validation**: Path extraction, normalisation
- **Error Classification**: Retry logic, blocking errors
- **Configuration**: Environment variable handling
- **Utilities**: Helper functions

## CI/CD Configuration

### GitHub Actions Workflow

The workflow file (`.github/workflows/fly-deploy.yml`) includes:

```yaml
- name: Run tests and generate coverage report
  env:
    DATABASE_URL: ${{ secrets.TEST_DATABASE_URL }}
  run: |
    go test -v -coverprofile=coverage.out ./... -json > test-output.json
    # Convert to JUnit for GitHub integration
    go-junit-report -parser gojson < test-output.json > junit.xml
```

### Important: IPv4 Connectivity

GitHub Actions doesn't support IPv6. Use Supabase pooler URL:

```
postgresql://postgres.PROJECT_REF:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:5432/postgres
```

Set this as `TEST_DATABASE_URL` in GitHub Secrets.

## Test Database Management

### Schema Synchronisation

The test database schema is automatically created by the `setupSchema()` function in `internal/db/db.go`.

### Test Isolation

Each test:
1. Loads test environment via `testutil.LoadTestEnv()`
2. Connects to test database
3. Runs operations in isolated context
4. Cleans up test data

### Connection Pooling

Test configuration:
- Max Idle Connections: 30
- Max Open Connections: 75
- Connection Lifetime: 20 minutes

## Coverage Requirements

Current coverage tracked via Codecov:
- Minimum threshold: Set in `codecov.yml`
- Coverage badges: Displayed in README.md
- Per-package breakdown: Available in Codecov dashboard

## Troubleshooting

### Common Issues

1. **IPv6 Connection Errors in CI**
   - Ensure `TEST_DATABASE_URL` uses pooler URL format
   - Verify secret is set correctly in GitHub

2. **Local Test Failures**
   - Check `.env.test` has valid `TEST_DATABASE_URL`
   - Ensure test database is accessible
   - Verify schema is up to date

3. **Flaky Tests**
   - Check for proper test isolation
   - Ensure cleanup between tests
   - Consider timing issues with async operations

### Debug Commands

```bash
# Test database connection
go test -v ./internal/jobs -run TestDatabaseConnection

# Run with race detector
go test -race ./...

# Verbose test output
go test -v -count=1 ./...  # -count=1 disables test caching
```

## Best Practices

1. **Use Real Database**: Integration tests provide confidence
2. **Test Isolation**: Each test should be independent
3. **Clear Names**: Test names should describe what they verify
4. **Fast Feedback**: Keep tests fast for developer productivity
5. **CI Compatibility**: Always verify tests work in CI environment

## Future Improvements

- [ ] Add more unit tests for complex business logic
- [ ] Implement test data factories
- [ ] Add performance benchmarks
- [ ] Set up parallel test execution
- [ ] Add mutation testing