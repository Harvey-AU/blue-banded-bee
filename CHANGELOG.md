# Changelog

All notable changes to the Blue Banded Bee project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Multiple version updates may occur on the same date, each with its own version number.
Each version represents a distinct set of changes, even if released on the same day.

## [0.5.2] – 2025-05-31

### Fixed

- **Authentication Component OAuth Redirect**: Resolved OAuth login redirecting to dashboard on test pages
  - Fixed auth state change listener to only redirect when `redirect-url` attribute is explicitly set
  - Simplified redirect logic - components without `redirect-url` stay on current page after login
  - Removed complex `test-mode` attribute approach in favour of intuitive behaviour
  - OAuth flows (Google, GitHub, Slack) now complete on test pages without unwanted redirects

### Enhanced

- **Component Design Philosophy**: Streamlined authentication component behaviour
  - Test pages: `<bb-auth-login>` (no redirect-url) = No redirect, works in both logged-in/out states
  - Production pages: `<bb-auth-login redirect-url="/dashboard">` = Redirects after successful login
  - Cleaner, more predictable component behaviour without special testing attributes

### Technical Details

- Auth state change listener now checks for `redirect-url` attribute before triggering redirects
- Removed `test-mode` from observed attributes and related logic
- Web Components rebuilt and deployed with simplified redirect handling
- Both initial load check and OAuth completion follow same redirect-url logic

## [0.5.1] – 2025-05-31

### Added

- **Dashboard Route**: Added `/dashboard` endpoint to resolve OAuth redirect 404 errors
  - Created dashboard page handler in Go API to serve `dashboard.html`
  - Updated Dockerfile to include dashboard.html in container deployment
  - Fixed authentication component redirect behaviour to prevent 404 errors after successful login

### Enhanced

- **Web Components Testing Infrastructure**: Comprehensive test page improvements
  - Added `test-mode` attribute to `bb-auth-login` component to prevent automatic redirects during testing
  - Created logout functionality for testing different authentication states
  - Enhanced test page with authentication status display and manual controls
  - Fixed redirect issues that prevented proper component testing

### Fixed

- **Authentication Component Redirect Logic**: Resolved automatic redirect problems
  - Modified `bb-auth-login` component to respect `test-mode="true"` attribute
  - Updated redirect logic to properly handle empty redirect URLs
  - Fixed issue where authenticated users were immediately redirected away from test pages

### Documentation

- **Supabase Integration Strategy**: Updated architecture documentation with platform integration recommendations
  - Added comprehensive Supabase feature mapping to development roadmap stages
  - Enhanced Architecture.md with real-time features, database functions, and Edge Functions strategy
  - Updated Roadmap.md to incorporate Supabase capabilities across Stage 5 (Performance & Scaling) and Stage 6 (Multi-tenant & Teams)
- **Development Workflow**: Enhanced CLAUDE.md with comprehensive working style guidance
  - Added communication preferences, git workflow, and tech stack leverage guidelines
  - Documented build process awareness, testing strategy, and configuration management practices
  - Created clear guidance for future AI sessions to work more productively

### Technical Details

- Dashboard route serves existing dashboard.html with corrected Supabase credentials
- Test mode in authentication component prevents both initial redirect checks and post-login redirects
- Web Components require rebuild (`npm run build`) when source files are modified
- Git workflow updated to commit freely but only push when ready for production testing

## [0.5.0] – 2025-05-30

### Added

- **Web Components MVP Interface**: Complete frontend infrastructure for Webflow integration
  - Built vanilla Web Components architecture using template + data slots pattern (industry best practice)
  - Created `bb-data-loader` core component for API data fetching and Webflow template population
  - Implemented `bb-auth-login` component with full Supabase authentication and social providers
  - Added `BBBaseComponent` base class with loading/error states, data binding, and event handling
- **Production Build System**: Rollup-based build pipeline for component distribution
  - Zero runtime dependencies (vanilla JavaScript, Supabase via CDN)
  - Minified production bundle (`bb-components.min.js`) ready for CDN deployment
  - Development and production builds with source maps and error handling
- **Static File Serving**: Integrated component serving into existing Go application
  - Added `/js/` endpoint to serve Web Components as static files from Go app
  - Components now accessible at `https://app.bluebandedbee.co/js/bb-components.min.js`
  - Docker container properly configured to include built components

### Enhanced

- **Webflow Integration Strategy**: Clarified multi-interface architecture and user journeys
  - **BBB Main Website**: Primary dashboard built on Webflow with embedded Web Components
  - **Webflow Designer Extension**: Lightweight progress modals within Webflow Designer
  - **Slack Integration**: Threaded conversations with links to main BBB site
  - Updated documentation to reflect three distinct user journey patterns
- **Component Architecture**: Template-driven approach for maximum Webflow compatibility
  - Data binding with `data-bind` attributes for text content population
  - Style binding with `data-style-bind` for dynamic CSS properties (progress bars, etc.)
  - Event handling for user interactions (view details, cancel jobs, form submissions)
  - Real-time updates with configurable refresh intervals and WebSocket support

### Technical Implementation

