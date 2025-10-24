# Changelog

All notable changes to the Blue Banded Bee project will be documented in this
file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Multiple version updates may occur on the same date, each with its own version
number. Each version represents a distinct set of changes, even if released on
the same day.

## Release Automation

When merging to main, CI automatically creates releases based on the changelog:

- `## [Unreleased]` or `## [Unreleased:patch]` → Patch release (0.6.4 → 0.6.5)
- `## [Unreleased:minor]` → Minor release (0.6.4 → 0.7.0)
- `## [Unreleased:major]` → Major release (0.6.4 → 1.0.0)

On merge, CI will:

1. Calculate the new version number
2. Replace the heading with `## [X.Y.Z] - YYYY-MM-DD`
3. Add a new `## [Unreleased]` section above
4. Create a git tag and GitHub release
5. Commit the updated changelog

## [Unreleased]

## [0.11.1] – 2025-10-24

### Fixed

- **Dashboard Timezone Handling**: Jobs now display correctly in user's local
  timezone instead of UTC
  - Added automatic timezone detection using browser's IANA timezone string
    (e.g., "Australia/Sydney")
  - Backend converts "today"/"yesterday" boundaries to user's timezone with
    graceful UTC fallback
  - Added "Last Hour" and "Last 24 Hours" rolling window filters alongside
    existing calendar-day filters
  - URL-encodes timezone parameter to handle special characters (`Etc/GMT+10` →
    `Etc%2FGMT%2B10`)
  - Applied to both bb-auth-extension.js and bb-components.js integration paths

## [0.11.0] – 2025-10-24

### Enhanced

- **Batch Task Status Updates**: Implemented PostgreSQL batch UPDATE system
  reducing database transactions by 95% (3000/min → 60/min)
- **Error Classification**: Distinguish infrastructure failures (retry
  indefinitely) from data corruption (poison pill isolation)
- **Graceful Shutdown**: Retry logic with backoff ensures zero data loss during
  application shutdown
- **Sentry Integration**: Critical failure monitoring for poison pills, database
  unavailability, and shutdown errors

## [0.10.3] – 2025-10-24

### Fixed

- **Trigger Storm Causing Deadlocks**: Optimised job progress trigger to fire
  only on task status changes, reducing executions by 80%
- **Dashboard Timezone Issue**: Jobs created in local timezone not showing when
  UTC rolls over (fix deferred, documented in plans/)

## [0.10.2] – 2025-10-23

## [0.10.1] – 2025-10-22

## [0.10.0] – 2025-10-22

### Fixed

- **Connection Pool Exhaustion and Deployment Crashes**: Fixed database
  connection exhaustion causing application crashes
  - Changed deployment strategy from rolling to immediate (stops old machine
    before starting new)
  - Prevents attempting 90 connections during deploys (old + new machine)
  - Eliminates "remaining connection slots reserved for SUPERUSER" errors
  - Brief downtime (~30-60s) during deploys is acceptable trade-off
- **Recovery Batch Timeouts**: Increased statement timeout for maintenance
  operations
  - Increased statement timeout from 30s to 60s for recovery batches
  - Increased context timeout from 35s to 65s to accommodate longer queries
  - Fixes persistent timeout errors when recovering 1,000+ stuck tasks

### Changed

- **Environment-Based Resource Scaling**: Worker pools and database connections
  now scale based on APP_ENV environment variable
  - **Production**: 50 workers (max 50), 32 max connections, 13 idle connections
  - **Preview/Staging**: 10 workers (max 10), 10 max connections, 4 idle
    connections
  - **Development**: 5 workers (max 50), 3 max connections, 1 idle connection
  - Dynamic worker scaling enforces environment-specific limits in AddJob and
    performance-based scaling
  - Prevents preview apps from scaling beyond their connection pool capacity
  - Prevents resource exhaustion during PR testing
  - Stays well under Supabase's 48-connection pool limit
  - Development uses minimal connections to allow multiple local instances

## [0.9.1] – 2025-10-20

## [0.9.0] – 2025-10-20

### Fixed

- **Task Recovery System**: Rewrote stuck task recovery to use batch processing
  - Processes stuck tasks in batches of 100 (oldest first) preventing
    transaction timeouts
  - Failed batches use exponential backoff and bail out after 5 consecutive
    failures to prevent database hammering
  - Tasks from cancelled/failed jobs are marked as failed immediately instead of
    retrying
  - Increased maintenance statement timeout from 5s to 30s to allow recovery
    batches to complete when processing large backlogs
  - Fixes issue where thousands of tasks could remain stuck indefinitely due to
    all-or-nothing transaction rollbacks
- **Monitoring and Alerting**: Reduced Sentry event spam whilst improving alert
  quality
  - Reduced stuck task monitoring from every 5 seconds to every 5 minutes
  - Replaced per-task Sentry events with single aggregated alert reporting
    actual totals (not sample size)
  - Separated job completion checks (30s) from health monitoring (5min)
  - Expected reduction: from 3,600+ events/hour to ~12 events/hour

## [0.10.2] – 2025-10-23

## [0.10.1] – 2025-10-22

## [0.10.0] – 2025-10-22

## [0.9.2] – 2025-10-20

## [0.9.1] – 2025-10-20

## [0.9.0] – 2025-10-20

## [0.8.8] – 2025-10-19

## [0.8.7] – 2025-10-19

## [0.8.6] – 2025-10-19

### Fixed

- **Job Timeout Cleanup**: Automatically mark jobs as failed if pending for 5+
  minutes with no tasks, or running for 30+ minutes with no progress

## [0.8.5] – 2025-10-19

### Fixed

- **Large Sitemap Processing**: Batch sitemap URL enqueueing (1000 URLs per
  batch) to prevent database timeouts on sites with 10,000+ URLs

## [0.8.4] – 2025-10-19

### Fixed

- **Cache Warming Optimisation**: Skip second request for BYPASS/DYNAMIC
  (uncacheable) content
- **Timeout Enforcement**: Clarified HTTP client and context timeout protection
- **Exponential Backoff**: Added backoff for 503/rate-limiting errors (1s, 2s,
  4s, 8s, 16s, 32s, 60s max)

## [0.8.3] – 2025-10-19

### Added

- **Crawling Analysis and Planning**: Research to improve crawling success rates
  and fix timeout issues
- **Referer Header**: Added Referer header to crawler requests
- **Grafana Cloud OTLP Integration**: Configured OpenTelemetry trace export to
  Grafana Cloud Tempo
  - Traces show complete request journeys with timing breakdowns for debugging
    slow jobs
  - Automatically captures job processing, URL warming, database queries, and
    HTTP requests
  - Configured with `OTEL_EXPORTER_OTLP_ENDPOINT` and
    `OTEL_EXPORTER_OTLP_HEADERS` environment variables
  - Health check endpoints (`/health`) excluded from tracing to reduce noise

### Changed

- **Crawler Random Delay**: Adjusted from 0-333ms to 200ms-1s range
- **Reduced Log Noise**: Health check requests from Fly.io no longer generate
  INFO-level logs
  - Health checks still function normally but don't clutter production logs
  - Real API requests continue to be logged for observability
- **Cloudflare Analytics Support**: Updated Content Security Policy to allow
  Cloudflare Web Analytics beacon resources when the zone is proxied

### Fixed

- **OTLP Endpoint Configuration**: Fixed trace export to use full URL path for
  Grafana Cloud compatibility
  - Endpoint now correctly includes `/otlp/v1/traces` path
  - Authentication uses HTTP Basic Auth with Base64-encoded Instance ID and
    Access Policy Token

## [0.8.2] – 2025-10-17

### Fixed

- **Job Recovery**: ensured stale task and job cleanup runs even when the DB
  pool is saturated by routing maintenance updates through a dedicated low-cost
  transaction helper

## [0.8.1] – 2025-10-17

## [0.8.0] – 2025-10-17

### Security

- **JWT Signing Keys Migration**: Migrated from legacy JWT secrets to asymmetric
  JWT signing keys
  - Replaced HMAC (HS256) shared secret validation with JWKS-based public key
    validation
  - **Supports both RS256 (RSA) and ES256 (Elliptic Curve P-256) signing
    algorithms**
  - Added `github.com/MicahParks/keyfunc/v3` for production-ready JWKS handling
    with automatic caching and key rotation
  - Removed `SUPABASE_JWT_SECRET` environment variable - no longer needed with
    public key cryptography
  - Implemented audience validation supporting both `authenticated` and
    `service_role` tokens
  - Enhanced error handling with JWKS-specific error detection and Sentry
    integration
  - Added context cancellation handling for graceful request timeouts
  - Updated authentication to use Supabase's `/auth/v1/certs` JWKS endpoint
  - 10-minute JWKS cache refresh aligns with Supabase Edge cache duration
  - Improved security posture by eliminating shared secret vulnerabilities

### Changed

- **Authentication Configuration**: Simplified auth config structure
  - Removed `JWTSecret` field from `auth.Config` struct
  - Renamed environment variables for clarity:
    - `SUPABASE_URL` → `SUPABASE_AUTH_URL`
    - `SUPABASE_ANON_KEY` → `SUPABASE_PUBLISHABLE_KEY`
  - Updated `NewConfigFromEnv()` to only require `SUPABASE_AUTH_URL` and
    `SUPABASE_PUBLISHABLE_KEY`
  - Updated all authentication tests to use RS256 tokens with proper JWKS
    mocking

### Enhanced

- **Test Coverage**: Comprehensive JWT validation tests for both RS256 and ES256
  - Added test JWKS servers with RSA and Elliptic Curve key generation
  - Tests for valid tokens (both RS256 and ES256), invalid signatures, invalid
    audiences, and context cancellation
  - Helper functions `startTestJWKS()`, `signTestToken()`,
    `startTestJWKSWithES256()`, and `signTestTokenES256()`
  - All tests passing with 100% coverage on new JWKS functionality

## [0.7.3] – 2025-10-14

### Security

- **gRPC Dependency Update**: Updated `google.golang.org/grpc` from v1.64.0 to
  v1.64.1
  - Fixes potential PII leak where private tokens in gRPC metadata could appear
    in logs if context is logged
  - Addresses Dependabot security alert CVE (indirect dependency via
    OpenTelemetry)
  - No impact on Blue Banded Bee as we don't log contexts containing gRPC
    metadata

## [0.7.2] – 2025-10-14

## [0.7.1] – 2025-10-13

### Enhanced

- **Database Performance Optimisation**: Composite index strategy based on
  EXPLAIN ANALYZE profiling
  - Created `idx_tasks_claim_optimised` composite index for worker pool task
    claiming (50-70% latency reduction)
  - Added `idx_jobs_org_status_created` and `idx_jobs_org_created` composite
    indexes for dashboard queries (90%+ improvement, 11ms → <1ms)
  - Dropped unused indexes (`idx_jobs_stats`, `idx_jobs_avg_time`,
    `idx_jobs_duration`) saving ~1.3 MB and improving write performance
  - Eliminated sequential scans on jobs table (was scanning 5899 buffers for 164
    rows)
  - Migration: `20251013104047_add_composite_indexes_for_query_optimisation.sql`
  - Migration: `20251013103326_drop_unused_job_indexes.sql`

