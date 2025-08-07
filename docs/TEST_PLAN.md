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

**Overall Coverage: 23.0%** (as of latest test run)

### Coverage by Package
- `internal/cache` - **100.0%** ✅
- `internal/util` - **81.1%** ✅
- `internal/crawler` - **65.5%** ⚠️
- `internal/api` - **41.2%** ⚠️
- `internal/auth` - **33.8%** ⚠️
- `internal/db` - **10.5%** ❌
- `internal/jobs` - **1.1%** ❌
- `internal/mocks` - **0.0%** (expected for mocks)
- `internal/testutil` - **0.0%** (expected for test utilities)

### Coverage by Functional Area

#### 🔐 Authentication & Authorization
- **Core Auth**: ⚠️ Partial (config done, JWT validation pending)
- **Auth API**: ✅ Good (endpoints tested)

#### 🗄️ Database Layer  
- **Connection Management**: ⚠️ Partial (config done, operations pending)
- **Job Queue**: ❌ Not tested
- **Dashboard Queries**: ❌ Not tested

#### 🕷️ Web Crawling
- **Core Crawler**: ✅ Good (sitemap/robots done, HTML parsing pending)
- **URL Processing**: ✅ Excellent (fully tested)

#### 👷 Job Processing
- **Job Types**: ✅ Good (types tested)
- **Worker Pool**: ❌ Not tested
- **Job Manager**: ❌ Not tested

#### 🌐 API & Web Layer
- **Core Handlers**: ✅ Good (health/routes tested)
- **Middleware**: ✅ Excellent (fully tested)
- **Response Helpers**: ✅ Excellent (fully tested)
- **Webhooks**: ❌ Not tested
- **Dashboard API**: ⚠️ Partial (date filtering done)

#### 💾 Caching
- **In-Memory Cache**: ✅ Excellent (100% coverage)

### ✅ Completed Tests
- [x] `internal/cache/cache_test.go` - Complete cache implementation tests with concurrency
- [x] `internal/auth/auth_test.go` - Config validation and environment handling
- [x] `internal/db/db_test.go` - Connection configuration and validation tests
- [x] `internal/db/health_integration_test.go` - Database health check integration
- [x] `internal/util/url_test.go` - URL utility functions comprehensive tests
- [x] `internal/api/auth_test.go` - Auth endpoints testing
- [x] `internal/api/handlers_test.go` - Handler functions comprehensive tests
- [x] `internal/api/middleware_test.go` - Complete middleware testing
- [x] `internal/api/response_test.go` - Response helpers comprehensive tests
- [x] `internal/jobs/types_test.go` - Job and task types with JSON serialization
- [x] `internal/crawler/sitemap_test.go` - Sitemap discovery and filtering

### ⚠️ Partially Tested
- [ ] `internal/api/handlers_simple_test.go` - Basic handlers, missing webhook logic
- [ ] `internal/crawler/crawler_test.go` - Basic crawling, missing link extraction
- [ ] `internal/jobs/manager_test.go` - Basic manager, missing concurrency tests

### ❌ Needs More Testing
- [ ] `internal/db/` - Database operations (needs more coverage beyond config)
- [ ] `internal/jobs/` - Worker pool and job processing (only types tested)

---

## Unit Tests by Functional Area

### 🔐 Authentication & Authorization

#### Core Auth (`internal/auth/`)
- [x] JWT token configuration loading
- [x] Environment variable validation
- [x] Config validation method
- [ ] JWT token validation with valid token
- [ ] JWT token expiry handling
- [ ] Malformed JWT token rejection
- [ ] Missing auth header handling
- [ ] User claims extraction from context
- [ ] Organisation role permission checks
- [ ] Mock Supabase client for auth verification

#### Auth API Endpoints (`internal/api/auth.go`)
- [x] Registration endpoint (POST /v1/auth/register)
- [x] Session endpoint (POST /v1/auth/session)
- [x] Profile endpoint (GET /v1/auth/profile)
- [ ] Token refresh endpoint
- [ ] Logout endpoint

