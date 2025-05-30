# Development Guide

## Prerequisites

- **Go 1.24.2+** - Latest Go version with module support
- **PostgreSQL** - Local instance or remote database access
- **Air** (optional) - Hot reloading for development (`go install github.com/cosmtrek/air@latest`)
- **Git** - Version control

## Quick Setup

### 1. Clone and Setup

```bash
# Fork and clone the repository
git clone https://github.com/[your-username]/blue-banded-bee.git
cd blue-banded-bee

# Copy environment template
cp .env.example .env
```

### 2. Configure Environment

Edit `.env` with your settings:

```bash
# Database Configuration
DATABASE_URL="postgres://user:password@localhost:5432/bluebandedbeee"
# OR individual settings
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=bluebandedbeee
DB_SSLMODE=prefer

# Application Settings
PORT=8080
APP_ENV=development
LOG_LEVEL=debug

# Sentry (optional for development)
SENTRY_DSN=your_sentry_dsn

# Supabase Auth (for API testing)
SUPABASE_JWT_SECRET=your_jwt_secret
SUPABASE_URL=your_supabase_url
SUPABASE_ANON_KEY=your_anon_key
```

### 3. Database Setup

```bash
# Create PostgreSQL database
createdb bluebandedbeee

# The application will automatically create tables on first run
go run ./cmd/app/main.go
```

## Development Server

### With Hot Reloading (Recommended)

```bash
# Install Air if not already installed
go install github.com/cosmtrek/air@latest

# Start development server with hot reloading
air
```

### Without Hot Reloading

```bash
# Build and run
go build ./cmd/app && ./app

# Or run directly
go run ./cmd/app/main.go
```

### Server will start on `http://localhost:8080`

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run tests with verbose output
go test ./... -v

# Run specific package tests
go test ./internal/jobs -v
```

### Integration Tests

Integration tests require a PostgreSQL database connection:

```bash
# Set environment flag and run
RUN_INTEGRATION_TESTS=true go test ./...
```

### Manual API Testing

Use the provided HTTP test file:

```bash
# Install httpie or use curl
pip install httpie

# Test health endpoint
http GET localhost:8080/health

# Test job creation (requires auth token)
http POST localhost:8080/v1/jobs \
  Authorization:"Bearer your-jwt-token" \
  domain=example.com \
  use_sitemap:=true
```

### Job Queue Testing

Test the job queue system:

```bash
# Run job queue test utility
go run ./cmd/test_jobs/main.go
```

## Code Organization

### Package Structure

```
cmd/
├── app/           # Main application entry point
└── test_jobs/     # Job queue testing utility

internal/
├── api/           # HTTP handlers and middleware
├── auth/          # Authentication logic
├── crawler/       # Web crawling functionality
├── db/            # Database operations
├── jobs/          # Job queue and worker management
└── util/          # Shared utilities
```

### Development Patterns

#### Error Handling
- Use wrapped errors: `fmt.Errorf("context: %w", err)`
- Log errors with context: `log.Error().Err(err).Str("job_id", id).Msg("Failed to process")`
- Capture critical errors in Sentry: `sentry.CaptureException(err)`

#### Database Operations
- Use PostgreSQL-style parameters: `$1, $2, $3`
- Wrap operations in transactions via `dbQueue.Execute()`
- Handle connection pooling automatically

#### Testing
- Place tests alongside implementation: `file_test.go`
- Use table-driven tests for multiple scenarios
- Mock external dependencies (HTTP, database)

## Debugging

### Log Levels

Set `LOG_LEVEL` in `.env`:
- `debug` - Verbose logging for development
- `info` - Standard operational logging
- `warn` - Warning conditions
- `error` - Error conditions only

### Sentry Integration

In development, Sentry captures all traces (100% sampling):

```bash
# Enable Sentry debugging
DEBUG=true
SENTRY_DSN=your_dsn
```

### Database Debugging

Enable SQL query logging:

```sql
-- In PostgreSQL console
ALTER SYSTEM SET log_statement = 'all';
SELECT pg_reload_conf();
```

### Common Debug Commands

```bash
# Check database connection
go run -ldflags="-X main.debugDB=true" ./cmd/app/main.go

# Run with race detection
go run -race ./cmd/app/main.go

# Profile memory usage
go run ./cmd/app/main.go -memprofile=mem.prof

# Check for goroutine leaks
GODEBUG=gctrace=1 go run ./cmd/app/main.go
```

## Contributing

### Code Quality

Before submitting:

1. **Format code**: `go fmt ./...`
2. **Run linter**: `golangci-lint run` (if installed)
3. **Run tests**: `go test ./...`
4. **Check coverage**: `go test ./... -coverprofile=coverage.out`
5. **Update docs**: Update relevant documentation

### Git Workflow

```bash
# Create feature branch
git checkout -b feature/your-feature

# Make changes and commit
git add .
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/your-feature
```

### Commit Messages

Use conventional commits:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test additions/changes
- `chore:` - Maintenance tasks

### Pull Request Process

1. **Update documentation** for any API or architectural changes
2. **Add/update tests** for new functionality
3. **Ensure all tests pass** and coverage remains high
4. **Update CHANGELOG.md** if the change affects users
5. **Reference relevant issues** in PR description

## Deployment

### Local Build

```bash
# Build for current platform
go build ./cmd/app

# Build for Linux (Fly.io deployment)
GOOS=linux GOARCH=amd64 go build ./cmd/app
```

### Docker Development

```bash
# Build container
docker build -t blue-banded-bee .

# Run with database link
docker run --env-file .env -p 8080:8080 blue-banded-bee
```

### Environment-Specific Configs

**Development**:
- Hot reloading enabled
- Verbose logging
- 100% Sentry trace sampling
- Debug mode enabled

**Production**:
- Optimised builds
- Error-level logging
- 10% Sentry trace sampling
- Security hardening

## Troubleshooting

### Common Issues

**Database Connection Errors**:
```bash
# Check PostgreSQL is running
pg_isready -h localhost -p 5432

# Verify credentials
psql -h localhost -U your_user -d bluebandedbeee
```

**Port Already in Use**:
```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>
```

**Module Dependencies**:
```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
```

**Hot Reloading Not Working**:
```bash
# Verify Air configuration
cat .air.toml

# Reinstall Air
go install github.com/cosmtrek/air@latest
```

### Performance Issues

**High Memory Usage**:
- Check for goroutine leaks with `go tool pprof`
- Monitor database connection pool usage
- Verify proper cleanup of HTTP clients

**Slow Database Queries**:
- Enable query logging in PostgreSQL
- Use `EXPLAIN ANALYZE` for query performance
- Check connection pool settings

### Getting Help

1. **Check existing documentation** in this guide and [ARCHITECTURE.md](ARCHITECTURE.md)
2. **Search closed issues** on GitHub for similar problems
3. **Enable debug logging** to get more context
4. **Create minimal reproduction** case for bugs
5. **Open GitHub issue** with detailed information

## Next Steps

After setting up development:

1. **Read [ARCHITECTURE.md](ARCHITECTURE.md)** to understand system design
2. **Review [API.md](API.md)** for endpoint documentation
3. **Check [DATABASE.md](DATABASE.md)** for schema details
4. **Explore the codebase** starting with `cmd/app/main.go`
5. **Run the test suite** to verify everything works