### Fixed

- **Database Connection Timeout Configuration**: Fixed nested timeout check bug
  - `idle_in_transaction_session_timeout` now correctly applied independently of
    `statement_timeout`
  - Previously, idle timeout was only added if statement_timeout was missing
  - Ensures zombie transaction cleanup works in all configurations

### Documentation

- **Database Performance**: Comprehensive documentation of optimisation strategy
  - Added "Connection Pool Sizing Strategy" section to DATABASE.md with sizing
    formulas and rationale
  - Documented composite index design and query patterns in DATABASE.md
  - Established PostgreSQL cache hit rate baseline (99.94% index, 99.76% table)
    via production query analysis
  - Both metrics exceed 99% target indicating optimal shared buffer
    configuration

## [0.7.0] – 2025-10-12

### Added

- **OpenTelemetry Tracing and Prometheus Metrics**: Comprehensive observability
  infrastructure for performance monitoring
  - Created dedicated `internal/observability` package with OpenTelemetry (OTLP)
    and Prometheus integration
  - Worker task tracing with span instrumentation for individual cache warming
    operations
  - Prometheus metrics endpoint (`/metrics`) exposing task duration histograms
    and counters
  - Configurable OTLP exporter for sending traces to Grafana Cloud or other
    OpenTelemetry backends
  - Environment-aware configuration with sampling controls (10% production, 100%
    development)
  - Process and Go runtime metrics automatically collected
  - HTTP request instrumentation via `otelhttp` middleware

- **Grafana Cloud Integration**: Production monitoring with Grafana Alloy for
  metrics collection
  - Deployed Grafana Alloy sidecar on Fly.io to scrape Prometheus metrics from
    application
  - Successfully configured metrics pipeline: App → Alloy → Grafana Cloud
    Prometheus
  - Resolved authentication and endpoint configuration for Cloud Access Policy
    tokens
  - 310+ metrics flowing into Grafana Cloud including database connections,
    worker performance, and HTTP traffic

- **Database Performance Optimisation**: Strategic indexing and query
  improvements
  - Added composite index `idx_tasks_running_started_at` on
    `(status, started_at)` for efficient stale task recovery
  - Enabled `pg_stat_statements` extension for PostgreSQL query performance
    analysis
  - Added `idle_in_transaction_session_timeout` (5 seconds) to prevent
    connection pool exhaustion
  - Cached normalised page paths on insert to reduce duplicate URL processing
  - Implemented duplicate page key check during URL enqueuing to prevent
    redundant tasks

- **Performance Testing Infrastructure**: Load testing tools for benchmarking
  and optimisation
  - Created `scripts/load-test-simple.sh` for automated performance testing
  - Batch job loading capability for testing with realistic workloads
  - Comprehensive documentation in `scripts/README-load-test.md`

- **Performance Research Documentation**: In-depth research on Go and PostgreSQL
  optimisation
  - Comprehensive analysis in `docs/research/2025-10/EVALUATION.md` covering
    profiling, database tuning, and architectural patterns
  - Documented 9 performance optimisation articles covering Go patterns,
    PostgreSQL pooling, and Supabase performance
  - Captured baseline performance metrics from Supabase dashboard for
    optimisation tracking

### Enhanced

- **Worker Pool Instrumentation**: Detailed telemetry for cache warming
  operations
  - Worker tasks emit OpenTelemetry spans with job ID, task ID, domain, path,
    and find_links attributes
  - Task duration and outcome metrics (completed/failed) recorded to Prometheus
  - Graceful shutdown with proper telemetry provider cleanup

- **Database Insert Efficiency**: Reduced redundant processing and improved
  throughput
  - Optimised insert operations to check for existing pages before database
    calls
  - Improved DB throttling to reduce duplicate queue insertions
  - Better handling of high-throughput scenarios with concurrent workers

- **HTTP Handler Instrumentation**: Automatic request tracing for API endpoints
  - `WrapHandler` function applies OpenTelemetry instrumentation when providers
    are active
  - Span names formatted as `METHOD /path` for clear trace visualisation
  - Trace and baggage context propagated across service boundaries

- **Link Extraction Performance**: Optimised visible link checker with reduced
  regex usage
  - Improved link visibility detection performance
  - Reduced CPU overhead from regex operations in crawler

### Fixed

- **Code Quality**: Addressed linting and formatting issues
  - Changed Codecov thresholds to informational mode (project-level only, not
    patch-level)
  - Fixed formatting across all modified files
  - Removed completed tasks from evaluation documentation

### Changed

- **Review App Workflow**: Skip documentation-only changes to reduce CI overhead
  - Review apps no longer deploy for `.md` file changes
  - Faster PR feedback cycle for documentation updates

- **Database Migrations**: New migrations for performance improvements
  - `20251012060206_idx_tasks_running_started_at.sql` - Adds composite index for
    worker recovery queries
  - `20251012070000_enable_pg_stat_statements.sql` - Enables query performance
    monitoring extension

### Documentation

- **Performance Analysis**: Extensive research documentation for future
  optimisation work
  - Supabase performance metrics baseline captured with 122 data points
  - Articles on Go performance patterns, database pooling, and microservices
    architecture
  - Evaluation plan documenting profiling methodology and optimisation targets

## [0.6.9] – 2025-10-12

## [0.6.8] – 2025-10-11

## [0.6.7] – 2025-10-11

### Fixed

- **Cache Warming Improvement Calculation**: Fixed "Improved Pages" incorrectly
  showing 100% when cache was already warm
  - Changed logic from `second_response_time < response_time` to
    `second_response_time > 0 AND second_response_time < response_time`
  - Pages with `second_response_time = 0` (already cached) are no longer counted
    as "improved"
  - Improvement rate now accurately reflects pages actually warmed by this job
  - Stats calculation version bumped to v4.0

## [0.6.6] – 2025-10-11

### Fixed

- **Job Metrics Calculation**: Fixed response time metrics displaying 0ms when
  cache warming is perfectly effective
  - Resolved bug where `COALESCE(second_response_time, response_time)` treated
    `0` as valid value instead of falling back to first request times
  - Now uses `NULLIF(second_response_time, 0)` to convert instant cache hits
    (0ms) to NULL, enabling fallback to meaningful first-request metrics
  - Recalculates existing jobs with buggy v1.0 or v2.0 stats automatically on
    migration
  - Stats calculation version bumped to v3.0

## [0.6.5] – 2025-10-11

### Fixed

- **Share Link API**: Return 200 with exists flag instead of 404 when no link
  exists
  - Changed GET /v1/jobs/:id/share-links to return 200 with `{"exists": false}`
    when no share link exists
  - Returns 200 with `{"exists": true, "token": "...", "share_link": "..."}`
    when link exists
  - Updated frontend to check exists field instead of 404 status
  - Eliminates console errors and provides cleaner API semantics

## [0.6.4] – 2025-10-11

### Added

- **Automated Release System**: CI now automatically creates releases when
  merging to main
  - Changelog-driven versioning using `[Unreleased]`, `[Unreleased:minor]`, or
    `[Unreleased:major]` markers
  - Auto-updates CHANGELOG.md with version number and date on merge
  - Creates git tags and GitHub releases with changelog content
  - All releases marked as pre-release until stable
- **Changelog Validation**: PR checks enforce changelog updates for all code
  changes
  - Blocks merges if `[Unreleased]` section is empty
  - Skips validation for docs/config-only changes

- **CI Formatting Enforcement**: Automated code formatting checks in CI pipeline
  - Added golangci-lint v2.5.0 with Australian English spell checking
  - Prettier formatting for Markdown, YAML, JSON, HTML, CSS, and JavaScript
  - Pre-commit hooks auto-format files before every commit
  - Format check job blocks merges if formatting is incorrect
- **Sentry Error Tracking on Preview Branches**: Preview apps now report to
  Sentry
  - Added `SENTRY_DSN` to review app secrets for staging environment visibility
  - 5% trace sampling for preview environments (vs 10% production)
  - Enables early issue detection before production deployment

### Fixed

- **Stuck Task Recovery**: Resolved recurring "task stuck in running state"
  errors
  - Fixed error handling in `recoverStaleTasks()` to properly rollback failed
    transactions
  - Recovery attempts now trigger transaction rollback when UPDATE fails
  - Expected 99.9% reduction in stuck task alerts (from 37k to <10 per
    occurrence)
- **Fly.io Review App Cleanup**: Fixed preview apps not being deleted after PR
  merge
  - Corrected YAML syntax error preventing cleanup job from running
  - Removed workflow-level paths-ignore that was blocking cleanup triggers
  - Added cleanup script for manual removal of orphaned apps
- **CI Coverage Report**: Fixed coverage job failing when tests are skipped
  - Coverage report now only runs if at least one test job succeeds
  - Prevents "no coverage files found" errors on documentation-only changes

### Enhanced

- **Code Quality Standards**: Documented linting requirements in CLAUDE.md
  - Australian English spelling enforced via misspell linter
  - Cyclomatic complexity limit: 35
  - Comprehensive linter suite: govet, staticcheck, errcheck, revive, gofmt,
    goimports

## [0.6.3] – 2025-10-09

### Added

- **Dashboard Share Links**: One-click share action on job cards generates and
  copies public URLs with inline feedback.
- **Share Link API Tests**: Contract coverage for create/reuse/revoke flows and
  shared endpoints.

### Improved

- **Unified Job Page**: `/shared/jobs/{token}` now reuses the job details
  template in read-only mode with shared API wiring.
- **Job Page Controls**: Share panel supports generate/copy/revoke with
  shared-mode guards and cleaner script loading.

## [0.6.2] – 2025-10-08

### Fixed

- **Worker Resilience**: Added per-task timeouts, panic recovery, and transient
  connection retries so stuck tasks and “bad connection” alerts recover
  automatically.

## [0.6.1] – 2025-10-08

### Added

- **Standalone Job Page**: Split the dashboard modal into `/jobs/{id}` with
  binder-driven stats, exports, metadata tooltips and pagination parity.

### Improved

- **Dashboard Binding Helpers**: Hardened metric visibility, tooltip loading,
  and task table rendering for the new job page.

## [0.6.0] – 2025-10-05

### Fixed

- **Database Security**: Resolved ambiguous column references in RLS policies
  - Fixed "column reference 'id' is ambiguous" errors preventing job cleanup
  - Qualified all column names with table names in RLS policy subqueries
  - Jobs no longer stuck in pending status due to SQL errors
- **Performance Metrics**: Corrected P95 response time display showing as NaN
  - Removed premature string conversion causing Math.round() to fail
  - P95 metric now displays correctly in job modal
- **Dashboard Authentication**: Fixed event delegation issues
  - Resolved authentication errors when accessing dashboard endpoints
  - Improved token handling and validation

### Changed