- **API Integration**: Seamless connection to existing `/v1/*` RESTful endpoints
  - Authentication via JWT tokens from Supabase Auth
  - Error handling with structured API responses and user-friendly error messages
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
- **Architecture Documentation**: Updated UI implementation plan with clarified user journeys
  - Documented template + data slots pattern and Web Components best practices
  - Clear separation between BBB main site, Designer Extension, and Slack integration
  - Technical justification for vanilla Web Components over framework alternatives

### Infrastructure

- **Deployment Ready**: Production infrastructure complete for Stage 4 MVP
  - Components automatically built and deployed with existing Fly.io workflow
  - Static file serving integrated into Go application without additional services
  - Backward compatible - no changes to existing API or authentication systems

## [0.4.3] – 2025-05-30

### Added

- **Complete Sentry Integration**: Comprehensive error tracking and performance monitoring
  - Properly initialised Sentry SDK in main.go with environment-aware configuration
  - Added error capture (`sentry.CaptureException()`) for critical business logic failures
  - Strategic error monitoring in job management, worker operations, and database transactions
  - Performance span tracking already operational: job operations, database operations, sitemap processing
  - Configured 10% trace sampling in production, 100% in development for optimal observability
- **Comprehensive Documentation Consolidation**: Streamlined from 31 to 10 documentation files
  - Created unified `ARCHITECTURE.md` combining system design, technical concepts, and component details
  - Consolidated `DEVELOPMENT.md` merging setup, testing, debugging, and contribution guidelines
  - Cleaned up `API.md` with consistent endpoint references and comprehensive documentation
  - Created `DATABASE.md` covering PostgreSQL schema, queries, operations, and performance optimisation
  - Consolidated future plans into 3 actionable documents: UI implementation, Webflow integration, scaling strategy

### Changed

- **Documentation Structure**: Complete reorganisation for maintainability and clarity
  - Eliminated content overlap between architecture files (mental-model, implementation-details, jobs)
  - Fixed content inconsistencies: corrected project stage references, removed deprecated depth column mentions
  - Updated README.md with accurate Stage 4 status and enhanced documentation index with descriptions
  - Improved CLAUDE.md with updated code organisation reflecting current package structure
- **Error Monitoring Strategy**: Strategic approach to avoid over-logging while capturing critical issues
  - Focus on infrastructure failures, data consistency issues, and critical business operations
  - Avoided granular task-level logging while maintaining comprehensive system health monitoring
  - Integration with existing performance spans for complete observability

### Removed

- **Redundant Documentation**: Eliminated 21 redundant files and outdated content
  - Removed overlapping architecture files: mental-model.md, implementation-details.md, jobs.md
  - Consolidated reference files: codebase-structure.md, file-map.md, auth-integration.md, database-config.md
  - Cleaned up outdated plans: 8 completed or irrelevant planning documents
  - Removed CONTRIBUTING.md (merged into DEVELOPMENT.md) and duplicate guide content

### Fixed

- **Documentation Accuracy**: Corrected stale and inconsistent information throughout
  - Fixed project stage references (Stage 3 → Stage 4) in README.md
  - Removed deprecated depth column references from database documentation
  - Updated API endpoint paths to match current `/v1/*` structure
  - Corrected outdated technology references (SQLite → PostgreSQL)

## [0.4.2] – 2025-05-29

### Added

- **RESTful API Architecture**: Complete API infrastructure overhaul with modern standards
  - Implemented standardised error handling with request IDs and consistent HTTP status codes
  - Created comprehensive middleware stack: CORS, request ID tracking, structured logging, rate limiting
  - Built RESTful endpoint structure under `/v1/*` namespace for versioned API access
  - Added proper authentication middleware integration with Supabase JWT validation
- **API Response Standardisation**: Consistent response formats across all endpoints
  - Success responses include `status`, `data`, `message`, and `request_id` fields
  - Error responses provide structured error information with HTTP status codes and error codes
  - Request ID tracking for distributed tracing and debugging support
- **Enhanced Security**: Improved security posture with secured admin endpoints
  - Moved debug endpoints to `/admin/*` namespace with environment variable protection
  - Added CORS middleware for secure cross-origin requests
  - Implemented rate limiting with proper IP detection and standardised error responses

### Changed

- **API Endpoint Structure**: Migrated from ad-hoc endpoints to RESTful design
  - Job creation: `POST /v1/jobs` with JSON body instead of query parameters
  - Job status: `GET /v1/jobs/:id` following RESTful conventions
  - Authentication: Consolidated under `/v1/auth/*` namespace
  - Health checks: Standardised `/health` and `/health/db` endpoints
- **Error Handling**: Replaced inconsistent `http.Error()` calls with structured error responses
  - All errors now include request IDs for tracing
  - Consistent error codes: `BAD_REQUEST`, `UNAUTHORISED`, `NOT_FOUND`, etc.
  - Proper HTTP status code usage throughout the API
- **Code Organisation**: Refactored API logic into dedicated `internal/api` package
  - Separated concerns: handlers, middleware, errors, responses, authentication
  - Dependency injection pattern with clean handler structure
  - Eliminated duplicate endpoint logic and inconsistent patterns

