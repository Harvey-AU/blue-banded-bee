# Blue Banded Bee Test Plan

## Overview
Comprehensive testing plan for unit and integration tests using mocks and testify framework.

## Testing Principles
- Unit tests should be isolated using mocks
- Integration tests should use real database connections (tagged)
- All tests use testify (assert/require) for consistency
- Table-driven tests for multiple scenarios
- Benchmark tests for performance-critical paths

## Current Test Coverage Status

### ✅ Well Tested
- `internal/api/admin_test.go` - System admin role validation
- `internal/api/jobs_test.go` - Job endpoints and request/response
- `internal/crawler/robots_test.go` - Robots.txt parsing
- `internal/jobs/worker_test.go` - Error retry logic

### ⚠️ Partially Tested
- `internal/api/handlers_simple_test.go` - Basic handlers, missing webhook logic
- `internal/crawler/crawler_test.go` - Basic crawling, missing link extraction
- `internal/jobs/manager_test.go` - Basic manager, missing concurrency tests

### ❌ No Tests
- `internal/auth/` - JWT validation and middleware
- `internal/db/` - Database operations (except health check)
- `internal/cache/` - Caching layer

---

## Unit Tests Required

### 🔴 Critical Priority

#### Authentication (`internal/auth/`)
- [ ] Test JWT token validation with valid token
- [ ] Test JWT token expiry handling
- [ ] Test malformed JWT token rejection
- [ ] Test missing auth header handling
- [ ] Test user claims extraction from context
- [ ] Test organisation role permission checks
- [ ] Test system admin role validation
- [ ] Mock Supabase client for auth verification

#### Database Core (`internal/db/`)
- [ ] Test connection pool initialisation
- [ ] Test connection retry with backoff
- [ ] Test transaction rollback on error
- [ ] Test prepared statement caching
- [ ] Mock pgx connection for unit tests
- [ ] Test connection health check timeout

#### Job Queue (`internal/db/queue.go`)
- [ ] Test CreateJob with valid domain
- [ ] Test GetJob by ID retrieval
- [ ] Test UpdateJobStatus state transitions
- [ ] Test GetPendingJobs priority ordering
- [ ] Test CreateTask with duplicate URL
- [ ] Test GetNextTask with locking
- [ ] Test UpdateTaskStatus with retry count
- [ ] Test GetTasksByJobID pagination
- [ ] Mock database queries for isolation

### 🟡 Medium Priority

#### Webhook Handlers (`internal/api/handlers.go`)
- [ ] Test Webflow webhook signature validation
- [ ] Test webhook payload parsing errors
- [ ] Test duplicate webhook request handling
- [ ] Test webhook token authentication
- [ ] Test rate limiting for webhooks
- [ ] Mock job manager for webhook tests

#### Worker Pool (`internal/jobs/worker.go`)
- [ ] Test worker pool size adjustment
- [ ] Test graceful shutdown with timeout
- [ ] Test panic recovery in workers
- [ ] Test task processing concurrency limits
- [ ] Test worker health monitoring
- [ ] Mock crawler for worker tests

#### Crawler Core (`internal/crawler/crawler.go`)
- [ ] Test link extraction from HTML
- [ ] Test URL normalisation and deduplication
- [ ] Test redirect following with limits
- [ ] Test timeout handling for slow sites
- [ ] Test cache header parsing
- [ ] Mock HTTP client for crawler tests

#### Sitemap Parser (`internal/crawler/sitemap.go`)
- [ ] Test XML sitemap parsing
- [ ] Test sitemap index handling
- [ ] Test gzipped sitemap decompression
- [ ] Test malformed XML error handling
- [ ] Test URL priority extraction
- [ ] Mock HTTP responses for sitemap tests

### 🟢 Lower Priority

#### Response Helpers (`internal/api/response.go`)
- [ ] Test JSON response formatting
- [ ] Test error response structure
- [ ] Test pagination header generation
- [ ] Test CORS header handling