- **Failed Pages Metric**: Replaced broken links metric with failed pages count
  - Now counts tasks with `status='failed'` instead of HTTP 4xx codes only
  - Captures crawler errors that don't set HTTP status codes
  - Renamed `total_broken_links` to `total_failed_pages` for clarity
  - Removed `total_404s` metric entirely (redundant with failed pages)
  - Updated dashboard UI labels and export button text
  - Statistics calculation version bumped to v3.0
- **Performance Statistics**: Switched to second response time for cache
  effectiveness
  - Job statistics now use `second_response_time` (after cache warming) for all
    metrics
  - Provides more accurate representation of user-facing performance
  - First response time still tracked separately for cache improvement analysis

### Added

- **Metadata Tooltips**: Added help information for all job metrics
  - Info icon (🛈) displays contextual help for each statistic
  - Covers cache metrics, response times, failed pages, slow pages, redirects
  - Improved user understanding of dashboard metrics
- **CSP Headers**: Enhanced Content Security Policy for analytics
  - Added Google Tag Manager (GTM) to script-src and img-src
  - Added Google Analytics domains to connect-src
  - Added gstatic.com for Google services resources
- **CSV Export**: Improved data export functionality
  - Export buttons now correctly labelled (Failed Pages, Slow Pages)
  - CSV exports match updated metric definitions

### Improved

- **Dashboard Data Binding**: Enhanced attribute system for cleaner HTML
  - Updated from `data-*` to `bbb-*` attributes across dashboard
  - Backwards compatibility maintained during transition
  - Improved separation of concerns in frontend code
- **External Links**: Dashboard preview links in PR comments open in new tabs
  - Added `target="_blank"` for better user experience

## [0.10.2] – 2025-10-23

## [0.10.1] – 2025-10-22

## [0.10.0] – 2025-10-22

## [0.9.2] – 2025-10-20

## [0.9.1] – 2025-10-20

## [0.9.0] – 2025-10-20

## [0.8.8] – 2025-10-19

## [0.8.7] – 2025-10-19

## [0.8.6] – 2025-10-19

## [0.8.5] – 2025-10-19

## [0.8.4] – 2025-10-19

## [0.8.3] – 2025-10-19

## [0.8.2] – 2025-10-17

## [0.8.1] – 2025-10-17

## [0.8.0] – 2025-10-17

## [0.7.3] – 2025-10-14

## [0.7.2] – 2025-10-14

## [0.7.1] – 2025-10-13

## [0.7.0] – 2025-10-12

## [0.6.9] – 2025-10-12

## [0.6.8] – 2025-10-11

## [0.6.7] – 2025-10-11

## [0.6.6] – 2025-10-11

## [0.6.5] – 2025-10-11 – 2025-08-18

### Enhanced

- **Comprehensive API Testing Infrastructure**: Complete testing foundation for
  Stage 5
  - Added comprehensive tests for all major API endpoints (createJob, getJob,
    updateJob, cancelJob, getJobTasks)
  - Achieved 33.2% API package coverage (+1500% improvement from baseline)
  - Implemented interface-based testing (JobManagerInterface) and sqlmock
    patterns
  - Added comprehensive dashboard and webhook endpoint testing
  - Created separated test file structure for maintainability (test_mocks.go,
    jobs_create_test.go, etc.)
- **Function Refactoring Excellence**: Major complexity reduction using
  Extract + Test + Commit
  - Completed processTask refactoring: 162 → 28 lines (83% reduction)
  - Completed processNextTask refactoring: 136 → 31 lines (77% reduction)
  - Created 6 focused, single-responsibility functions with 100% coverage on
    testable functions
  - Achieved consistent 75-85% complexity reductions across targeted functions

### Added

- **Testing Architecture**: Interface-based dependency injection enabling
  comprehensive unit testing
  - MockJobManager and MockDBClient for isolated API testing
  - Sqlmock integration for testing direct SQL query functions
  - Auth context utilities testing (GetUserFromContext: 0% → 100%)
  - Table-driven test patterns with comprehensive edge case coverage

### Improved

- **Documentation Cleanup**: Streamlined and accuracy-focused documentation
  - Retired REFACTOR_PLAN.md after successful completion of methodology goals
  - Streamlined TEST_PLAN.md to forward-focused testing guide
  - Removed outdated testing documentation with inaccurate coverage claims
  - Deleted redundant and completed planning documents

## [0.5.34+] – 2025-08-16

### Improved

- **Code Architecture**: Major refactoring eliminating monster functions (>200
  lines)
  - Applied Extract + Test + Commit methodology across 5 core functions
  - 80% reduction in function complexity (1353 → 274 lines total)
  - Created 23 focused, single-responsibility functions with comprehensive tests
- **Testing Coverage**: Expanded from 30% to 38.9% total coverage
  - Added 350+ test cases across API, database, job management, and crawler
    logic
  - Introduced focused unit testing patterns with comprehensive mocking
  - Implemented table-driven tests and edge case validation
- **API Architecture**: Improved async patterns and error handling
  - CreateJob returns immediately with background processing
  - Proper context propagation with timeouts for goroutines
  - Idiomatic Go error patterns throughout
- **Database Operations**: Simplified and modularised core database functions
  - Separated table creation, indexing, and security setup
  - Enhanced testability with focused functions

## [Previous] – 2025-08-16

### Enhanced

- **Test Coverage Expansion**: Major improvements to jobs package testing
  - Improved test coverage: jobs package (1% → 31.6%)
  - Refactored WorkerPool to use interfaces for proper dependency injection
  - Created comprehensive unit tests for worker processing and job lifecycle
  - Moved helper functions from tests to production code where they belong
  - Fixed test design to test actual code rather than re-implement logic

### Added

- **Interface-Based Architecture**: Enabled proper unit testing
  - `DbQueueInterface` - Interface for database queue operations
  - `CrawlerInterface` - Extended with GetUserAgent method
  - Mock implementations for both interfaces in tests
- **Worker Processing Tests**: Core task processing functionality
  - `worker_process_test.go` - Tests for processTask and processNextTask
  - Error classification and retry logic tests
  - Task processing with various scenarios (delays, redirects, errors)

- **Job Lifecycle Tests**: Job management functionality
  - `job_lifecycle_test.go` - Tests for job completion detection
  - Job progress calculation tests
  - Status transition validation tests
  - Job status update mechanism tests

- **Production Helper Methods**: Moved from tests to JobManager
  - `IsJobComplete()` - Determines when a job is finished
  - `CalculateJobProgress()` - Calculates job completion percentage
  - `ValidateStatusTransition()` - Validates job status changes
  - `UpdateJobStatus()` - Updates job status with timestamps

### Fixed

- **Architectural Issues**: Resolved design problems preventing testing
  - WorkerPool now accepts interfaces instead of concrete types
  - CreatePageRecords now accepts TransactionExecutor interface
  - Removed unused methods from DbQueueInterface
  - Added missing GetUserAgent to MockCrawler

## [0.5.34] – 2025-08-08

### Enhanced

- **Test Infrastructure Improvements**: Comprehensive test suite enhancements
  - Fixed critical health endpoint panic when DB is nil - now returns 503 status
  - Made DBClient interface fully mockable for unit testing
  - Added sqlmock tests for database health endpoint
  - Extracted DSN augmentation logic to testable helper function
  - Created comprehensive unit tests for worker and manager components
  - Fixed broken `contains()` function in advanced worker tests
  - Added proper cleanup with `t.Cleanup()` for resource management
  - Removed fragile timing assertions in middleware tests
  - Enabled fail-fast behaviour in test scripts with `set -euo pipefail`

### Added

- **Mock Infrastructure**: Complete mock implementations for testing
  - Expanded MockDB with all DBClient interface methods
  - Created MockDBWithRealDB wrapper for sqlmock integration
  - Added comprehensive DSN helper tests covering URL and key=value formats

### Fixed

- **Test Quality Issues**: Resolved critical test suite problems
  - Fixed placeholder tests in db_operations_test.go
  - Centralised version string management to avoid hardcoded values
  - Improved test coverage: db package (10.5% → 14.3%), jobs package (1.1% →
    5.0%)
  - Modernised interface{} to any throughout test files

## [0.5.33] – 2025-08-06

### Enhanced

- **Webhook ID**: Created unique field on user for use in Webhook verification
  (Webflow) rather than using user ID

### Fixed

- **Account creation**: New accounts weren't being created and org name was
  wrong
  - Updated sign in or create account to create a new profile
  - Fix logic to create org name based on domain

- **Supabase / Github workflow**: Fixed schema issues with main vs. test-branch
  in supabase and github
  - Deleted all data from Supabase, including a gigantic job (abc.net.au) that
    was in an infinite loop and huge dataset and was just for testing.
  - Deleted all migrations
  - Created new clean migration file for both branches
  - Setup preview branching for PRs in Github to apply migrations there for
    tests

## [0.5.33] – 2025-08-02

### Enhanced

- **Admin Endpoint Security**: Implemented proper authentication for admin
  endpoints
  - Added `system_role` authentication requirement for all admin endpoints
    (`/admin/*`)
  - Admin endpoints now require `system_role` claim in JWT token for access
  - Unauthorised access attempts properly rejected with 403 Forbidden responses
  - Comprehensive test coverage added for admin authentication scenarios
  - Security enhancement ensures admin functionality is protected in production

## [0.5.32] – 2025-08-01

### Fixed

- **Job Progress Counting**: Fixed database trigger causing completed_tasks to
  exceed total_tasks
  - Updated `update_job_progress` trigger to recalculate total_tasks from actual
    task count
  - Migration: `20250801113006_fix_update_job_progress_trigger_total_tasks.sql`

## [0.5.31] – 2025-07-28

### Added

- **Comprehensive robots.txt Compliance**:
  - Added robots.txt parsing and crawl-delay honouring
  - Implemented URL filtering against Disallow/Allow patterns
  - Added robots.txt caching at job level to prevent repeated fetches
  - Manual root URLs now fail if robots.txt cannot be checked
  - Dynamically discovered links are filtered against robots rules
  - Added GetUserAgent method to crawler for proper identification
  - Added 1MB size limit for robots.txt parsing (security)

### Changed

- **Performance Optimisation**:
  - Robots.txt is now fetched once per job and cached in worker pool
  - Database query reduced from per-task to per-job for job information
  - Refactored processSitemap into smaller, maintainable functions

### Fixed

- **Interface Cleanup**: Removed duplicate DiscoverSitemaps method from
  interfaces
- **Security**: Reduced robots.txt size limit from 10MB to 1MB to prevent memory
  exhaustion

## [0.5.30] – 2025-07-27

### Added

- **Comprehensive Test Suite**:
  - Integration tests for core job operations (GetJob, CreateJob, CancelJob,
    ProcessSitemapFallback, EnqueueJobURLs)
  - Unit tests with mocks using testify framework
  - Refactored to use interfaces for better testability (CrawlerInterface)
  - Test coverage reporting with Codecov (17.4% coverage achieved)
  - Test Analytics enabled with JUnit XML reports
  - Codecov Flags and Components configuration for better test categorisation
- **Codecov Configuration**: Added codecov.yml for coverage reporting settings
- **Post-Launch API Testing Plan**: Created comprehensive testing strategy for
  implementation after product launch