### 🗄️ Database Layer

#### Connection Management (`internal/db/`)
- [x] Connection pool initialisation
- [x] Connection configuration and validation
- [x] DatabaseURL vs individual field precedence
- [x] Environment variable loading
- [x] Health check timeout (integration test)
- [ ] Connection retry with backoff
- [ ] Transaction rollback on error
- [ ] Prepared statement caching
- [ ] Mock pgx connection for unit tests

#### Job Queue Operations (`internal/db/queue.go`)
- [ ] CreateJob with valid domain
- [ ] GetJob by ID retrieval
- [ ] UpdateJobStatus state transitions
- [ ] GetPendingJobs priority ordering
- [ ] CreateTask with duplicate URL
- [ ] GetNextTask with locking
- [ ] UpdateTaskStatus with retry count
- [ ] GetTasksByJobID pagination
- [ ] Mock database queries for isolation

#### Dashboard Queries (`internal/db/dashboard.go`)
- [ ] Stats aggregation queries
- [ ] Activity timeline queries
- [ ] Date range filtering in queries
- [ ] Pagination in dashboard queries

### 🕷️ Web Crawling

#### Core Crawler (`internal/crawler/`)
- [x] Sitemap discovery and parsing
- [x] Robots.txt parsing
- [x] URL filtering (include/exclude paths)
- [ ] Link extraction from HTML
- [ ] URL normalisation and deduplication
- [ ] Redirect following with limits
- [ ] Timeout handling for slow sites
- [ ] Cache header parsing
- [ ] Mock HTTP client for crawler tests

#### URL Processing (`internal/util/`)
- [x] URL normalisation functions
- [x] Domain extraction
- [x] Path extraction
- [x] URL construction
- [x] Protocol handling

### 👷 Job Processing

#### Job Types & Models (`internal/jobs/types.go`)
- [x] Job and task type definitions
- [x] Status constants
- [x] JSON serialization of jobs/tasks

#### Worker Pool (`internal/jobs/worker.go`)
- [ ] Worker pool size adjustment
- [ ] Graceful shutdown with timeout
- [ ] Panic recovery in workers
- [ ] Task processing concurrency limits
- [ ] Worker health monitoring
- [ ] Mock crawler for worker tests

#### Job Manager (`internal/jobs/manager.go`)
- [ ] Job scheduling
- [ ] Job cancellation
- [ ] Worker allocation
- [ ] Queue management
- [ ] Concurrency control

### 🌐 API & Web Layer

#### Core Handlers (`internal/api/handlers.go`)
- [x] Health check endpoint
- [x] Database health check endpoint
- [x] Handler initialization
- [x] Route setup
- [x] Static file serving

#### Middleware (`internal/api/middleware.go`)
- [x] Request ID middleware
- [x] Logging middleware
- [x] CORS middleware
- [x] Security headers middleware
- [x] Cross-origin protection middleware

#### Response Helpers (`internal/api/response.go`)
- [x] JSON response formatting
- [x] Success response structure
- [x] Error response structure
- [x] Health response structure
- [x] No content responses

#### Webhook Handling (`internal/api/webhooks.go`)
- [ ] Webflow webhook signature validation
- [ ] Webhook payload parsing errors
- [ ] Duplicate webhook request handling
- [ ] Webhook token authentication
- [ ] Rate limiting for webhooks

#### Dashboard API (`internal/api/dashboard.go`)
- [ ] Stats endpoint aggregation
- [ ] Activity timeline generation
- [x] Date range filtering (calculateDateRange)
- [ ] Pagination handling

### 💾 Caching

#### In-Memory Cache (`internal/cache/`)
- [x] Cache implementation
- [x] Get/Set/Delete operations
- [x] Concurrent access safety
- [x] Multiple key operations
- [x] Benchmark cache operations

---

## Integration Tests by Functional Area

### 🔄 End-to-End Workflows

