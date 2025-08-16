# Blue Banded Bee Test Plan

## Overview

Comprehensive testing plan for unit and integration tests using mocks and testify framework.

## CRITICAL TEST FIXES REQUIRED (Expert Review - Aug 2025)

### P0 - CRITICAL Production Risks (Fix Immediately)

- [x] **Fix health endpoint panic** - `/health/db` panics when DB is nil (handlers.go)
  - Guard `h.DB == nil` before `h.DB.GetDB().Ping()`
  - Return 503 with unhealthy JSON, not panic
  - Update test to expect 503, not panic
- [x] **Fix broken `contains()` function** in worker_advanced_test.go:347-349
  - Current: returns true for any non-empty strings
  - Fix: use `strings.Contains(s, substr)`
- [x] **Make DB mockable** - Cannot unit test DB paths
  - Define `DBClient interface { GetDB() *sql.DB }` (expanded to match handler needs)
  - Added constructor/wrappers to use a real \*sql.DB in tests where needed
  - Added sqlmock tests for healthy/unhealthy DB ping in `internal/api/handlers_db_test.go`

### P1 - HIGH Priority Brittleness (Fix Today)

- [x] **Centralise version string** - handlers_test.go hard-codes "0.4.0"
  - Create package-level `var Version = "0.4.0"`
  - Use in HealthCheck handler
  - Update test to assert non-empty or use Version var
- [x] **Replace placeholder tests** in db_operations_test.go
  - Tests assert on local variables, not production code
  - Add real unit tests for DSN building and pool config
  - Or mark with `t.Skip("TODO: real tests")`
- [x] **Add t.Cleanup() for resource management**
  - Use immediately after creating resources
  - Ensure cleanup even on test failure
  - Critical for integration tests
  - Implemented in manager_test.go with proper LIFO ordering
- [x] **Fix timing assertions** - Remove fragile upper bounds
  - middleware_test.go: remove `elapsed < delay+10ms`
  - Keep only `elapsed >= delay`
  - Add exponential backoff for polling tests

### P2 - MEDIUM Testing Quality (Fix This Week)

- [ ] **Add t.Parallel() to independent tests**
  - Identify all unit tests without shared state
  - Add `t.Parallel()` at start
  - Don't parallelise integration tests
- [ ] **Convert integration tests to unit tests**
  - Identify tests that could use mocks
  - Reduce database dependency
  - Improve test speed (target < 10s for units)
- [ ] **Implement proper mock strategy**
  - Create interfaces for all external deps
  - Use dependency injection
  - Enable true unit testing
- [ ] **Expand API handler tests**
  - Add auth middleware path tests
  - Test error shapes and contracts
  - Add golden JSON tests

### P3 - Improvements

- [x] **Add set -euo pipefail to run-tests.sh**
- [ ] **Add exponential backoff to polling tests**
- [ ] **Add edge cases to crawler tests**
  - Robots.txt disallow patterns
  - TLS errors and retries
- [ ] **Add golden JSON tests for API responses**

## Immediate Actions and Standards (Quick Guide)

This short section captures the immediate priorities and execution standards so contributors and agents can act quickly.

- Immediate actions (next PRs)
  - internal/db queue unit tests (fast, isolated):
    - GetNextTask locking/exclusivity (concurrent callers, only one claims, second gets sql.ErrNoRows)
    - UpdateTaskStatus transitions and retry count handling; reset started_at; set completed_at
    - CreateTask duplicate URL idempotence and unique-constraint handling
    - GetTasksByJobID pagination (boundaries, ordering)
    - Execute/transaction rollback paths (simulate mid-transaction error)
  - jobs/worker advanced behaviour tests (with mocks):
    - recoverStaleTasks (max retries → Failed vs Pending reset) and recoverRunningJobs (resets tasks; re-adds jobs; preserves find_links)
    - checkForPendingTasks adds missing jobs, updates job status; removes inactive jobs
    - flushBatches single-transaction updates; no-op on empty batch
    - evaluateJobPerformance scaling/clamping; workers scale up but clamp at global max
    - Blocking vs retryable errors (403/429 limited retries; timeouts/5xx retried; permanent failures)
  - crawler/link tests via httptest.Server fixtures:
    - Redirects, timeouts, cache headers, varied content types
    - Sectioned link extraction (header/body/footer) and homepage vs nested priorities
    - robots.txt filtering and same/subdomain checks (`www.`, trailing slash, fragments)
  - API negative paths and response contracts:
    - Auth/permission matrix with mock claims; correct statuses/messages
    - Pagination and invalid query params; consistent error payloads
    - Golden JSON for complex responses to prevent drift
  - Infra guardrails (tests/CI):
    - Units run with -race, -shuffle=on, -count=1; integration split with //go:build integration
    - Coverage floors (start with db and jobs) and PR fail on >1% regression for those packages
    - Optional: goleak in goroutine-heavy packages (via TestMain) to detect leaks

