# CI/CD Pipeline

## GitHub Actions Workflow

The pipeline runs on every push to main branch.

### Workflow Steps

1. **Setup Go 1.25rc2**
   ```bash
   wget https://go.dev/dl/go1.25rc2.linux-amd64.tar.gz
   ```

2. **Code Quality Check** (golangci-lint v2.3.0)
   ```bash
   # Runs 140+ linters for:
   # - Security vulnerabilities (gosec)
   # - Code formatting (gofmt, goimports)  
   # - Performance issues (ineffassign)
   # - Complexity analysis (gocyclo)
   # - Best practices (errcheck)
   ```
   **Fast failure**: Stops pipeline immediately if quality issues found

3. **Run Tests**
   ```bash
   # Unit tests
   go test -v -coverprofile=coverage-unit.out -tags='!integration' ./...
   
   # Integration tests  
   go test -v -coverprofile=coverage-integration.out -tags=integration ./... -json > test-output.json
   ```

4. **Generate Reports**
   - JUnit XML for GitHub integration
   - Coverage report for Codecov
   - Merged coverage from unit + integration tests

5. **Deploy** (on success)
   - Deploys to Fly.io

### Key Configuration

```yaml
env:
  DATABASE_URL: ${{ secrets.TEST_DATABASE_URL }}
```

Uses `TEST_DATABASE_URL` secret which must be a Supabase pooler URL for IPv4 support.

### Coverage Integration

- **Codecov**: Tracks coverage over time
- **JUnit Reports**: Test results in GitHub UI
- **Badges**: Updated automatically in README

### Debugging CI Failures

Check the workflow output for:

**Linting Failures** (Step 2):
- Code formatting issues → Run `go fmt ./...` locally
- Security vulnerabilities → Review gosec warnings  
- Unused variables → Clean up with `go vet ./...`
- Complex functions → Refactor to reduce cyclomatic complexity

**Test Failures** (Step 3):
- Connection errors → Verify pooler URL format
- Test failures → Check test output JSON
- Coverage drops → Review Codecov report

**Common Linting Issues**:
```bash
# Fix most common issues locally:
go fmt ./...           # Formatting
go vet ./...           # Basic static analysis
go mod tidy            # Clean dependencies
```

## Local CI Testing

Simulate CI environment:

```bash
# Set DATABASE_URL like CI
export DATABASE_URL="postgresql://postgres.ref:pass@pooler.supabase.com:5432/postgres"

# Run tests with same flags
go test -v -coverprofile=coverage.out ./... -json > test-output.json
```