#### Complete Job Processing Flow
- [ ] Job creation via API
- [ ] Webhook triggering job creation
- [ ] Job processing with real crawler
- [ ] Task retry on failure
- [ ] Job cancellation mid-process
- [ ] Job completion and stats update
- **Requirements**: Real database, mock HTTP server

#### User Journey
- [ ] User registration flow
- [ ] Authentication and authorization
- [ ] Create and monitor job
- [ ] View dashboard stats
- [ ] Webhook integration
- **Requirements**: Full stack setup

### 🗄️ Database Integration

#### Migration Testing
- [ ] Migration application ordering
- [ ] Rollback functionality
- [ ] Idempotent migration runs
- [ ] Schema version tracking
- **Requirements**: Test database instance

#### Database Health
- [x] Health check with timeout
- [ ] Connection pool under load
- [ ] Transaction isolation levels
- [ ] Deadlock handling
- **Requirements**: PostgreSQL test instance

### 🔐 Authentication Integration

#### Supabase Integration
- [ ] JWT validation end-to-end
- [ ] Role-based access control
- [ ] Token refresh flow
- [ ] Logout and token revocation
- **Requirements**: Supabase test project

#### API Security
- [ ] Rate limiting per IP
- [ ] Rate limiting per user
- [ ] Rate limit headers
- [ ] Burst handling
- **Requirements**: Redis/memory store

### 🔗 External Services

#### Webhook Integration
- [ ] Webflow webhook processing
- [ ] Webhook retry on failure
- [ ] Webhook deduplication
- [ ] Signature verification
- **Requirements**: Mock Webflow server

#### Monitoring Services
- [ ] Sentry error reporting
- [ ] Performance metrics
- [ ] Health check endpoints
- [ ] Graceful degradation
- **Requirements**: Mock monitoring services

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
- [ ] Unit test coverage > 80% (Currently: 23.0%)
- [ ] Integration test coverage > 60%
- [ ] All critical paths tested
- [ ] Mock interfaces for all external dependencies
- [ ] No flaky tests
- [x] Tests run in < 30 seconds (unit)
- [ ] Tests run in < 2 minutes (integration)

---

## Progress Towards Goals

### Current Status
- **Overall Coverage**: 23.0% ↑ (from initial 13.3%)
- **Target**: 40-50% short-term, 80% long-term
- **Packages with Excellent Coverage (>80%)**: 
  - [x] cache (100%)
  - [x] util (81%)
- **Packages with Good Coverage (>40%)**:
  - [x] crawler (65%)
  - [x] api (41%)
- **Packages with Basic Coverage (>20%)**:
  - [x] auth (34%)
- **Packages Needing Work (<20%)**:
  - [ ] db (10%)
  - [ ] jobs (1%)

### Completed in This Session
- [x] Added comprehensive cache tests with concurrency
- [x] Created auth config validation tests
- [x] Added db connection configuration tests
- [x] Created database health check integration tests
- [x] Added comprehensive URL utility tests
- [x] Created middleware tests (request ID, logging, CORS, security)
- [x] Added response helper tests (JSON, Success, Error, Health)
- [x] Created handler tests for API endpoints
- [x] Added job/task type tests with JSON serialization
- [x] Created sitemap discovery and filtering tests

### Next Priority Tasks
1. **High Impact**: Increase `internal/db` coverage from 10% to 30%+
   - [ ] Add queue.go tests (CreateJob, GetJob, UpdateJobStatus)
   - [ ] Add transaction and rollback tests
   - [ ] Add connection pool tests
2. **High Impact**: Increase `internal/jobs` coverage from 1% to 20%+
   - [ ] Add worker pool tests
   - [ ] Add job manager tests
   - [ ] Add task processing tests
3. **Medium Impact**: Complete webhook handling tests
   - [ ] Add Webflow webhook signature validation
   - [ ] Add webhook deduplication tests
4. **Medium Impact**: Add integration tests
   - [ ] End-to-end job processing
   - [ ] Database migration tests