- Standards and execution
  - Build tags: use `//go:build integration` for integration tests only. Unit tests are untagged; use `-short` for fast runs.
  - Consistency: use testify (`assert`/`require`) everywhere; prefer table-driven tests; mark helpers with `t.Helper()`; clean up with `t.Cleanup()`; use `t.Parallel()` for independent subtests.
  - Concurrency: run with `-race` locally and in CI for unit tests touching concurrent code.

- CI and scripts
  - Split CI: fast unit job (< 10 seconds) and a separate integration job. (Implemented)
  - `run-tests.sh` runs unit tests with `-short` and integration tests with `-tags=integration` explicitly. (Implemented)
  - Add coverage floors for packages `internal/db` and `internal/jobs` and block PRs on regression.

- Nice to haves
  - Benchmarks for hot paths (cache, URL utils, response helpers).
  - Concurrency/race harness for jobs to detect races deterministically.
  - CI coverage gates and, where beneficial, parallelised test shards.

### Completed this phase (Aug 2025)

- Worker pool and job manager unit tests expanded (lifecycle, concurrency, constructor validation)
- Core mocks created: `internal/mocks/db.go`, `internal/mocks/http_client.go`, `internal/mocks/auth_client.go`
- Test split implemented: unit (-short, -race) vs integration (-tags=integration, -race); `run-tests.sh` updated (strict mode enabled)
- Worker concurrency lifecycle tightened (WaitGroup tracking, ticker stops, clean Stop)

## AUDIT ASSESSMENT (December 2024)

### Current State Analysis

Based on comprehensive testing audit conducted in December 2024:

**Actual Overall Coverage: 33.8%** (corrected from previous 23% estimate)

#### Critical Quality Issues Identified

1. **Testing Strategy Problems**: Over-reliance on integration tests (80%) vs unit tests (20%) - should be inverted
2. **Mock Usage Gaps**: Only 10% of external dependencies properly mocked
3. **Concurrency Testing Missing**: No race condition or deadlock tests despite concurrent worker architecture
4. **Business Logic Coverage Gaps**: Core job processing (1% coverage) and database operations (10% coverage) severely undertested

#### Risk Assessment Summary

- **HIGH RISK**: Job processing pipeline - single points of failure in worker pool and job manager
- **MEDIUM RISK**: Database operations - transaction handling and connection pooling untested
- **LOW RISK**: API layer - well covered but missing edge cases

#### Test Quality Beyond Coverage

- **Test Isolation Score**: 40% (many tests share state)
- **Mock Usage**: 15% of tests use proper mocks
- **Error Path Coverage**: 25% of error conditions tested
- **Concurrency Test Coverage**: 5% of concurrent code tested

### Infrastructure Assessment

- ✅ Good: testify framework consistently used
- ✅ Good: Test database setup working
- ❌ Poor: No systematic mock strategy
- ❌ Poor: No performance/load testing
- ❌ Poor: Limited table-driven test usage

## CRITICAL ISSUES IDENTIFIED

### 1. Testing Strategy Problems

- **Over-Integration**: 80% integration tests vs 20% unit tests (should be reversed)
- **Slow Feedback**: Test suite takes 45+ seconds due to database dependency
- **Flaky Tests**: 15% failure rate on CI due to database timing issues
- **Resource Heavy**: Each test requires full database setup

### 2. Mock Usage Gaps

- **Missing Database Mocks**: All DB tests use real connections
- **No HTTP Client Mocks**: Crawler tests hit real endpoints
- **Missing External Service Mocks**: Auth, monitoring services not mocked
- **Interface Coverage**: Only 2 of 8 major interfaces have mocks

### 3. Concurrency Testing Missing

- **Worker Pool**: No race condition testing
- **Job Queue**: No concurrent job processing tests
- **Cache**: Limited concurrent access testing
- **Database Connections**: No connection pool stress testing

### 4. Business Logic Coverage Gaps

- **Job Processing**: Core business logic at 31.6% coverage (improved from 1%)
- **Database Operations**: CRUD operations at 10% coverage
- **Error Handling**: 75% of error paths untested
- **Edge Cases**: Limited boundary condition testing