### Changed

- **CI/CD Pipeline**:
  - Updated to use Supabase pooler URLs for IPv4 compatibility in GitHub Actions
  - Separated test workflow from deployment workflow
  - Added unit and integration test separation with build tags
- **Test Environment**: Standardised on TEST_DATABASE_URL for all test database
  connections
- **Testing Documentation**: Reorganised into modular structure under
  docs/testing/
- **Project Guidance**: Updated CLAUDE.md and gemini.md with platform
  documentation verification approach

### Fixed

- **CI Database Connection**: Resolved IPv6 connectivity issues by using
  Supabase session pooler
- **Test Environment Loading**: Fixed test configuration to properly use
  .env.test file
- **Coverage Calculation**: Fixed coverage reporting to include all packages
  with -coverpkg=./...
- **Test Race Conditions**: Implemented polling approach instead of fixed sleep
  times

## [0.5.29] – 2025-07-26

### Added

- **Sitemap Fallback**: Falls back to crawling from root page when sitemap
  unavailable
- **Database Migrations**: Transitioned to migration-based database management

## [0.5.28] – 2025-07-19

### Fixed

- **Memory Leak**: Removed unbounded in-memory HTTP cache that was causing
  memory exhaustion during crawl jobs. The cache was storing entire HTML pages
  without eviction, leading to out-of-memory crashes.

## [0.5.27] – 2025-07-19

### Enhanced

- **DB Optimisation**: Implemented a bunch of indexes on Supabase tables and
  deleted all historical data on pages/domains/tasks/jobs to clean and speed up.

## [0.5.26] – 2025-07-07

### Enhanced

- **Crawler Efficiency**: Implemented an in-memory cache for the HTTP client
  used by the crawler. This significantly reduces bandwidth and speeds up
  crawling by preventing the repeated download of assets (like JavaScript and
  CSS) within the same crawl job.

## [0.5.25] – 2025-07-06

### Github action updates

- **Codecov**: Implemented integration to report on testing coverage, indicated
  in badge in README
- **Go Report**: Added code quality reporting into README

## [0.5.24] – 2025-07-06

### Security

- **CSRF Protection**: Implemented global Cross-Site Request Forgery (CSRF)
  protection by adding Go 1.25's experimental `http.CrossOriginProtection`
  middleware to all API endpoints. This hardens the application against
  malicious cross-origin requests that could otherwise perform unauthorised
  actions on behalf of an authenticated user.

## [0.5.23] – 2025-07-06

### Added

- **Performance Debugging**: Implemented Go's built-in flight recorder
  (`runtime/trace`) to allow for in-depth performance analysis of the
  application in production environments. The trace data is accessible via the
  `/debug/fgtrace` endpoint.

### Fixed

- **Flight Recorder**: Corrected the flight recorder's shutdown logic to ensure
  `trace.Stop()` is called during graceful server shutdown instead of
  immediately on startup. This allows the recorder to capture the full
  application lifecycle, making it usable for production performance debugging.

## [0.5.22] – 2025-07-03

### Enhanced

- **Database Performance**: Implemented an in-memory cache for page lookups
  (`pages` table) to significantly reduce redundant "upsert" queries. This
  dramatically improves performance during the page creation phase of a job by
  caching results for URLs that are processed multiple times within the same
  job.

## [0.5.21] – 2025-07-03

### Changed

- **Database Driver**: Switched the PostgreSQL driver from `lib/pq` to the more
  modern and performant `pgx`.
  - This resolves underlying issues with connection poolers (like Supabase
    PgBouncer) without requiring connection string workarounds.
  - The `prepare_threshold=0` setting is no longer needed and has been removed.
- **Notification System**: Rewrote the database notification listener
  (`LISTEN/NOTIFY`) to use `pgx`'s native, more robust implementation, improving
  real-time worker notifications.

### Enhanced

- **Database Performance**: Optimised the `tasks` table indexing for faster
  worker performance.
  - Replaced several general-purpose indexes with a highly specific partial
    index (`idx_tasks_pending_claim_order`) for the critical task-claiming
    query.
  - This significantly improves the speed and scalability of task processing by
    eliminating expensive sorting operations.

### Fixed

- **Graceful Shutdown**: Fixed an issue where the new `pgx`-based notification
  listener would not terminate correctly during a graceful shutdown, preventing
  the worker pool from stopping cleanly.

## [0.5.20] – 2025-07-03

### Added

- **Cache Warming Auditing**: Added detailed auditing for the cache warming
  retry mechanism.
  - The `tasks` table now includes a `cache_check_attempts` JSONB column to
    store the results of each `HEAD` request check.
  - Each attempt logs the cache status and the delay before the check.

### Enhanced

- **Cache Warming Strategy**: Improved the cache warming retry logic for more
  robust cache verification.
  - Increased the maximum number of `HEAD` check retries from 5 to 10.
  - Implemented a progressive backoff for the delay between checks, starting at
    2 seconds and increasing by 1 second for each subsequent attempt.

### Fixed

- **Database Connection Stability**: Resolved a critical issue causing
  `driver: bad connection` and `unexpected Parse response` errors when using a
  connection pooler (like Supabase PgBouncer).
  - The PostgreSQL connection string now includes `prepare_threshold=0` to
    disable server-side prepared statements, ensuring compatibility with
    transaction-based poolers.
  - Added an automatic schema migration (`ALTER TABLE`) to ensure the
    `cache_check_attempts` column is added to existing databases.

## [0.5.19] – 2025-07-02

### Enhanced

- **Task Prioritisation**: Refactored job initiation and link discovery for more
  accurate and efficient priority assignment.
  - The separate, post-sitemap homepage scan for header/footer links has been
    removed, eliminating a redundant HTTP request and potential race conditions.
  - The homepage (`/`) is now assigned a priority of `1.000` directly during
    sitemap processing.
  - Link discovery logic is now context-aware:
    - On the homepage, links in the `<header>` are assigned priority `1.000`,
      and links in the `<footer>` get `0.990`.
    - On all other pages, links within `<header>` and `<footer>` are ignored,
      preventing low-value navigation links from being crawled repeatedly.
    - Links in the page body inherit their priority from the parent page as
      before.

## [0.5.18] – 2025-07-02

### Enhanced

- **Crawler Efficiency**: Implemented a comprehensive visibility check to
  prevent the crawler from processing links that are hidden. The check includes
  inline styles (`display: none`, `visibility: hidden`), common utility classes
  (`hide`, `d-none`, `sr-only`, etc.), and attributes like `aria-hidden="true"`,
  `data-hidden`, and `data-visible="false"`. This significantly reduces the
  number of unnecessary tasks created.

## [0.5.17] – 2025-07-02

### Added

- **Task Logging**: Included the `priority_score` in the log message when a task
  is claimed by a worker for improved debugging.

### Fixed

- **Crawler Stability**: Fixed an infinite loop issue where relative links
  containing only a query string (e.g., `?page=2`) were repeatedly appended to
  the current URL instead of replacing the existing query.

## [0.5.16] – 2025-07-02

### Enhanced

- **User Registration**: The default organisation name is now set to the user's
  full name upon registration for a more personalized experience.
- **Organisation Name Cleanup**: Organisation names derived from email addresses
  are now cleaned of common TLDs (e.g., `.com`), ignores generic domains, and
  doesn't capitalise.

### Fixed

- **Database Efficiency**: Removed a redundant database call in the page
  creation process by passing the domain name as a parameter.
- **Task Auditing**: Ensured that the `retry_count` for a task is correctly
  preserved when a task succeeds after one or more retries.

## [0.5.15] – 2025-07-02

### Changed

- **Codebase Cleanup**: Numerous small changes to improve code clarity,
  including fixing comment typos, removing unused code, and standardising
  function names.
- **Worker Pool Logic**: Simplified worker scaling logic and reduced worker
  sleep time to improve responsiveness.

### Fixed

- **Architectural Consistency**: Corrected a flaw where the `WorkerPool` did not
  correctly use the `JobManager` for enqueueing tasks, ensuring
  duplicate-checking logic is now properly applied.

### Documentation

- **Project Management**: Updated `TODO.md` to convert all file references to
  clickable links and consolidated several in-code `TODO` comments into the main
  file for better tracking.
- **AI Collaboration**: Added `gemini.md` to document the best practices and
  working protocols for AI collaboration on this project.
- **Language Standardisation**: Renamed `Serialize` function to `Serialise` to
  maintain British English consistency throughout the codebase.

## [0.5.14] – 2025-06-26

### Added

- **Task Prioritisation Implementation**: Implemented priority-based task
  processing system
  - Added `priority_score` column to tasks table (0.000-1.000 range)
  - Tasks now processed by priority score (DESC) then creation time (ASC)
  - Homepage automatically gets priority 1.000 after sitemap processing
  - Header links (detected by common paths) also get priority 1.000
  - Discovered links inherit 80% of source page priority (propagation)
  - Added index `idx_tasks_priority` for efficient priority-based queries
  - All tasks start at 0.000 and only increase based on criteria

### Enhanced

- **Task Processing Order**: Changed from FIFO to priority-based processing
  - High-value pages (homepage, header links) crawled first
  - Important pages discovered early get higher priority
  - Ensures critical site pages are cached before less important ones

### Fixed

- **Recrawl pages with EXPIRED cache status**

## [0.5.13] – 2025-06-26

### Added

- **Task Prioritisation Planning**: Created comprehensive plan for
  priority-based task processing
  - PostgreSQL view-based approach using percentage scores (0.0-1.0)
  - Minimal schema changes - only adds `priority_score` column to pages table
  - Homepage detection and automatic highest priority assignment
  - Link propagation scoring - pages inherit 80% of source page priority
  - Detailed implementation plan in
    [docs/plans/task-prioritization.md](docs/plans/_archive/task-prioritisation.md)

### Enhanced

- **Job Duplicate Prevention**: Cancel existing jobs when creating new job for
  same domain
  - Prevents multiple concurrent crawls of same domain
  - Automatically cancels in-progress jobs for domain before creating new one
  - Improves resource utilisation and prevents redundant crawling

- **Cache Warming Timing**: Adjusted delay for second cache warming attempt
  - Increased delay to 1.5x initial response time for better cache propagation
  - Added randomisation to cache warming delays for more natural traffic
    patterns
  - Enhanced logging of cache status results for analysis

### Fixed

- **Link Discovery**: Fixed link extraction to properly find paginated links
  - Restored proper link discovery functionality that was inadvertently disabled

## [0.5.12] – 2025-06-06

### Enhanced

- **Cache Warming Timing**: Added 500ms delay between first request (cache MISS
  detection) and second request (cache verification) to allow CDNs time to
  process and cache the first response
- **Webflow Webhook Domain Selection**: Fixed webhook to use first domain
  (primary/canonical) instead of last domain (staging .webflow.io)
- **Webflow Webhook Page Limits**: Removed 100-page limit for webhook-triggered
  jobs - now unlimited for complete site cache warming

### Fixed

- **Build Error**: Removed unused `fmt` import from `internal/api/handlers.go`
  that was causing GitHub Actions build failures