#### URL Utilities (`internal/util/url.go`)
- [ ] Test URL validation for domains
- [ ] Test path normalisation edge cases
- [ ] Test query parameter handling
- [ ] Test protocol enforcement rules

#### Cache Manager (`internal/cache/cache.go`)
- [ ] Test cache key generation
- [ ] Test TTL expiry behaviour
- [ ] Test cache eviction policies
- [ ] Test concurrent access safety
- [ ] Mock Redis/memory store

---

## Integration Tests Required

### 🔴 Critical Priority

#### End-to-End Job Processing
- [ ] Test complete job lifecycle from creation
- [ ] Test webhook triggering job creation
- [ ] Test job processing with real crawler
- [ ] Test task retry on failure
- [ ] Test job cancellation mid-process
- [ ] Requires: Real database, mock HTTP server

#### Database Migrations
- [ ] Test migration application ordering
- [ ] Test rollback functionality
- [ ] Test idempotent migration runs
- [ ] Test schema version tracking
- [ ] Requires: Test database instance

### 🟡 Medium Priority

#### Authentication Flow
- [ ] Test Supabase JWT validation end-to-end
- [ ] Test role-based access control
- [ ] Test token refresh flow
- [ ] Test logout and token revocation
- [ ] Requires: Supabase test project

#### Rate Limiting
- [ ] Test per-IP rate limiting
- [ ] Test per-user rate limiting
- [ ] Test rate limit headers
- [ ] Test burst handling
- [ ] Requires: Real Redis/memory store

#### Webhook Processing
- [ ] Test Webflow webhook integration
- [ ] Test webhook retry on failure
- [ ] Test webhook deduplication
- [ ] Test webhook signature verification
- [ ] Requires: Mock Webflow server

### 🟢 Lower Priority

#### Monitoring & Metrics
- [ ] Test Sentry error reporting
- [ ] Test performance metric collection
- [ ] Test health check endpoints
- [ ] Test graceful degradation
- [ ] Requires: Mock monitoring services

---

## Test Infrastructure Required

### Mocks to Create
1. **Database Mock** (`internal/mocks/db.go`)
   - Mock all DB interface methods
   - Support transaction testing
   - Error injection capability

2. **HTTP Client Mock** (`internal/mocks/http_client.go`)
   - Mock HTTP responses
   - Simulate timeouts
   - Test redirect chains

3. **Job Manager Mock** (`internal/mocks/job_manager.go`)
   - Mock job lifecycle
   - Test concurrency
   - Simulate failures

4. **Auth Client Mock** (`internal/mocks/auth_client.go`)
   - Mock Supabase client
   - Test token validation
   - Role checking

### Test Helpers Needed
1. **Test Database Setup** (`internal/testutil/db.go`)
   - Create test database
   - Run migrations
   - Seed test data
   - Cleanup after tests

2. **Test Server** (`internal/testutil/server.go`)
   - Mock HTTP endpoints
   - Simulate slow responses
   - Return various status codes

3. **Test Fixtures** (`internal/testutil/fixtures.go`)
   - Sample job data
   - Sample webhook payloads
   - Sample HTML responses
   - Sample sitemaps

---

## Testing Commands

### Run Unit Tests Only
```bash
go test -v -race ./... -short
```

### Run Integration Tests Only
```bash
go test -v -race -tags=integration ./...
```

### Run All Tests with Coverage
```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Benchmarks
```bash
go test -bench=. -benchmem ./...
```

---

## Success Metrics
- [ ] Unit test coverage > 80%
- [ ] Integration test coverage > 60%
- [ ] All critical paths tested
- [ ] Mock interfaces for all external dependencies
- [ ] No flaky tests
- [ ] Tests run in < 30 seconds (unit)
- [ ] Tests run in < 2 minutes (integration)

---

## Next Steps
1. Create mock interfaces for database and HTTP
2. Implement critical unit tests for auth and database
3. Set up test database for integration tests
4. Add integration test build tags
5. Update CI pipeline to run both test suites