# Changelog

All notable changes to the Blue Banded Bee project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Multiple version updates may occur on the same date, each with its own version number.
Each version represents a distinct set of changes, even if released on the same day.

## [0.3.11] – 2025-05-26

### Added
- **MaxPages Functionality**: Implemented page limit controls for jobs
  - `max` query parameter now limits number of pages processed per job
  - Tasks beyond limit automatically set to 'skipped' status during creation
  - Added `skipped_tasks` column to jobs table and Job struct
  - Progress calculation excludes skipped tasks: `(completed + failed) / (total - skipped) * 100`
  - API responses include skipped count for full visibility

### Enhanced
- **Smart Task Status Management**: Tasks receive appropriate status at creation time
  - First N tasks (up to max_pages) get 'pending' status
  - Remaining tasks automatically get 'skipped' status  
  - Eliminates need for post-creation status updates
- **Database Triggers**: Updated progress calculation triggers to handle skipped tasks
  - Automatic counting of completed, failed, and skipped tasks
  - Progress percentage calculation excludes skipped tasks from denominator
  - Job completion logic updated to account for skipped tasks

### Fixed
- **Job Completion Logic**: Fixed job completion detection for jobs with limits
  - Updated completion checker: `(completed + failed) >= (total - skipped)`
  - Added safety check for division by zero in progress calculations
  - Prevents jobs from being stuck with remaining skipped tasks

### Technical Details
- MaxPages limit of 0 means unlimited processing (default behaviour)
- Task status determined during `EnqueueURLs` based on current task count vs max_pages
- Database schema migration adds `skipped_tasks INTEGER DEFAULT 0` column
- Backward compatible with existing jobs (skipped_tasks defaults to 0)

## [0.3.10] – 2025-05-26

### Added
- **Database-Driven Architecture**: Moved critical business logic to PostgreSQL triggers for improved reliability
  - Automatic job progress calculation (`progress`, `completed_tasks`, `failed_tasks`) via database triggers
  - Auto-generated timestamps (`started_at`, `completed_at`) based on task completion status
  - Eliminates race conditions and ensures data consistency across concurrent workers
- **Enhanced Dashboard UX**: Comprehensive date range and filtering improvements
  - Smart date range presets: Today, Last 24 Hours, Yesterday, Last 7/28/90 Days, All Time, Custom
  - Automatic timezone conversion from UTC database timestamps to user's local timezone
  - Complete time series charts with all increments (shows empty periods for accurate visualization)
  - Dynamic group-by selection that auto-updates based on date range scope

### Fixed
- **Timezone Consistency**: Resolved incorrect timestamp display in dashboard
  - Standardized all database timestamps to use UTC (`time.Now().UTC()` in Go, `NOW()` in PostgreSQL)
  - Fixed dashboard date formatting to properly convert UTC to user's local timezone
  - Corrected date picker logic to handle precise timestamp filtering instead of date-only ranges
- **Dashboard Data Access**: Fixed Row Level Security (RLS) blocking dashboard queries
  - Added anonymous read policies for `domains`, `pages`, `jobs`, and `tasks` tables
  - Enables dashboard functionality while maintaining security framework for future auth

### Enhanced
- **Simplified Go Code**: Removed complex manual progress calculation logic
  - `UpdateJobProgress()` function now handled entirely by database triggers
  - Eliminated manual timestamp management in job start/completion workflows
  - Reduced code complexity while improving reliability through database-enforced consistency
- **Chart Visualization**: Improved dashboard charts with complete time coverage
  - Charts now display all time increments for selected range (e.g., all 24 hours for "Today")
  - Fixed grouping logic to automatically select appropriate time granularity
  - Enhanced debugging output for troubleshooting data visualization issues

### Technical Details
- Database triggers automatically fire on task status changes (`INSERT`, `UPDATE`, `DELETE` on tasks table)
- Progress calculation uses PostgreSQL aggregate functions for atomic updates
- Timezone handling leverages JavaScript's native `Intl.DateTimeFormat()` for accurate local conversion
- Chart time series generation creates complete axis labels even for periods with zero activity

## [0.3.9] – 2025-05-25

### Added
- **Startup Recovery System**: Automatic recovery for jobs interrupted by server restarts
  - Jobs with 'running' status and 'running' tasks are automatically detected on startup
  - Tasks are reset from 'running' to 'pending' and jobs are added back to worker pool
  - Eliminates need for manual intervention when jobs are stuck after restarts
- **Smart Link Filtering**: Enhanced crawler to extract only visible, user-clickable links
  - Filters out hidden elements (display:none, visibility:hidden, screen-reader-only)
  - Skips non-navigation links (javascript:, mailto:, empty hrefs)
  - Rejects links without visible text content (unless they have aria-labels)
  - Prevents extraction of framework-generated or accessibility-only links
- **Live Dashboard**: Real-time job monitoring dashboard with Supabase integration
  - Auto-refresh every 10 seconds with date range filtering
  - Smart time grouping (minute/hour/6-hour/day based on selected range)
  - Bar charts showing task completion over time with local timezone support
  - Comprehensive debugging and fallback displays for data access issues

### Fixed
- **Domain Filtering**: Improved same-domain detection to handle www prefix variations
  - `www.test.com` and `test.com` now correctly recognized as same domain
  - Enhanced subdomain detection works with both normalized and original domains
  - Prevents false rejection of internal links due to www prefix mismatches
- **External Link Rejection**: Strict filtering to prevent crawling external domains
  - All external domain links are now properly rejected with detailed logging
  - Eliminates failed crawls from external links being treated as relative URLs
  - Maintains focus on target domain while preventing scope creep