## [0.5.11] – 2025-06-06

### Added

- **Webflow Webhook Integration**: Automatic cache warming triggered by Webflow
  site publishing
  - Webhook endpoint `/v1/webhooks/webflow/USER_ID` for user-specific triggers
  - Automatic job creation and execution when Webflow sites are published
  - Smart domain selection (uses first domain in array - primary/canonical
    domain)
- **Job Source Tracking**: Comprehensive tracking of job creation sources for
  debugging and analytics
  - `source_type` field: `"webflow_webhook"` or `"dashboard"`
  - `source_detail` field: Clean display text (publisher name or action type)
  - `source_info` field: Full debugging JSON (webhook payload or request
    details)

### Enhanced

- **Job Creation Architecture**: Refactored to eliminate code duplication
  - Extracted shared `createJobFromRequest()` function for consistent job
    creation
  - Webhook and dashboard endpoints now use common job creation logic
  - Improved maintainability and consistency across creation sources

### Technical Implementation

- **Database Schema**: Added `source_type`, `source_detail`, and `source_info`
  columns to jobs table
- **Webhook Security**: No authentication required for webhooks (Webflow can
  POST directly)
- **Source Attribution**: Dashboard jobs tagged as `"create_job"` ready for
  future `"retry_job"` actions

## [0.5.10] – 2025-06-06

### Fixed

- **Cache Warming Data Storage**: Fixed second cache warming data not being
  stored in database
- **Timeout Retry Logic**: Added automatic retry for network timeouts and
  connection errors up to 5 attempts

## [0.5.9] – 2025-06-06

### Enhanced

- **Worker Pool Scaling**: Improved auto-scaling for better performance and bot
  protection
  - Simplified worker scaling from complex job-requirements tracking to simple
    +5/-5 arithmetic per job
  - Auto-scaling: 1 job = 5 workers, 2 jobs = 10 workers, up to maximum 50
    workers (10 jobs)
  - Each job gets dedicated workers preventing single-job monopolisation and bot
    detection risks
- **Database Connection Pool**: Increased to support higher concurrency
  - MaxOpenConns: 25 → 75 connections to prevent bottlenecks with increased
    worker count
  - MaxIdleConns: 10 → 25 connections for better connection reuse
- **Crawler Rate Limiting**: Reduced aggressive settings for better politeness
  to target servers
  - MaxConcurrency: 50 → 10 concurrent requests per crawler instance
  - RateLimit: 100 → 10 requests per second for safer cache warming

### Technical Implementation

- **Simplified Scaling Logic**: Removed complex `jobRequirements` map and
  maximum calculation logic
  - `AddJob()`: Simple `currentWorkers + 5` with max limit of 50
  - `RemoveJob()`: Simple `currentWorkers - 5` with minimum limit of 5
  - Eliminated per-job worker requirement tracking for cleaner, more predictable
    scaling

## [0.5.8] – 2025-06-03

### Added

- **Cache Warming System**: Implemented blocking cache warming with second HTTP
  requests on cache MISS/BYPASS
  - Added `second_response_time` and `second_cache_status` columns to track
    cache warming effectiveness
  - Cache warming logic integrated into crawler with automatic MISS detection
    from multiple CDN headers
  - Blocking approach ensures complete metrics collection and immediate cache
    verification
  - Supabase can calculate cache warming success (`cache_warmed`) using:
    `second_cache_status = 'HIT'`

### Fixed

- **Critical Link Extraction Bug**: Fixed context handling bug that was
  preventing all link discovery
  - Link extraction was defaulting to disabled when `find_links` context value
    wasn't properly set
  - Now defaults to enabled link extraction, fixing pagination link discovery
    (e.g., `?b84bb98f_page=2`)
  - **TODO: Verify this fix works by testing teamharvey.co/stories pagination
    links**