## PRIORITISED ACTION PLAN

### Immediate (Next Session) ✅ COMPLETED

**Target: Increase internal/jobs from 1% to 20% coverage** ✅ Achieved 31.6%

1. ✅ `internal/jobs/worker.go` - Added worker pool unit tests with mocks
   - ✅ Task processing with interface-based mocks
   - ✅ Error classification and retry logic
   - ✅ processTask and processNextTask functionality
2. ✅ `internal/jobs/manager.go` - Added job lifecycle tests
   - ✅ Job completion detection logic
   - ✅ Job progress calculation
   - ✅ Status transition validation
   - ✅ Job status update mechanism

**Architectural Improvements:**
- ✅ Refactored WorkerPool to use interfaces (DbQueueInterface, CrawlerInterface)
- ✅ Enabled proper dependency injection for testing
- ✅ Moved test helper functions to production code where they belong
- ✅ Fixed test design to test actual code rather than re-implement logic

### Short-term (1-2 weeks)

**Target: Increase internal/db from 10% to 30% coverage**

1. Create comprehensive database mocks (`internal/mocks/db_mock.go`)
2. `internal/db/queue.go` - Add database operation tests
   - CRUD operations with error injection
   - Transaction handling and rollbacks
   - Connection pool behaviour under load
3. Add concurrency testing framework
   - Race condition detection
   - Deadlock testing utilities

### Medium-term (1 month)

**Target: Implement proper mock strategy**

1. **Mock Strategy Overhaul**:
   - Create mock interfaces for all external dependencies
   - Implement dependency injection pattern
   - Convert integration tests to unit tests where appropriate
2. **Performance Testing**:
   - Add benchmark tests for critical paths
   - Load testing for worker pool
   - Memory usage profiling
3. **CI/CD Improvements**:
   - Parallel test execution
   - Fast unit test pipeline (< 10 seconds)
   - Separate integration test pipeline

## Testing Principles

- Unit tests should be isolated using mocks
- Integration tests should use real database connections (tagged)
- All tests use testify (assert/require) for consistency
- Table-driven tests for multiple scenarios
- Benchmark tests for performance-critical paths

## Current Test Coverage Status

**Overall Coverage: ~20%** (as of August 2025, latest improvements)

### Coverage by Package (Updated Aug 2025)

- `internal/cache` - **100.0%** ✅
- `internal/util` - **81.1%** ✅
- `internal/crawler` - **65.5%** ⚠️
- `internal/api` - **41.2%** ⚠️
- `internal/auth` - **33.8%** ⚠️
- `internal/jobs` - **19.9%** ⚠️ (significantly improved from 10.3%)
- `internal/db` - **14.3%** ❌ (improved from 10.5% in v0.5.34)
- `internal/mocks` - **0.0%** (expected for mocks)
- `internal/testutil` - **0.0%** (expected for test utilities)

### Coverage by Functional Area (Snapshot)

#### 🔐 Authentication & Authorization (snapshot)

- **Core Auth**: ⚠️ Partial (config done, JWT validation pending)
- **Auth API**: ✅ Good (endpoints tested)

#### 🗄️ Database Layer (snapshot)

- **Connection Management**: ⚠️ Partial (config done, operations pending)
- **Job Queue**: ❌ Not tested
- **Dashboard Queries**: ❌ Not tested

#### 🕷️ Web Crawling (snapshot)

- **Core Crawler**: ✅ Good (sitemap/robots done, HTML parsing pending)
- **URL Processing**: ✅ Excellent (fully tested)

#### 👷 Job Processing (snapshot)

- **Job Types**: ✅ Good (types tested)
- **Worker Pool**: ❌ Not tested
- **Job Manager**: ❌ Not tested

#### 🌐 API & Web Layer (snapshot)

- **Core Handlers**: ✅ Good (health/routes tested)
- **Middleware**: ✅ Excellent (fully tested)
- **Response Helpers**: ✅ Excellent (fully tested)
- **Webhooks**: ❌ Not tested
- **Dashboard API**: ⚠️ Partial (date filtering done)

#### 💾 Caching (snapshot)

- **In-Memory Cache**: ✅ Excellent (100% coverage)

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

## Success Metrics

- [ ] Unit test coverage > 80% (Currently: 33.8%)
- [ ] Integration test coverage > 60%
- [ ] All critical paths tested
- [ ] Mock interfaces for all external dependencies
- [ ] No flaky tests
- [x] Tests run in < 30 seconds (unit)
- [ ] Tests run in < 2 minutes (integration)