- **Database Reset**: Enhanced schema reset to handle views and dependencies
  - Properly drops views (job_list, job_dashboard) before dropping tables
  - Uses CASCADE to handle remaining dependencies automatically
  - Added comprehensive error logging and sequence cleanup

### Enhanced
- **Database Connection Resilience**: Improved connection pool settings and retry logic
  - Updated connection pool: MaxOpenConns (25→35), MaxIdleConns (10→15), MaxLifetime (5min→30min)
  - Added automatic retry logic with exponential backoff for transient connection failures
  - Enhanced error detection for connection-related issues (bad connection, unexpected parse, etc.)
- **Worker Recovery**: Enhanced task monitoring and job completion detection
  - Improved cleanup of stuck jobs where all tasks are complete but job status is still running
  - Better handling of stale task recovery with proper timeout detection
  - Enhanced logging throughout the recovery and monitoring processes
- **URL Normalisation**: Advanced link processing to eliminate duplicate pages
  - Automatic anchor fragment stripping (`/page#section1` → `/page`)
  - Trailing slash normalisation (`/events-news/` → `/events-news`)
  - Ensures consistent URL handling and prevents duplicate crawling of identical pages

### Technical Details
- Dashboard uses date-only pickers with proper timezone handling for accurate time grouping
- Link filtering integrates with Colly's HTML element processing for efficient visibility detection
- Domain comparison uses normalized hostname matching with comprehensive subdomain support
- Database retry logic specifically targets PostgreSQL connection issues with appropriate backoff strategies

## [0.3.8] – 2025-05-25

### Fixed
- **Critical Production Fix**: Resolved database schema mismatch causing task insertion failures
  - Fixed INSERT statement parameter count mismatch in `/internal/db/queue.go`
  - Corrected VALUES clause to match 9 fields with 9 placeholders (`$1` through `$9`)
  - Eliminated `pq: null value in column "depth" of relation "tasks" violates not-null constraint` error
- Fixed compilation issues in test utilities:
  - Updated `cmd/test_jobs/main.go` to use correct function signatures for `NewWorkerPool` and `NewJobManager`
  - Added proper `dbQueue` parameter initialization following production code patterns

### Technical Details
- The production database retained the deprecated `depth` column from v0.3.6, but the code was updated in v0.3.7 to remove depth functionality
- Database schema reset was required to align production database with current code expectations
- Task queue now successfully processes jobs without depth-related constraint violations

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
  - Added missing functionality to update sitemap counter when sitemap URLs are processed
  - Implemented incrementing of found task counter for URLs discovered during crawling
  - Fixed duplicate processing issue by moving the page processing mark after successful task creation
  - Updated job query to properly return counter values
- Improved task creation reliability by ensuring pages are only marked as processed after successful DB operations

## [0.3.5] – 2025-05-17

### Changed
- Major code refactoring to improve architecture and maintainability:
  - Eliminated duplicate code across the codebase
  - Removed global state in favor of proper dependency injection
  - Standardized function naming conventions
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
- Added URL normalization in sitemap processing
- Implemented robust error handling for URL processing
- Added better detection and correction of malformed URLs

### Fixed
- Fixed sitemap URL discovery and processing issues
- Improved relative URL handling in crawler
- Resolved issues with URL encoding/decoding in sitemap parser
- Fixed task queue URL processing in worker pool

### Changed
- Enhanced worker pool to better handle URL variations
- Updated job manager to properly normalize URLs before processing
- Improved URL validation logic in task processing

## [0.3.3] – 2025-04-22

### Added

- Added `sitemap_tasks` and `found_tasks` columns to the `jobs` table and corresponding fields in the Job struct
- Enqueued discovered links (same-domain pages and document URLs) via link extraction in the worker pool

### Changed

- `processTask` now filters `result.Links` to include only same-site pages and docs (`.pdf`, `.doc`, `.docx`) and enqueues them
- Updated `setupSchema` to include new columns with `ALTER TABLE IF NOT EXISTS`
- Exposed `Crawler.Config()` method to allow workers to read the `FindLinks` flag

### Documentation

- Updated `docs/architecture/jobs.md` to document new task counters and link-extraction behaviour

## [0.3.2] – 2025-04-21

### Changed

- Improved database configuration management with validation for required fields
- Enhanced worker pool notification system with more robust connection handling
- Simplified notification handling in worker pool with better error recovery
- Fixed linting issues in worker pool implementation

## [0.3.1] – 2025-04-21

### Changed

- Fixed documentation file references in INIT.md and README.md to use explicit relative paths
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
- Modified `jobs` table to reference domains by ID instead of storing domain names directly
- Updated `tasks` table to use page references instead of storing full URLs
- Refactored URL handling throughout the codebase to work with the new reference system

### Fixed

- Correctly set job and task completion timestamps (`CompletedAt`) when tasks and jobs complete.
- Fixed "append result never used" warnings in database operations
- Resolved unused import warnings and other code quality issues
- Fixed SQL parameter placeholders to use PostgreSQL-style numbered parameters (`$1`, `$2`, etc.) instead of MySQL/SQLite-style (`?`)
- Fixed task processing issues after database reset by ensuring consistent parameter style in all SQL queries
- Corrected parameter count mismatch in batch insert operations

## [0.2.0] - 2025-04-20

### Changed

- **Major Database Migration**: Fully migrated from SQLite/Turso to PostgreSQL
  - Removed all SQLite dependencies including `github.com/tursodatabase/libsql-client-go`
  - Reorganized database code structure, moving from `internal/db/postgres` to `internal/db`
  - Updated all application code to use PostgreSQL exclusively
  - Fixed all database-related tests

### Fixed

- Fixed crawler's `WarmURL` method to properly handle HTTP responses, context cancellation, and timeouts
- Resolved undefined functions and variables in test files related to the PostgreSQL task queue
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