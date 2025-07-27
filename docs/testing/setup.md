# Test Environment Setup

## Local Development

### 1. Create Test Database

Create a Supabase test branch for isolated testing.

### 2. Configure Environment

Create `.env.test` in project root:

```bash
TEST_DATABASE_URL=postgresql://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
```

### 3. Verify Setup

```bash
# Test connection
go test -v ./internal/jobs -run TestDatabaseConnection
```

## CI Environment

### GitHub Actions Configuration

Set `TEST_DATABASE_URL` secret in GitHub repository settings.

**Important**: Use Supabase pooler URL for IPv4 compatibility:

```
postgresql://postgres.[PROJECT_REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:5432/postgres
```

GitHub Actions doesn't support IPv6, so the pooler URL (session mode, port 5432) is required.

### Required Secrets

- `TEST_DATABASE_URL` - Pooler URL for test database
- `CODECOV_TOKEN` - For coverage reporting
- `FLY_API_TOKEN` - For deployment
- `SENTRY_DSN` - Error tracking (optional)