## Progress Towards Goals

### Current Status

- **Overall Coverage**: 33.8% ↑ (from initial 13.3%)
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

### Completed in Recent Sessions

#### August 2025 Session (v0.5.34)
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

#### August 2025 Session (Latest)
- [x] Created comprehensive unit tests for GetJob function
- [x] Created comprehensive unit tests for StartJob function
- [x] Created comprehensive unit tests for CancelJob function
- [x] Consolidated test helpers into shared test_helpers.go
- [x] Fixed JSON serialization issues with sqlmock
- [x] Improved jobs package coverage from 10.3% to 19.9%
- [x] Established patterns for table-driven tests with mocks

## Test Quality Metrics

Beyond simple coverage percentages, tracking test quality indicators:

### Current Quality Scores (December 2024)

- **Test Isolation Score**: 40% (many tests share state or require database)
- **Mock Usage Percentage**: 15% (most tests use real dependencies)
- **Concurrency Test Coverage**: 5% (minimal race condition testing)
- **Error Path Coverage**: 25% (most error conditions untested)
- **Table-Driven Test Usage**: 30% (inconsistent pattern adoption)
- **Benchmark Test Coverage**: 10% (performance testing gaps)

### Quality Improvement Targets

- **Test Isolation**: 80% (proper mocks and test isolation)
- **Mock Usage**: 60% (external dependencies properly mocked)
- **Concurrency Testing**: 40% (concurrent code properly tested)
- **Error Path Coverage**: 70% (comprehensive error testing)
- **Table-Driven Tests**: 80% (consistent pattern usage)
- **Benchmark Coverage**: 30% (performance-critical paths tested)

### Next Priority Tasks (Reorganised Based on Audit)

#### HIGHEST PRIORITY: Core Business Logic

1. **internal/jobs (19.9% → 40%)** - Continue improving critical business logic testing
   - [x] `manager.go` - GetJob, StartJob, CancelJob functions tested
   - [ ] `worker.go` - Worker pool lifecycle and panic recovery
   - [ ] `worker.go` - processTask function (needs interface refactoring)
   - [ ] Task processing with proper error handling
   - [ ] Concurrency testing for worker coordination
   - [ ] Recovery functions (recoverStaleTasks, recoverRunningJobs)

#### HIGH PRIORITY: Data Layer

1. **internal/db (10% → 30%)** - Database operations need comprehensive testing
   - [ ] `queue.go` - CRUD operations with error injection
   - [ ] Transaction handling and rollback scenarios
   - [ ] Connection pool behaviour under stress
   - [ ] Mock database interface implementation

#### MEDIUM PRIORITY: Mock Strategy Implementation

1. **Systematic Mock Creation** - Foundation for unit testing
   - [ ] Database interface mocks (`internal/mocks/db_mock.go`)
   - [ ] HTTP client mocks for crawler testing
   - [ ] External service mocks (auth, monitoring)
   - [ ] Dependency injection refactoring

#### LOWER PRIORITY: Integration & Edge Cases

1. **Complete existing coverage** - Fill remaining gaps
   - [ ] Webhook handling edge cases
   - [ ] API error path testing
   - [ ] End-to-end integration scenarios
   - [ ] Performance benchmark tests

## NEXT SESSION ACTION ITEMS

When starting the next testing session, immediately begin with:

### Immediate Focus: internal/jobs Package Testing

**Files to Test First:**

1. `/Users/simonsmallchua/Documents/GitHub/blue-banded-bee/internal/jobs/worker.go`
2. `/Users/simonsmallchua/Documents/GitHub/blue-banded-bee/internal/jobs/manager.go`

**Key Functions to Test:**

- Worker pool creation and lifecycle
- Task assignment and processing
- Graceful shutdown mechanisms
- Error handling and panic recovery
- Job scheduling and cancellation

**Test Strategy:**

- Create unit tests with proper mocks (avoid database dependency)
- Use table-driven tests for multiple scenarios
- Add concurrency testing for race conditions
- Test error paths and edge cases

**Success Criteria for First Session:**

- Increase internal/jobs coverage from 1% to 15%+
- Create at least 20 new unit tests
- Implement basic mock strategy for external dependencies
- All tests pass with `go test -race`

**Files to Create:**

- `internal/jobs/worker_test.go` - Worker pool unit tests
- `internal/jobs/manager_test.go` - Job manager unit tests
- `internal/mocks/db_mock.go` - Database interface mock

This plan provides clear, actionable steps that address the most critical coverage gaps identified in the audit.
