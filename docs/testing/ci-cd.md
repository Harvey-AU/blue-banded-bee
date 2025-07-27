# CI/CD Pipeline

## GitHub Actions Workflow

The pipeline runs on every push to main branch.

### Workflow Steps

1. **Setup Go 1.25rc2**
   ```bash
   wget https://go.dev/dl/go1.25rc2.linux-amd64.tar.gz
   ```

2. **Run Tests**
   ```bash
   go test -v -coverprofile=coverage.out ./... -json > test-output.json
   ```

3. **Generate Reports**
   - JUnit XML for GitHub integration
   - Coverage report for Codecov

4. **Deploy** (on success)
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
- Connection errors → Verify pooler URL format
- Test failures → Check test output JSON
- Coverage drops → Review Codecov report

## Local CI Testing

Simulate CI environment:

```bash
# Set DATABASE_URL like CI
export DATABASE_URL="postgresql://postgres.ref:pass@pooler.supabase.com:5432/postgres"

# Run tests with same flags
go test -v -coverprofile=coverage.out ./... -json > test-output.json
```