- **Link Extraction Logic**: Consolidated to Colly-only crawler to extract only
  user-clickable links
  - Removed overly aggressive filtering that was blocking legitimate navigation
    links
  - Now only filters empty hrefs, fragments (#), javascript:, and mailto: links
- **Dashboard Form**: Fixed max_pages input field to consistently show default
  value of 0 (unlimited)

### Enhanced

- **Code Architecture**: Eliminated logic duplication in cache warming
  implementation
  - Cache warming second request reuses main `WarmURL()` method with
    `findLinks=false`
  - Removed redundant `cache_warmed` field - can be calculated in
    Supabase/dashboard
  - Database schema includes cache warming columns in initial table creation for
    new installations

## [0.5.7] – 2025-06-01

### Fixed

- **Critical Job Creation Bug**: Resolved POST request failures preventing job
  creation functionality
  - Fixed `BBDataBinder.fetchData()` method to properly accept and use options
    parameter for method, headers, and body
  - Method signature updated from `async fetchData(endpoint)` to
    `async fetchData(endpoint, options = {})`
  - POST requests now correctly send JSON body data instead of being converted
    to GET requests
  - Job creation modal now successfully creates jobs and refreshes dashboard
    data
- **Data Binding Library**: Enhanced fetchData method to support all HTTP
  methods
  - Added proper options parameter spread to fetch configuration
  - Maintained backward compatibility for GET requests (existing code
    unaffected)
  - Fixed API integration throughout dashboard for POST, PUT, DELETE operations

### Enhanced

- **Job Creation Modal**: Simplified interface with essential fields only
  - Removed non-functional include_paths and exclude_paths fields that aren't
    implemented in API
  - Hidden concurrency setting as job-level concurrency has no effect (system
    uses global concurrency of 50)
  - Set sensible defaults: use_sitemap=true, find_links=true, concurrency=5
  - Changed domain input from URL type to text type to allow simple domain names
    (e.g., "teamharvey.co")
- **User Experience**: Improved job creation workflow with better validation and
  feedback
  - Domain input now accepts domain names without requiring full URLs with
    protocol
  - Better error messaging when job creation fails
  - Real-time progress updates after successful job creation
  - Toast notifications for success and error states

### Technical Implementation

- **Data Binding Library Rebuild**: Updated and rebuilt all Web Components with
  fetchData fix
  - Rebuilt `bb-data-binder.js` and `bb-data-binder.min.js` with corrected
    method implementation
  - Updated `bb-components.js` and `bb-components.min.js` for production
    deployment
  - All POST/PUT/DELETE API calls throughout the application now function
    correctly
- **API Integration**: Fixed job creation endpoint integration
  - `/v1/jobs` POST endpoint now receives proper JSON data from dashboard
  - Request debugging confirmed proper method, headers, and body transmission
  - Removed debug logging after confirming fix works correctly

### Development Process

- **Testing Workflow**: Comprehensive debugging and testing of job creation flow
  - Traced request path from modal form submission through data binding library
    to API
  - Console logging confirmed fetchData method was ignoring POST parameters
  - Verified fix works by testing job creation with various domain inputs
  - Confirmed dashboard refresh and real-time updates work after job creation

## [0.5.6] – 2025-06-01

### Enhanced

- **User Experience**: Improved dashboard user identification and testing
  workflow
  - Updated dashboard header to display actual user email instead of placeholder
    "user@example.com"
  - Added automatic user avatar generation with smart initials extraction from
    email addresses
  - Real-time user info updates when authentication state changes
    (login/logout/token refresh)
  - Enhanced user session management with proper cleanup and state
    synchronisation

### Fixed

- **Dashboard User Display**: Resolved hardcoded placeholder in user interface
  - Replaced static "user@example.com" text with dynamic user email from
    Supabase session
  - Fixed avatar initials to properly reflect current authenticated user
  - Added fallback states for loading and error conditions
  - Improved authentication state listening for immediate UI updates

### Technical Implementation

- **Session Management**: Enhanced authentication flow integration
  - Direct Supabase session querying for reliable user data access
  - Auth state change listeners update user info automatically across
    login/logout cycles
  - Graceful error handling for session retrieval failures
  - Smart initials generation supporting various email formats
    (firstname.lastname, firstname_lastname, etc.)

### Development Workflow

- **Production Testing**: Completed full 6-step development workflow
  - GitHub Actions deployment successful for user interface improvements
  - Playwright MCP testing infrastructure verified (with noted session stability
    issues)
  - Production deployment confirmed working via manual verification

## [0.5.5] – 2025-06-01

### Added

- **Authentication Testing Infrastructure**: Comprehensive testing workflow for
  authentication flows
  - Created and successfully tested account creation with
    `simon+claude@teamharvey.co` using real-time password validation
  - Implemented real-time password strength checking using zxcvbn library with
    visual feedback indicators
  - Added password confirmation validation with visual success/error states
  - Comprehensive authentication modal testing via MCP Playwright browser
    automation
  - Database verification of account creation process (account created but
    requires email confirmation)

### Enhanced

- **Authentication Modal UX**: Production-ready authentication interface with
  industry-standard patterns
  - Real-time password strength evaluation using zxcvbn library (0-4 scale with
    colour-coded feedback)
  - Live password confirmation matching with instant visual validation feedback
  - Enhanced form validation with field-level error states and success
    indicators
  - Improved user experience with immediate feedback on password quality and
    match status
  - Modal-based authentication flow supporting login, signup, and password reset
    workflows

### Fixed

- **Domain References**: Corrected all application URLs to use proper domain
  structure
  - Updated authentication redirect URLs from `bluebandedbee.co` to
    `app.bluebandedbee.co`
  - Fixed API base URLs in Web Components to point to `app.bluebandedbee.co`
  - Updated all script URLs and CDN references in examples and documentation
  - Rebuilt Web Components with correct production URLs

### Documentation

- **Domain Usage Clarification**: Comprehensive documentation of domain
  structure and usage
  - **Local development**: `http://localhost:8080` - Blue Banded Bee application
    for local testing
  - **Production marketing site**: `https://bluebandedbee.co` - Marketing
    website only
  - **Production application**: `https://app.bluebandedbee.co` - Live
    application, services, demo pages
  - **Authentication service**: `https://auth.bluebandedbee.co` - Supabase
    authentication (unchanged)
  - Updated all documentation files to clearly specify domain purposes and usage
    contexts

### Technical Implementation

- **Authentication Flow Testing**: Complete browser automation testing of
  authentication workflows
  - Tested modal opening/closing, form switching between login/signup modes
  - Verified real-time password validation and strength indicators
  - Confirmed account creation process with database verification
  - Established testing patterns for future authentication feature development
- **Web Components Updates**: Rebuilt production components with correct domain
  configuration
  - Updated `web/src/utils/api.js` and rebuilt distribution files
  - Fixed OAuth redirect URLs in `dashboard.html`
  - Updated test helpers and example files with correct domain references

## [0.5.4] – 2025-05-31

### Added

- **Complete Data Binding Library**: Comprehensive template + data binding
  system for flexible dashboard development
  - Built `BBDataBinder` JavaScript library with `data-bb-bind` attribute
    processing for dynamic content
  - Implemented template engine with `data-bb-template` for repeated elements
    (job lists, tables, etc.)
  - Added authentication integration with `data-bb-auth` for conditional element
    display
  - Created comprehensive form handling with `data-bb-form` attributes and
    real-time validation
  - Built style and attribute binding with `data-bb-bind-style` and
    `data-bb-bind-attr` for dynamic CSS and attributes
- **Enhanced Form Processing**: Production-ready form handling with validation
  and error management
  - Real-time field validation with `data-bb-validate` attributes and custom
    validation rules
  - Automatic form submission to API endpoints with authentication token
    handling
  - Loading states, success/error messaging, and form reset capabilities
  - Support for job creation, profile updates, and custom forms with
    configurable endpoints
- **Example Templates**: Complete working examples demonstrating all data
  binding features
  - `data-binding-example.html` - Full demonstration of template binding with
    mock data
  - `form-example.html` - Comprehensive form handling examples with validation
  - `dashboard-enhanced.html` - Production-ready dashboard using data binding
    library

### Enhanced

- **Build System**: Updated Rollup configuration to build data binding library
  alongside Web Components
  - Added `bb-data-binder.js` and `bb-data-binder.min.js` builds for production
    deployment
  - Library available at `/js/bb-data-binder.min.js` endpoint for CDN-style
    usage
  - Zero runtime dependencies - works with vanilla JavaScript and Supabase

### Technical Implementation

- **Data Binding Architecture**: Template-driven approach where HTML controls
  layout and JavaScript provides functionality
  - DOM scanning system finds and registers elements with data binding
    attributes
  - Efficient element updates with path-based data mapping and template caching
  - Event delegation for `bb-action` attributes combined with data binding for
    complete template system
- **Authentication Integration**: Seamless Supabase Auth integration with
  conditional rendering
  - Elements with `data-bb-auth="required"` only show when authenticated
  - Elements with `data-bb-auth="guest"` only show when not authenticated
  - Automatic auth state monitoring and element visibility updates
- **Form Processing Pipeline**: Complete form lifecycle management from
  validation to submission
  - Client-side validation with multiple rule types (required, email, URL,
    length, pattern)
  - API endpoint determination based on form action with automatic
    authentication headers
  - Success/error handling with custom events and configurable redirects

## [0.5.3] – 2025-05-31

### Changed

- **Dashboard Architecture**: Replaced Web Components with vanilla JavaScript +
  attribute-based event handling
  - Removed Web Components dependencies (`bb-auth-login`, `bb-job-dashboard`)
    from dashboard
  - Implemented vanilla JavaScript with modern styling for better reliability
    and maintainability
  - Added attribute-based event system: elements with `bb-action` attributes
    automatically handle functionality
  - Replaced `onclick` handlers with `bb-action="refresh-dashboard"`,
    `bb-action="create-job"` pattern
  - Maintained modern UI design whilst switching to proven vanilla JavaScript
    approach

### Enhanced

- **Template + Data Binding Foundation**: Established framework for flexible
  dashboard development
  - Dashboard now demonstrates template approach where HTML layout is
    customisable
  - JavaScript automatically scans for `bb-action` and `bb-data-*` attributes to
    provide functionality
  - Event delegation system allows any HTML element with `bb-action` to trigger
    Blue Banded Bee features
  - Sets foundation for future template binding system where users control
    layout design

### Fixed

- **Production Dashboard Stability**: Resolved Web Components authentication and
  loading issues
  - Dashboard now uses proven vanilla JavaScript patterns instead of
    experimental Web Components
  - Removed complex component lifecycle management in favour of direct API
    integration
  - Eliminated dependency on Web Components build pipeline for core dashboard
    functionality

### Technical Details

- Consolidated `dashboard-new.html` and `dashboard.html` into single vanilla
  JavaScript implementation
- Added `setupAttributeHandlers()` function with event delegation for
  `bb-action` attributes
- Maintained API integration with `/v1/dashboard/stats` and `/v1/jobs` endpoints
- Preserved modern grid layout and responsive design from Web Components version

## [0.5.2] – 2025-05-31

### Fixed

- **Authentication Component OAuth Redirect**: Resolved OAuth login redirecting
  to dashboard on test pages
  - Fixed auth state change listener to only redirect when `redirect-url`
    attribute is explicitly set
  - Simplified redirect logic - components without `redirect-url` stay on
    current page after login
  - Removed complex `test-mode` attribute approach in favour of intuitive
    behaviour
  - OAuth flows (Google, GitHub, Slack) now complete on test pages without
    unwanted redirects

### Enhanced

- **Component Design Philosophy**: Streamlined authentication component
  behaviour
  - Test pages: `<bb-auth-login>` (no redirect-url) = No redirect, works in both
    logged-in/out states
  - Production pages: `<bb-auth-login redirect-url="/dashboard">` = Redirects
    after successful login
  - Cleaner, more predictable component behaviour without special testing
    attributes

### Technical Details

- Auth state change listener now checks for `redirect-url` attribute before
  triggering redirects
- Removed `test-mode` from observed attributes and related logic
- Web Components rebuilt and deployed with simplified redirect handling
- Both initial load check and OAuth completion follow same redirect-url logic

## [0.5.1] – 2025-05-31

### Added

- **Dashboard Route**: Added `/dashboard` endpoint to resolve OAuth redirect 404
  errors
  - Created dashboard page handler in Go API to serve `dashboard.html`
  - Updated Dockerfile to include dashboard.html in container deployment
  - Fixed authentication component redirect behaviour to prevent 404 errors
    after successful login

### Enhanced

- **Web Components Testing Infrastructure**: Comprehensive test page
  improvements
  - Added `test-mode` attribute to `bb-auth-login` component to prevent
    automatic redirects during testing
  - Created logout functionality for testing different authentication states
  - Enhanced test page with authentication status display and manual controls
  - Fixed redirect issues that prevented proper component testing

### Fixed

- **Authentication Component Redirect Logic**: Resolved automatic redirect
  problems
  - Modified `bb-auth-login` component to respect `test-mode="true"` attribute
  - Updated redirect logic to properly handle empty redirect URLs
  - Fixed issue where authenticated users were immediately redirected away from
    test pages

### Documentation

- **Supabase Integration Strategy**: Updated architecture documentation with
  platform integration recommendations
  - Added comprehensive Supabase feature mapping to development roadmap stages
  - Enhanced Architecture.md with real-time features, database functions, and
    Edge Functions strategy
  - Updated Roadmap.md to incorporate Supabase capabilities across Stage 5
    (Performance & Scaling) and Stage 6 (Multi-tenant & Teams)
- **Development Workflow**: Enhanced CLAUDE.md with comprehensive working style
  guidance
  - Added communication preferences, git workflow, and tech stack leverage
    guidelines
  - Documented build process awareness, testing strategy, and configuration
    management practices
  - Created clear guidance for future AI sessions to work more productively

### Technical Details

- Dashboard route serves existing dashboard.html with corrected Supabase
  credentials
- Test mode in authentication component prevents both initial redirect checks
  and post-login redirects
- Web Components require rebuild (`npm run build`) when source files are
  modified
- Git workflow updated to commit freely but only push when ready for production
  testing

## [0.5.0] – 2025-05-30

### Added

- **Web Components MVP Interface**: Complete frontend infrastructure for Webflow
  integration
  - Built vanilla Web Components architecture using template + data slots
    pattern (industry best practice)
  - Created `bb-data-loader` core component for API data fetching and Webflow
    template population
  - Implemented `bb-auth-login` component with full Supabase authentication and
    social providers
  - Added `BBBaseComponent` base class with loading/error states, data binding,
    and event handling
- **Production Build System**: Rollup-based build pipeline for component
  distribution
  - Zero runtime dependencies (vanilla JavaScript, Supabase via CDN)
  - Minified production bundle (`bb-components.min.js`) ready for CDN deployment
  - Development and production builds with source maps and error handling
- **Static File Serving**: Integrated component serving into existing Go
  application
  - Added `/js/` endpoint to serve Web Components as static files from Go app
  - Components now accessible at
    `https://app.bluebandedbee.co/js/bb-components.min.js`
  - Docker container properly configured to include built components

### Enhanced

- **Webflow Integration Strategy**: Clarified multi-interface architecture and
  user journeys
  - **BBB Main Website**: Primary dashboard built on Webflow with embedded Web
    Components
  - **Webflow Designer Extension**: Lightweight progress modals within Webflow
    Designer
  - **Slack Integration**: Threaded conversations with links to main BBB site
  - Updated documentation to reflect three distinct user journey patterns
- **Component Architecture**: Template-driven approach for maximum Webflow
  compatibility
  - Data binding with `data-bind` attributes for text content population
  - Style binding with `data-style-bind` for dynamic CSS properties (progress
    bars, etc.)
  - Event handling for user interactions (view details, cancel jobs, form
    submissions)
  - Real-time updates with configurable refresh intervals and WebSocket support

### Technical Implementation

- **API Integration**: Seamless connection to existing `/v1/*` RESTful endpoints
  - Authentication via JWT tokens from Supabase Auth
  - Error handling with structured API responses and user-friendly error
    messages
  - Rate limiting and CORS support for cross-origin requests
- **Development Workflow**: Streamlined build and deployment process
  - Source files in `/web/src/` with modular component structure
  - Build process: `npm run build` → commit built files → Fly deployment
  - No CDN required initially - components served from existing infrastructure

### Documentation

- **Complete Integration Examples**: Production-ready code examples for Webflow
  - `webflow-integration.html` - Copy-paste example for Webflow pages
  - `complete-example.html` - Full-featured demo with all component features
  - Comprehensive README with step-by-step Webflow integration instructions
- **Architecture Documentation**: Updated UI implementation plan with clarified
  user journeys
  - Documented template + data slots pattern and Web Components best practices
  - Clear separation between BBB main site, Designer Extension, and Slack
    integration
  - Technical justification for vanilla Web Components over framework
    alternatives

### Infrastructure

- **Deployment Ready**: Production infrastructure complete for Stage 4 MVP
  - Components automatically built and deployed with existing Fly.io workflow
  - Static file serving integrated into Go application without additional
    services
  - Backward compatible - no changes to existing API or authentication systems

## [0.4.3] – 2025-05-30

### Added

- **Complete Sentry Integration**: Comprehensive error tracking and performance
  monitoring
  - Properly initialised Sentry SDK in main.go with environment-aware
    configuration
  - Added error capture (`sentry.CaptureException()`) for critical business
    logic failures
  - Strategic error monitoring in job management, worker operations, and
    database transactions
  - Performance span tracking already operational: job operations, database
    operations, sitemap processing
  - Configured 10% trace sampling in production, 100% in development for optimal
    observability
- **Comprehensive Documentation Consolidation**: Streamlined from 31 to 10
  documentation files
  - Created unified `ARCHITECTURE.md` combining system design, technical
    concepts, and component details
  - Consolidated `DEVELOPMENT.md` merging setup, testing, debugging, and
    contribution guidelines
  - Cleaned up `API.md` with consistent endpoint references and comprehensive
    documentation
  - Created `DATABASE.md` covering PostgreSQL schema, queries, operations, and
    performance optimisation
  - Consolidated future plans into 3 actionable documents: UI implementation,
    Webflow integration, scaling strategy

### Changed

- **Documentation Structure**: Complete reorganisation for maintainability and
  clarity
  - Eliminated content overlap between architecture files (mental-model,
    implementation-details, jobs)
  - Fixed content inconsistencies: corrected project stage references, removed
    deprecated depth column mentions
  - Updated README.md with accurate Stage 4 status and enhanced documentation
    index with descriptions
  - Improved CLAUDE.md with updated code organisation reflecting current package
    structure
- **Error Monitoring Strategy**: Strategic approach to avoid over-logging while
  capturing critical issues
  - Focus on infrastructure failures, data consistency issues, and critical
    business operations
  - Avoided granular task-level logging while maintaining comprehensive system
    health monitoring
  - Integration with existing performance spans for complete observability

### Removed

- **Redundant Documentation**: Eliminated 21 redundant files and outdated
  content
  - Removed overlapping architecture files: mental-model.md,
    implementation-details.md, jobs.md
  - Consolidated reference files: codebase-structure.md, file-map.md,
    auth-integration.md, database-config.md
  - Cleaned up outdated plans: 8 completed or irrelevant planning documents
  - Removed CONTRIBUTING.md (merged into DEVELOPMENT.md) and duplicate guide
    content

### Fixed

- **Documentation Accuracy**: Corrected stale and inconsistent information
  throughout
  - Fixed project stage references (Stage 3 → Stage 4) in README.md
  - Removed deprecated depth column references from database documentation
  - Updated API endpoint paths to match current `/v1/*` structure
  - Corrected outdated technology references (SQLite → PostgreSQL)

## [0.4.2] – 2025-05-29

### Added

- **RESTful API Architecture**: Complete API infrastructure overhaul with modern
  standards
  - Implemented standardised error handling with request IDs and consistent HTTP
    status codes
  - Created comprehensive middleware stack: CORS, request ID tracking,
    structured logging, rate limiting
  - Built RESTful endpoint structure under `/v1/*` namespace for versioned API
    access
  - Added proper authentication middleware integration with Supabase JWT
    validation
- **API Response Standardisation**: Consistent response formats across all
  endpoints
  - Success responses include `status`, `data`, `message`, and `request_id`
    fields
  - Error responses provide structured error information with HTTP status codes
    and error codes
  - Request ID tracking for distributed tracing and debugging support
- **Enhanced Security**: Improved security posture with secured admin endpoints
  - Moved debug endpoints to `/admin/*` namespace with environment variable
    protection
  - Added CORS middleware for secure cross-origin requests
  - Implemented rate limiting with proper IP detection and standardised error
    responses

### Changed

- **API Endpoint Structure**: Migrated from ad-hoc endpoints to RESTful design
  - Job creation: `POST /v1/jobs` with JSON body instead of query parameters
  - Job status: `GET /v1/jobs/:id` following RESTful conventions
  - Authentication: Consolidated under `/v1/auth/*` namespace
  - Health checks: Standardised `/health` and `/health/db` endpoints
- **Error Handling**: Replaced inconsistent `http.Error()` calls with structured
  error responses
  - All errors now include request IDs for tracing
  - Consistent error codes: `BAD_REQUEST`, `UNAUTHORISED`, `NOT_FOUND`, etc.
  - Proper HTTP status code usage throughout the API
- **Code Organisation**: Refactored API logic into dedicated `internal/api`
  package
  - Separated concerns: handlers, middleware, errors, responses, authentication
  - Dependency injection pattern with clean handler structure
  - Eliminated duplicate endpoint logic and inconsistent patterns

### Removed

- **Legacy Endpoints**: Removed unused endpoints since APIs are not yet
  published
  - Removed `/site` and `/job-status` legacy endpoints and their handlers
  - Cleaned up duplicate code paths and unused imports
  - Simplified codebase by removing backward compatibility code

### Enhanced

- **Testing Infrastructure**: Created comprehensive API testing tools
  - Updated test login page to use new `/v1/*` endpoints
  - Added `api-tests.http` file for VS Code REST Client testing
  - Created detailed API testing guide with authentication examples
- **Documentation**: Updated API reference and implementation documentation
  - Comprehensive API testing guide with practical examples
  - Updated roadmap to reflect completed API infrastructure work
  - Enhanced code documentation with clear separation of concerns

### Technical Details

- New `internal/api` package structure with clean separation of handlers,
  middleware, and utilities
- Middleware stack processes requests in proper order: CORS → Request ID →
  Logging → Rate Limiting
- JWT authentication middleware integrates seamlessly with Supabase token
  validation
- Request ID generation uses timestamp + random bytes for unique request
  tracking
- Error responses provide consistent structure while maintaining security (no
  information leakage)

## [0.4.1] – 2025-05-27

### Fixed

- **Database Schema Issues**: Resolved critical production database errors
  - Added missing `error_message` column to jobs table to prevent database
    insertion failures
  - Fixed duplicate user creation constraint violations with idempotent user
    registration
  - `CreateUserWithOrganisation` now handles existing users gracefully instead
    of failing
- **User Registration Flow**: Enhanced authentication reliability
  - Multiple login attempts with same user ID no longer cause database
    constraint violations
  - Existing users are returned with their organisations rather than attempting
    duplicate creation
  - Improved error handling and logging for user creation scenarios

### Enhanced

- **Development Workflow**: Added git policy documentation to prevent accidental
  commits
- **Project Planning**: Added multi-provider account linking testing to roadmap
  for future investigation

### Technical Details

- Database migration adds `error_message TEXT` column to jobs table with
  `ALTER TABLE IF NOT EXISTS`
- User creation now checks for existing users before attempting INSERT
  operations
- Transaction rollback properly handles failed user creation attempts
- All database fixes are backward compatible with existing installations

## [0.4.0] – 2025-05-27

### Added

- **Complete Supabase Authentication System**: Full multi-tenant authentication
  with social login support
  - JWT validation middleware with structured error handling and token
    validation
  - Support for 8 social login providers: Google, Facebook, Slack, GitHub,
    Microsoft, Figma, LinkedIn + Email/Password
  - Custom domain authentication using `auth.bluebandedbee.co` for professional
    OAuth flows
  - User and organisation management with automatic organisation creation on
    signup
  - Row Level Security (RLS) policies for secure multi-tenant data access
- **Protected API Endpoints**: All job creation and user data endpoints now
  require authentication
  - `/site` endpoint (job creation) now requires valid JWT token and links jobs
    to users/organisations
  - `/job-status` endpoint protected with organisation-scoped access control
  - `/api/auth/profile` endpoint for authenticated user profile access
  - User registration API with automatic organisation linking
- **Database Schema Extensions**: Enhanced schema to support multi-tenant
  architecture
  - Added `users` and `organisations` tables with foreign key relationships
  - Added `user_id` and `organisation_id` columns to `jobs` table
  - Implemented Row Level Security on all user-related tables
  - Database migration logic for existing installations

### Enhanced

- **Authentication Flow**: Complete OAuth integration with account linking
  support
  - Flexible email-based account linking with UUID-based permanent user identity
  - Session management with token expiry detection and refresh warnings
  - Structured error responses for authentication failures
  - Support for multiple auth providers per user account
- **Multi-tenant Job Management**: Jobs are now scoped to organisations with
  shared access
  - All organisation members can view and manage all jobs within their
    organisation
  - Jobs automatically linked to creator's user ID and organisation ID
  - Database queries respect organisation boundaries through RLS policies

### Security

- **Comprehensive Authentication Security**: Production-ready security features
  - JWT token validation with proper error handling and logging
  - Authentication service configuration validation
  - Standardised error responses that don't leak sensitive information
  - Row Level Security policies prevent cross-organisation data access
- **Protected Endpoints**: All sensitive operations require valid authentication
  - Job creation requires authentication and organisation membership
  - Job status queries limited to organisation members
  - User profile access restricted to authenticated user's own data

### Technical Details

- Custom domain setup eliminates unprofessional Supabase URLs in OAuth flows
- Database migration handles existing installations with
  `ALTER TABLE IF NOT EXISTS`
- JWT middleware supports both required and optional authentication scenarios
- Account linking strategy preserves user choice while preventing duplicate
  accounts
- All authentication endpoints follow RESTful conventions with proper HTTP
  status codes

## [0.3.11] – 2025-05-26

### Added

- **MaxPages Functionality**: Implemented page limit controls for jobs
  - `max` query parameter now limits number of pages processed per job
  - Tasks beyond limit automatically set to 'skipped' status during creation
  - Added `skipped_tasks` column to jobs table and Job struct
  - Progress calculation excludes skipped tasks:
    `(completed + failed) / (total - skipped) * 100`
  - API responses include skipped count for full visibility

### Enhanced

- **Smart Task Status Management**: Tasks receive appropriate status at creation
  time
  - First N tasks (up to max_pages) get 'pending' status
  - Remaining tasks automatically get 'skipped' status
  - Eliminates need for post-creation status updates
- **Database Triggers**: Updated progress calculation triggers to handle skipped
  tasks
  - Automatic counting of completed, failed, and skipped tasks
  - Progress percentage calculation excludes skipped tasks from denominator
  - Job completion logic updated to account for skipped tasks

### Changed

- **Link Discovery Default**: Changed default behaviour to enable link discovery
  by default
  - `find_links` now defaults to `true` (was previously `false`)
  - Use `find_links=false` to disable link discovery and only crawl sitemap URLs
  - More intuitive API behaviour for comprehensive cache warming

### Fixed

- **Job Completion Logic**: Fixed job completion detection for jobs with limits
  - Updated completion checker: `(completed + failed) >= (total - skipped)`
  - Added safety check for division by zero in progress calculations
  - Prevents jobs from being stuck with remaining skipped tasks

### Technical Details

- MaxPages limit of 0 means unlimited processing (default behaviour)
- Task status determined during `EnqueueURLs` based on current task count vs
  max_pages
- Database schema migration adds `skipped_tasks INTEGER DEFAULT 0` column
- Backward compatible with existing jobs (skipped_tasks defaults to 0)

## [0.3.10] – 2025-05-26

### Added

- **Database-Driven Architecture**: Moved critical business logic to PostgreSQL
  triggers for improved reliability
  - Automatic job progress calculation (`progress`, `completed_tasks`,
    `failed_tasks`) via database triggers
  - Auto-generated timestamps (`started_at`, `completed_at`) based on task
    completion status
  - Eliminates race conditions and ensures data consistency across concurrent
    workers
- **Enhanced Dashboard UX**: Comprehensive date range and filtering improvements
  - Smart date range presets: Today, Last 24 Hours, Yesterday, Last 7/28/90
    Days, All Time, Custom
  - Automatic timezone conversion from UTC database timestamps to user's local
    timezone
  - Complete time series charts with all increments (shows empty periods for
    accurate visualisation)
  - Dynamic group-by selection that auto-updates based on date range scope

### Fixed

- **Timezone Consistency**: Resolved incorrect timestamp display in dashboard
  - Standardised all database timestamps to use UTC (`time.Now().UTC()` in Go,
    `NOW()` in PostgreSQL)
  - Fixed dashboard date formatting to properly convert UTC to user's local
    timezone
  - Corrected date picker logic to handle precise timestamp filtering instead of
    date-only ranges
- **Dashboard Data Access**: Fixed Row Level Security (RLS) blocking dashboard
  queries
  - Added anonymous read policies for `domains`, `pages`, `jobs`, and `tasks`
    tables
  - Enables dashboard functionality while maintaining security framework for
    future auth

### Enhanced

- **Simplified Go Code**: Removed complex manual progress calculation logic
  - `UpdateJobProgress()` function now handled entirely by database triggers
  - Eliminated manual timestamp management in job start/completion workflows
  - Reduced code complexity while improving reliability through
    database-enforced consistency
- **Chart Visualisation**: Improved dashboard charts with complete time coverage
  - Charts now display all time increments for selected range (e.g., all 24
    hours for "Today")
  - Fixed grouping logic to automatically select appropriate time granularity
  - Enhanced debugging output for troubleshooting data visualisation issues

### Technical Details

- Database triggers automatically fire on task status changes (`INSERT`,
  `UPDATE`, `DELETE` on tasks table)
- Progress calculation uses PostgreSQL aggregate functions for atomic updates
- Timezone handling leverages JavaScript's native `Intl.DateTimeFormat()` for
  accurate local conversion
- Chart time series generation creates complete axis labels even for periods
  with zero activity

## [0.3.9] – 2025-05-25

### Added

- **Startup Recovery System**: Automatic recovery for jobs interrupted by server
  restarts
  - Jobs with 'running' status and 'running' tasks are automatically detected on
    startup
  - Tasks are reset from 'running' to 'pending' and jobs are added back to
    worker pool
  - Eliminates need for manual intervention when jobs are stuck after restarts
- **Smart Link Filtering**: Enhanced crawler to extract only visible,
  user-clickable links
  - Filters out hidden elements (display:none, visibility:hidden,
    screen-reader-only)
  - Skips non-navigation links (javascript:, mailto:, empty hrefs)
  - Rejects links without visible text content (unless they have aria-labels)
  - Prevents extraction of framework-generated or accessibility-only links
- **Live Dashboard**: Real-time job monitoring dashboard with Supabase
  integration
  - Auto-refresh every 10 seconds with date range filtering
  - Smart time grouping (minute/hour/6-hour/day based on selected range)
  - Bar charts showing task completion over time with local timezone support
  - Comprehensive debugging and fallback displays for data access issues

### Fixed

- **Domain Filtering**: Improved same-domain detection to handle www prefix
  variations
  - `www.test.com` and `test.com` now correctly recognised as same domain
  - Enhanced subdomain detection works with both normalised and original domains
  - Prevents false rejection of internal links due to www prefix mismatches
- **External Link Rejection**: Strict filtering to prevent crawling external
  domains
  - All external domain links are now properly rejected with detailed logging
  - Eliminates failed crawls from external links being treated as relative URLs
  - Maintains focus on target domain while preventing scope creep
- **Database Reset**: Enhanced schema reset to handle views and dependencies
  - Properly drops views (job_list, job_dashboard) before dropping tables
  - Uses CASCADE to handle remaining dependencies automatically
  - Added comprehensive error logging and sequence cleanup

### Enhanced

- **Database Connection Resilience**: Improved connection pool settings and
  retry logic
  - Updated connection pool: MaxOpenConns (25→35), MaxIdleConns (10→15),
    MaxLifetime (5min→30min)
  - Added automatic retry logic with exponential backoff for transient
    connection failures
  - Enhanced error detection for connection-related issues (bad connection,
    unexpected parse, etc.)
- **Worker Recovery**: Enhanced task monitoring and job completion detection
  - Improved cleanup of stuck jobs where all tasks are complete but job status
    is still running
  - Better handling of stale task recovery with proper timeout detection
  - Enhanced logging throughout the recovery and monitoring processes
- **URL Normalisation**: Advanced link processing to eliminate duplicate pages
  - Automatic anchor fragment stripping (`/page#section1` → `/page`)
  - Trailing slash normalisation (`/events-news/` → `/events-news`)
  - Ensures consistent URL handling and prevents duplicate crawling of identical
    pages

### Technical Details

- Dashboard uses date-only pickers with proper timezone handling for accurate
  time grouping
- Link filtering integrates with Colly's HTML element processing for efficient
  visibility detection
- Domain comparison uses normalised hostname matching with comprehensive
  subdomain support
- Database retry logic specifically targets PostgreSQL connection issues with
  appropriate backoff strategies

## [0.3.8] – 2025-05-25

### Fixed

- **Critical Production Fix**: Resolved database schema mismatch causing task
  insertion failures
  - Fixed INSERT statement parameter count mismatch in `/internal/db/queue.go`
  - Corrected VALUES clause to match 9 fields with 9 placeholders (`$1` through
    `$9`)
  - Eliminated
    `pq: null value in column "depth" of relation "tasks" violates not-null constraint`
    error
- Fixed compilation issues in test utilities:
  - Updated `cmd/test_jobs/main.go` to use correct function signatures for
    `NewWorkerPool` and `NewJobManager`
  - Added proper `dbQueue` parameter initialisation following production code
    patterns

### Technical Details

- The production database retained the deprecated `depth` column from v0.3.6,
  but the code was updated in v0.3.7 to remove depth functionality
- Database schema reset was required to align production database with current
  code expectations
- Task queue now successfully processes jobs without depth-related constraint
  violations

## [0.3.7] – 2025-05-18

### Removed

- Removed depth functionality from the codebase:
  - Removed depth column from tasks table schema
  - Removed depth parameter from all EnqueueURLs functions
  - Updated code to not use depth in task processing
  - Modified database queries to exclude the depth field
  - Simplified code by removing unused functionality

## [0.3.6] – 2025-05-18

### Fixed

- Fixed job counter updates for `sitemap_tasks` and `found_tasks` columns:
  - Added missing functionality to update sitemap counter when sitemap URLs are
    processed
  - Implemented incrementing of found task counter for URLs discovered during
    crawling
  - Fixed duplicate processing issue by moving the page processing mark after
    successful task creation
  - Updated job query to properly return counter values
- Improved task creation reliability by ensuring pages are only marked as
  processed after successful DB operations

## [0.3.5] – 2025-05-17

### Changed

- Major code refactoring to improve architecture and maintainability:
  - Eliminated duplicate code across the codebase
  - Removed global state in favor of proper dependency injection
  - Standardised function naming conventions
  - Clarified responsibilities between packages
  - Moved database operations to a unified interface
  - Improved transaction management with DbQueue

### Removed

- Removed redundant files and functions:
  - Eliminated `jobs/db.go` (moved functions to other packages)
  - Removed `jobs/queue_helpers.go` (consolidated functionality)
  - Removed global state management with `SetDBInstance`
  - Eliminated duplicate SQL operations

## [0.3.4] – 2025-05-17

### Added

- Enhanced sitemap crawling with improved URL handling
- Added URL normalisation in sitemap processing
- Implemented robust error handling for URL processing
- Added better detection and correction of malformed URLs

### Fixed

- Fixed sitemap URL discovery and processing issues
- Improved relative URL handling in crawler
- Resolved issues with URL encoding/decoding in sitemap parser
- Fixed task queue URL processing in worker pool

### Changed

- Enhanced worker pool to better handle URL variations
- Updated job manager to properly normalise URLs before processing
- Improved URL validation logic in task processing

## [0.3.3] – 2025-04-22

### Added

- Added `sitemap_tasks` and `found_tasks` columns to the `jobs` table and
  corresponding fields in the Job struct
- Enqueued discovered links (same-domain pages and document URLs) via link
  extraction in the worker pool

### Changed

- `processTask` now filters `result.Links` to include only same-site pages and
  docs (`.pdf`, `.doc`, `.docx`) and enqueues them
- Updated `setupSchema` to include new columns with `ALTER TABLE IF NOT EXISTS`
- Exposed `Crawler.Config()` method to allow workers to read the `FindLinks`
  flag

### Documentation

- Updated `docs/architecture/jobs.md` to document new task counters and
  link-extraction behaviour

## [0.3.2] ��� 2025-04-21

### Changed

- Improved database configuration management with validation for required fields
- Enhanced worker pool notification system with more robust connection handling
- Simplified notification handling in worker pool with better error recovery
- Fixed linting issues in worker pool implementation

## [0.3.1] – 2025-04-21

### Changed

- Fixed documentation file references in INIT.md and README.md to use explicit
  relative paths
- Updated ROADMAP.md references to point to root directory instead of docs/
- Ensured consistent file linking across documentation

## [0.3.0] – 2025-04-20

### Added

- New `domains` and `pages` reference tables for improved data integrity
- Helper methods for domain and page management
- Added depth control for crawling with per-task depth configuration

### Removed

- Removed legacy `crawl_results` table and associated code.
- Removed unused functions and methods to improve code maintainability
- Eliminated deprecated code including outdated `rand.Seed` usage

### Changed

- Restructured documentation under `docs/` directory.
- Added limit to site crawl to control no of pages to crawl.
- Modified `jobs` table to reference domains by ID instead of storing domain
  names directly
- Updated `tasks` table to use page references instead of storing full URLs
- Refactored URL handling throughout the codebase to work with the new reference
  system

### Fixed

- Correctly set job and task completion timestamps (`CompletedAt`) when tasks
  and jobs complete.
- Fixed "append result never used" warnings in database operations
- Resolved unused import warnings and other code quality issues
- Fixed SQL parameter placeholders to use PostgreSQL-style numbered parameters
  (`$1`, `$2`, etc.) instead of MySQL/SQLite-style (`?`)
- Fixed task processing issues after database reset by ensuring consistent
  parameter style in all SQL queries
- Corrected parameter count mismatch in batch insert operations

## [0.2.0] - 2025-04-20

### Changed

- **Major Database Migration**: Fully migrated from SQLite/Turso to PostgreSQL
  - Removed all SQLite dependencies including
    `github.com/tursodatabase/libsql-client-go`
  - Reorganised database code structure, moving from `internal/db/postgres` to
    `internal/db`
  - Updated all application code to use PostgreSQL exclusively
  - Fixed all database-related tests

### Fixed

- Fixed crawler's `WarmURL` method to properly handle HTTP responses, context
  cancellation, and timeouts
- Resolved undefined functions and variables in test files related to the
  PostgreSQL task queue
- Implemented rate limiting functionality in the app server
- Updated all tests to work with the PostgreSQL backend
- Ensured all tests pass successfully after modifications

### Technical Debt

- Removed duplicated code from the SQLite implementation
- Cleaned up directory structure to better reflect current architecture

## [0.1.0] - 2025-04-15

### Added

- Initial project setup
- Basic crawler implementation with Colly
- Job queue system for managing crawl tasks
- Web API for submitting and monitoring crawl jobs
- SQLite database integration with Turso

### Technical Details

- Go modules for dependency management
- Internal package structure with clean separation of concerns
- Test suite for crawler and database operations
- Basic rate limiting and error handling
