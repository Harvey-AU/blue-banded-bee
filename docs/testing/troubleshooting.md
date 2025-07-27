# Troubleshooting

## Common Issues

### IPv6 Connection Errors in CI

**Error**: `dial tcp [IPv6]:5432: connect: network is unreachable`

**Solution**: Use Supabase pooler URL in `TEST_DATABASE_URL`:
```
postgresql://postgres.PROJECT_REF:PASSWORD@aws-0-REGION.pooler.supabase.com:5432/postgres
```

### Local Test Database Connection

**Error**: `Failed to connect to test database`

**Check**:
1. `.env.test` exists with valid `TEST_DATABASE_URL`
2. Test database is accessible
3. Password is correct

### SQL Scan Errors

**Error**: `sql: Scan error on column index 1, name "min": converting NULL to string is unsupported`

**Solution**: Use `sql.NullString` for nullable columns or COALESCE in queries.

### Test Timeouts

**Issue**: Tests hang or timeout

**Debug**:
```bash
# Run with timeout
go test -timeout 30s ./...

# Check for deadlocks
go test -race ./...
```

### Flaky Tests

**Issue**: Tests pass locally but fail in CI

**Common Causes**:
- Timing dependencies
- Test data conflicts
- Database state assumptions

**Fix**: Ensure proper test isolation and cleanup.

## Debug Commands

```bash
# Verbose output
go test -v ./...

# Specific test with logs
go test -v -run TestName ./package

# Disable test cache
go test -count=1 ./...

# Check database state
psql $TEST_DATABASE_URL -c "SELECT * FROM jobs;"
```