### Removed

- **Legacy Endpoints**: Removed unused endpoints since APIs are not yet published
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

- New `internal/api` package structure with clean separation of handlers, middleware, and utilities
- Middleware stack processes requests in proper order: CORS → Request ID → Logging → Rate Limiting
- JWT authentication middleware integrates seamlessly with Supabase token validation
- Request ID generation uses timestamp + random bytes for unique request tracking
- Error responses provide consistent structure while maintaining security (no information leakage)

## [0.4.1] – 2025-05-27

### Fixed

- **Database Schema Issues**: Resolved critical production database errors
  - Added missing `error_message` column to jobs table to prevent database insertion failures
  - Fixed duplicate user creation constraint violations with idempotent user registration
  - `CreateUserWithOrganisation` now handles existing users gracefully instead of failing
- **User Registration Flow**: Enhanced authentication reliability
  - Multiple login attempts with same user ID no longer cause database constraint violations
  - Existing users are returned with their organisations rather than attempting duplicate creation
  - Improved error handling and logging for user creation scenarios

### Enhanced

- **Development Workflow**: Added git policy documentation to prevent accidental commits
- **Project Planning**: Added multi-provider account linking testing to roadmap for future investigation

### Technical Details

- Database migration adds `error_message TEXT` column to jobs table with `ALTER TABLE IF NOT EXISTS`
- User creation now checks for existing users before attempting INSERT operations
- Transaction rollback properly handles failed user creation attempts
- All database fixes are backward compatible with existing installations

## [0.4.0] – 2025-05-27

### Added

- **Complete Supabase Authentication System**: Full multi-tenant authentication with social login support
  - JWT validation middleware with structured error handling and token validation
  - Support for 8 social login providers: Google, Facebook, Slack, GitHub, Microsoft, Figma, LinkedIn + Email/Password
  - Custom domain authentication using `auth.bluebandedbee.co` for professional OAuth flows
  - User and organisation management with automatic organisation creation on signup
  - Row Level Security (RLS) policies for secure multi-tenant data access
- **Protected API Endpoints**: All job creation and user data endpoints now require authentication
  - `/site` endpoint (job creation) now requires valid JWT token and links jobs to users/organisations
  - `/job-status` endpoint protected with organisation-scoped access control
  - `/api/auth/profile` endpoint for authenticated user profile access
  - User registration API with automatic organisation linking
- **Database Schema Extensions**: Enhanced schema to support multi-tenant architecture
  - Added `users` and `organisations` tables with foreign key relationships
  - Added `user_id` and `organisation_id` columns to `jobs` table
  - Implemented Row Level Security on all user-related tables
  - Database migration logic for existing installations

### Enhanced

- **Authentication Flow**: Complete OAuth integration with account linking support
  - Flexible email-based account linking with UUID-based permanent user identity
  - Session management with token expiry detection and refresh warnings
  - Structured error responses for authentication failures
  - Support for multiple auth providers per user account
- **Multi-tenant Job Management**: Jobs are now scoped to organisations with shared access
  - All organisation members can view and manage all jobs within their organisation
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
- Database migration handles existing installations with `ALTER TABLE IF NOT EXISTS`
- JWT middleware supports both required and optional authentication scenarios
- Account linking strategy preserves user choice while preventing duplicate accounts
- All authentication endpoints follow RESTful conventions with proper HTTP status codes

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

### Changed

- **Link Discovery Default**: Changed default behaviour to enable link discovery by default
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
  - Complete time series charts with all increments (shows empty periods for accurate visualisation)
  - Dynamic group-by selection that auto-updates based on date range scope

### Fixed

- **Timezone Consistency**: Resolved incorrect timestamp display in dashboard
  - Standardised all database timestamps to use UTC (`time.Now().UTC()` in Go, `NOW()` in PostgreSQL)
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
- **Chart Visualisation**: Improved dashboard charts with complete time coverage
  - Charts now display all time increments for selected range (e.g., all 24 hours for "Today")
  - Fixed grouping logic to automatically select appropriate time granularity
  - Enhanced debugging output for troubleshooting data visualisation issues

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
  - `www.test.com` and `test.com` now correctly recognised as same domain
  - Enhanced subdomain detection works with both normalised and original domains
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
- Domain comparison uses normalised hostname matching with comprehensive subdomain support
- Database retry logic specifically targets PostgreSQL connection issues with appropriate backoff strategies

## [0.3.8] – 2025-05-25

### Fixed

- **Critical Production Fix**: Resolved database schema mismatch causing task insertion failures
  - Fixed INSERT statement parameter count mismatch in `/internal/db/queue.go`
  - Corrected VALUES clause to match 9 fields with 9 placeholders (`$1` through `$9`)
  - Eliminated `pq: null value in column "depth" of relation "tasks" violates not-null constraint` error
- Fixed compilation issues in test utilities:
  - Updated `cmd/test_jobs/main.go` to use correct function signatures for `NewWorkerPool` and `NewJobManager`
  - Added proper `dbQueue` parameter initialisation following production code patterns

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
  - Reorganised database code structure, moving from `internal/db/postgres` to `internal/db`
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
