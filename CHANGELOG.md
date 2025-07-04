# Changelog

All notable changes to the Blue Banded Bee project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Multiple version updates may occur on the same date, each with its own version number.
Each version represents a distinct set of changes, even if released on the same day.

## [0.5.22] – 2025-07-03

### Enhanced
- **Database Performance**: Implemented an in-memory cache for page lookups (`pages` table) to significantly reduce redundant "upsert" queries. This dramatically improves performance during the page creation phase of a job by caching results for URLs that are processed multiple times within the same job.

## [0.5.21] – 2025-07-03

### Changed
- **Database Driver**: Switched the PostgreSQL driver from `lib/pq` to the more modern and performant `pgx`.
  - This resolves underlying issues with connection poolers (like Supabase PgBouncer) without requiring connection string workarounds.
  - The `prepare_threshold=0` setting is no longer needed and has been removed.
- **Notification System**: Rewrote the database notification listener (`LISTEN/NOTIFY`) to use `pgx`'s native, more robust implementation, improving real-time worker notifications.

### Enhanced
- **Database Performance**: Optimised the `tasks` table indexing for faster worker performance.
  - Replaced several general-purpose indexes with a highly specific partial index (`idx_tasks_pending_claim_order`) for the critical task-claiming query.
  - This significantly improves the speed and scalability of task processing by eliminating expensive sorting operations.

### Fixed
- **Graceful Shutdown**: Fixed an issue where the new `pgx`-based notification listener would not terminate correctly during a graceful shutdown, preventing the worker pool from stopping cleanly.

## [0.5.20] – 2025-07-03

### Added
- **Cache Warming Auditing**: Added detailed auditing for the cache warming retry mechanism.
  - The `tasks` table now includes a `cache_check_attempts` JSONB column to store the results of each `HEAD` request check.
  - Each attempt logs the cache status and the delay before the check.

### Enhanced
- **Cache Warming Strategy**: Improved the cache warming retry logic for more robust cache verification.
  - Increased the maximum number of `HEAD` check retries from 5 to 10.
  - Implemented a progressive backoff for the delay between checks, starting at 2 seconds and increasing by 1 second for each subsequent attempt.

### Fixed
- **Database Connection Stability**: Resolved a critical issue causing `driver: bad connection` and `unexpected Parse response` errors when using a connection pooler (like Supabase PgBouncer).
  - The PostgreSQL connection string now includes `prepare_threshold=0` to disable server-side prepared statements, ensuring compatibility with transaction-based poolers.
  - Added an automatic schema migration (`ALTER TABLE`) to ensure the `cache_check_attempts` column is added to existing databases.

## [0.5.19] – 2025-07-02

### Enhanced
- **Task Prioritisation**: Refactored job initiation and link discovery for more accurate and efficient priority assignment.
  - The separate, post-sitemap homepage scan for header/footer links has been removed, eliminating a redundant HTTP request and potential race conditions.
  - The homepage (`/`) is now assigned a priority of `1.000` directly during sitemap processing.
  - Link discovery logic is now context-aware:
    - On the homepage, links in the `<header>` are assigned priority `1.000`, and links in the `<footer>` get `0.990`.
    - On all other pages, links within `<header>` and `<footer>` are ignored, preventing low-value navigation links from being crawled repeatedly.
    - Links in the page body inherit their priority from the parent page as before.

## [0.5.18] – 2025-07-02

### Enhanced
- **Crawler Efficiency**: Implemented a comprehensive visibility check to prevent the crawler from processing links that are hidden. The check includes inline styles (`display: none`, `visibility: hidden`), common utility classes (`hide`, `d-none`, `sr-only`, etc.), and attributes like `aria-hidden="true"`, `data-hidden`, and `data-visible="false"`. This significantly reduces the number of unnecessary tasks created.

## [0.5.17] – 2025-07-02

### Added
- **Task Logging**: Included the `priority_score` in the log message when a task is claimed by a worker for improved debugging.

### Fixed
- **Crawler Stability**: Fixed an infinite loop issue where relative links containing only a query string (e.g., `?page=2`) were repeatedly appended to the current URL instead of replacing the existing query.

## [0.5.16] – 2025-07-02

### Enhanced
- **User Registration**: The default organisation name is now set to the user's full name upon registration for a more personalized experience.
- **Organisation Name Cleanup**: Organisation names derived from email addresses are now cleaned of common TLDs (e.g., `.com`), ignores generic domains, and doesn't capitalise.

### Fixed
- **Database Efficiency**: Removed a redundant database call in the page creation process by passing the domain name as a parameter.
- **Task Auditing**: Ensured that the `retry_count` for a task is correctly preserved when a task succeeds after one or more retries.

## [0.5.15] – 2025-07-02

### Changed
- **Codebase Cleanup**: Numerous small changes to improve code clarity, including fixing comment typos, removing unused code, and standardising function names.
- **Worker Pool Logic**: Simplified worker scaling logic and reduced worker sleep time to improve responsiveness.

### Fixed
- **Architectural Consistency**: Corrected a flaw where the `WorkerPool` did not correctly use the `JobManager` for enqueueing tasks, ensuring duplicate-checking logic is now properly applied.

### Documentation
- **Project Management**: Updated `TODO.md` to convert all file references to clickable links and consolidated several in-code `TODO` comments into the main file for better tracking.
- **AI Collaboration**: Added `gemini.md` to document the best practices and working protocols for AI collaboration on this project.
- **Language Standardisation**: Renamed `Serialize` function to `Serialise` to maintain British English consistency throughout the codebase.

## [0.5.14] – 2025-06-26

### Added

- **Task Prioritisation Implementation**: Implemented priority-based task processing system
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

- **Task Prioritisation Planning**: Created comprehensive plan for priority-based task processing
  - PostgreSQL view-based approach using percentage scores (0.0-1.0)
  - Minimal schema changes - only adds `priority_score` column to pages table
  - Homepage detection and automatic highest priority assignment
  - Link propagation scoring - pages inherit 80% of source page priority
  - Detailed implementation plan in [docs/plans/task-prioritization.md](docs/plans/task-prioritisation.md)

### Enhanced

- **Job Duplicate Prevention**: Cancel existing jobs when creating new job for same domain

  - Prevents multiple concurrent crawls of same domain
  - Automatically cancels in-progress jobs for domain before creating new one
  - Improves resource utilisation and prevents redundant crawling

- **Cache Warming Timing**: Adjusted delay for second cache warming attempt
  - Increased delay to 1.5x initial response time for better cache propagation
  - Added randomisation to cache warming delays for more natural traffic patterns
  - Enhanced logging of cache status results for analysis

### Fixed

- **Link Discovery**: Fixed link extraction to properly find paginated links
  - Restored proper link discovery functionality that was inadvertently disabled

## [0.5.12] – 2025-06-06

### Enhanced

- **Cache Warming Timing**: Added 500ms delay between first request (cache MISS detection) and second request (cache verification) to allow CDNs time to process and cache the first response
- **Webflow Webhook Domain Selection**: Fixed webhook to use first domain (primary/canonical) instead of last domain (staging .webflow.io)
- **Webflow Webhook Page Limits**: Removed 100-page limit for webhook-triggered jobs - now unlimited for complete site cache warming

### Fixed

- **Build Error**: Removed unused `fmt` import from `internal/api/handlers.go` that was causing GitHub Actions build failures

## [0.5.11] – 2025-06-06

### Added

- **Webflow Webhook Integration**: Automatic cache warming triggered by Webflow site publishing
  - Webhook endpoint `/v1/webhooks/webflow/USER_ID` for user-specific triggers
  - Automatic job creation and execution when Webflow sites are published
  - Smart domain selection (uses first domain in array - primary/canonical domain)
- **Job Source Tracking**: Comprehensive tracking of job creation sources for debugging and analytics
  - `source_type` field: `"webflow_webhook"` or `"dashboard"`
  - `source_detail` field: Clean display text (publisher name or action type)
  - `source_info` field: Full debugging JSON (webhook payload or request details)

### Enhanced

- **Job Creation Architecture**: Refactored to eliminate code duplication
  - Extracted shared `createJobFromRequest()` function for consistent job creation
  - Webhook and dashboard endpoints now use common job creation logic
  - Improved maintainability and consistency across creation sources

### Technical Implementation

- **Database Schema**: Added `source_type`, `source_detail`, and `source_info` columns to jobs table
- **Webhook Security**: No authentication required for webhooks (Webflow can POST directly)
- **Source Attribution**: Dashboard jobs tagged as `"create_job"` ready for future `"retry_job"` actions

## [0.5.10] – 2025-06-06

### Fixed

- **Cache Warming Data Storage**: Fixed second cache warming data not being stored in database
- **Timeout Retry Logic**: Added automatic retry for network timeouts and connection errors up to 5 attempts

## [0.5.9] – 2025-06-06

### Enhanced

- **Worker Pool Scaling**: Improved auto-scaling for better performance and bot protection
  - Simplified worker scaling from complex job-requirements tracking to simple +5/-5 arithmetic per job
  - Auto-scaling: 1 job = 5 workers, 2 jobs = 10 workers, up to maximum 50 workers (10 jobs)
  - Each job gets dedicated workers preventing single-job monopolisation and bot detection risks
- **Database Connection Pool**: Increased to support higher concurrency
  - MaxOpenConns: 25 → 75 connections to prevent bottlenecks with increased worker count
  - MaxIdleConns: 10 → 25 connections for better connection reuse
- **Crawler Rate Limiting**: Reduced aggressive settings for better politeness to target servers
  - MaxConcurrency: 50 → 10 concurrent requests per crawler instance
  - RateLimit: 100 → 10 requests per second for safer cache warming

### Technical Implementation

- **Simplified Scaling Logic**: Removed complex `jobRequirements` map and maximum calculation logic
  - `AddJob()`: Simple `currentWorkers + 5` with max limit of 50
  - `RemoveJob()`: Simple `currentWorkers - 5` with minimum limit of 5
  - Eliminated per-job worker requirement tracking for cleaner, more predictable scaling

## [0.5.8] – 2025-06-03

### Added

- **Cache Warming System**: Implemented blocking cache warming with second HTTP requests on cache MISS/BYPASS
  - Added `second_response_time` and `second_cache_status` columns to track cache warming effectiveness
  - Cache warming logic integrated into crawler with automatic MISS detection from multiple CDN headers
  - Blocking approach ensures complete metrics collection and immediate cache verification
  - Supabase can calculate cache warming success (`cache_warmed`) using: `second_cache_status = 'HIT'`

### Fixed

- **Critical Link Extraction Bug**: Fixed context handling bug that was preventing all link discovery
  - Link extraction was defaulting to disabled when `find_links` context value wasn't properly set
  - Now defaults to enabled link extraction, fixing pagination link discovery (e.g., `?b84bb98f_page=2`)
  - **TODO: Verify this fix works by testing teamharvey.co/stories pagination links**
- **Link Extraction Logic**: Consolidated to Colly-only crawler to extract only user-clickable links
  - Removed overly aggressive filtering that was blocking legitimate navigation links
  - Now only filters empty hrefs, fragments (#), javascript:, and mailto: links
- **Dashboard Form**: Fixed max_pages input field to consistently show default value of 0 (unlimited)

### Enhanced

- **Code Architecture**: Eliminated logic duplication in cache warming implementation
  - Cache warming second request reuses main `WarmURL()` method with `findLinks=false`
  - Removed redundant `cache_warmed` field - can be calculated in Supabase/dashboard
  - Database schema includes cache warming columns in initial table creation for new installations

## [0.5.7] – 2025-06-01

### Fixed

- **Critical Job Creation Bug**: Resolved POST request failures preventing job creation functionality
  - Fixed `BBDataBinder.fetchData()` method to properly accept and use options parameter for method, headers, and body
  - Method signature updated from `async fetchData(endpoint)` to `async fetchData(endpoint, options = {})`
  - POST requests now correctly send JSON body data instead of being converted to GET requests
  - Job creation modal now successfully creates jobs and refreshes dashboard data
- **Data Binding Library**: Enhanced fetchData method to support all HTTP methods
  - Added proper options parameter spread to fetch configuration
  - Maintained backward compatibility for GET requests (existing code unaffected)
  - Fixed API integration throughout dashboard for POST, PUT, DELETE operations

### Enhanced

- **Job Creation Modal**: Simplified interface with essential fields only
  - Removed non-functional include_paths and exclude_paths fields that aren't implemented in API
  - Hidden concurrency setting as job-level concurrency has no effect (system uses global concurrency of 50)
  - Set sensible defaults: use_sitemap=true, find_links=true, concurrency=5
  - Changed domain input from URL type to text type to allow simple domain names (e.g., "teamharvey.co")
- **User Experience**: Improved job creation workflow with better validation and feedback
  - Domain input now accepts domain names without requiring full URLs with protocol
  - Better error messaging when job creation fails
  - Real-time progress updates after successful job creation
  - Toast notifications for success and error states

### Technical Implementation

- **Data Binding Library Rebuild**: Updated and rebuilt all Web Components with fetchData fix
  - Rebuilt `bb-data-binder.js` and `bb-data-binder.min.js` with corrected method implementation
  - Updated `bb-components.js` and `bb-components.min.js` for production deployment
  - All POST/PUT/DELETE API calls throughout the application now function correctly
- **API Integration**: Fixed job creation endpoint integration
  - `/v1/jobs` POST endpoint now receives proper JSON data from dashboard
  - Request debugging confirmed proper method, headers, and body transmission
  - Removed debug logging after confirming fix works correctly

### Development Process

- **Testing Workflow**: Comprehensive debugging and testing of job creation flow
  - Traced request path from modal form submission through data binding library to API
  - Console logging confirmed fetchData method was ignoring POST parameters
  - Verified fix works by testing job creation with various domain inputs
  - Confirmed dashboard refresh and real-time updates work after job creation

## [0.5.6] – 2025-06-01

### Enhanced

- **User Experience**: Improved dashboard user identification and testing workflow
  - Updated dashboard header to display actual user email instead of placeholder "user@example.com"
  - Added automatic user avatar generation with smart initials extraction from email addresses
  - Real-time user info updates when authentication state changes (login/logout/token refresh)
  - Enhanced user session management with proper cleanup and state synchronisation

### Fixed

- **Dashboard User Display**: Resolved hardcoded placeholder in user interface
  - Replaced static "user@example.com" text with dynamic user email from Supabase session
  - Fixed avatar initials to properly reflect current authenticated user
  - Added fallback states for loading and error conditions
  - Improved authentication state listening for immediate UI updates

### Technical Implementation

- **Session Management**: Enhanced authentication flow integration
  - Direct Supabase session querying for reliable user data access
  - Auth state change listeners update user info automatically across login/logout cycles
  - Graceful error handling for session retrieval failures
  - Smart initials generation supporting various email formats (firstname.lastname, firstname_lastname, etc.)

### Development Workflow

- **Production Testing**: Completed full 6-step development workflow
  - GitHub Actions deployment successful for user interface improvements
  - Playwright MCP testing infrastructure verified (with noted session stability issues)
  - Production deployment confirmed working via manual verification

## [0.5.5] – 2025-06-01

### Added

- **Authentication Testing Infrastructure**: Comprehensive testing workflow for authentication flows
  - Created and successfully tested account creation with `simon+claude@teamharvey.co` using real-time password validation
  - Implemented real-time password strength checking using zxcvbn library with visual feedback indicators
  - Added password confirmation validation with visual success/error states
  - Comprehensive authentication modal testing via MCP Playwright browser automation
  - Database verification of account creation process (account created but requires email confirmation)

### Enhanced

- **Authentication Modal UX**: Production-ready authentication interface with industry-standard patterns
  - Real-time password strength evaluation using zxcvbn library (0-4 scale with colour-coded feedback)
  - Live password confirmation matching with instant visual validation feedback
  - Enhanced form validation with field-level error states and success indicators
  - Improved user experience with immediate feedback on password quality and match status
  - Modal-based authentication flow supporting login, signup, and password reset workflows

### Fixed

- **Domain References**: Corrected all application URLs to use proper domain structure
  - Updated authentication redirect URLs from `bluebandedbee.co` to `app.bluebandedbee.co`
  - Fixed API base URLs in Web Components to point to `app.bluebandedbee.co`
  - Updated all script URLs and CDN references in examples and documentation
  - Rebuilt Web Components with correct production URLs

### Documentation

- **Domain Usage Clarification**: Comprehensive documentation of domain structure and usage
  - **Local development**: `http://localhost:8080` - Blue Banded Bee application for local testing
  - **Production marketing site**: `https://bluebandedbee.co` - Marketing website only
  - **Production application**: `https://app.bluebandedbee.co` - Live application, services, demo pages
  - **Authentication service**: `https://auth.bluebandedbee.co` - Supabase authentication (unchanged)
  - Updated all documentation files to clearly specify domain purposes and usage contexts

### Technical Implementation

- **Authentication Flow Testing**: Complete browser automation testing of authentication workflows
  - Tested modal opening/closing, form switching between login/signup modes
  - Verified real-time password validation and strength indicators
  - Confirmed account creation process with database verification
  - Established testing patterns for future authentication feature development
- **Web Components Updates**: Rebuilt production components with correct domain configuration
  - Updated `web/src/utils/api.js` and rebuilt distribution files
  - Fixed OAuth redirect URLs in `dashboard.html`
  - Updated test helpers and example files with correct domain references

## [0.5.4] – 2025-05-31

### Added

- **Complete Data Binding Library**: Comprehensive template + data binding system for flexible dashboard development
  - Built `BBDataBinder` JavaScript library with `data-bb-bind` attribute processing for dynamic content
  - Implemented template engine with `data-bb-template` for repeated elements (job lists, tables, etc.)
  - Added authentication integration with `data-bb-auth` for conditional element display
  - Created comprehensive form handling with `data-bb-form` attributes and real-time validation
  - Built style and attribute binding with `data-bb-bind-style` and `data-bb-bind-attr` for dynamic CSS and attributes
- **Enhanced Form Processing**: Production-ready form handling with validation and error management
  - Real-time field validation with `data-bb-validate` attributes and custom validation rules
  - Automatic form submission to API endpoints with authentication token handling
  - Loading states, success/error messaging, and form reset capabilities
  - Support for job creation, profile updates, and custom forms with configurable endpoints
- **Example Templates**: Complete working examples demonstrating all data binding features
  - `data-binding-example.html` - Full demonstration of template binding with mock data
  - `form-example.html` - Comprehensive form handling examples with validation
  - `dashboard-enhanced.html` - Production-ready dashboard using data binding library

### Enhanced

- **Build System**: Updated Rollup configuration to build data binding library alongside Web Components
  - Added `bb-data-binder.js` and `bb-data-binder.min.js` builds for production deployment
  - Library available at `/js/bb-data-binder.min.js` endpoint for CDN-style usage
  - Zero runtime dependencies - works with vanilla JavaScript and Supabase

### Technical Implementation

- **Data Binding Architecture**: Template-driven approach where HTML controls layout and JavaScript provides functionality
  - DOM scanning system finds and registers elements with data binding attributes
  - Efficient element updates with path-based data mapping and template caching
  - Event delegation for `bb-action` attributes combined with data binding for complete template system
- **Authentication Integration**: Seamless Supabase Auth integration with conditional rendering
  - Elements with `data-bb-auth="required"` only show when authenticated
  - Elements with `data-bb-auth="guest"` only show when not authenticated
  - Automatic auth state monitoring and element visibility updates
- **Form Processing Pipeline**: Complete form lifecycle management from validation to submission
  - Client-side validation with multiple rule types (required, email, URL, length, pattern)
  - API endpoint determination based on form action with automatic authentication headers
  - Success/error handling with custom events and configurable redirects

## [0.5.3] – 2025-05-31

### Changed

- **Dashboard Architecture**: Replaced Web Components with vanilla JavaScript + attribute-based event handling
  - Removed Web Components dependencies (`bb-auth-login`, `bb-job-dashboard`) from dashboard
  - Implemented vanilla JavaScript with modern styling for better reliability and maintainability
  - Added attribute-based event system: elements with `bb-action` attributes automatically handle functionality
  - Replaced `onclick` handlers with `bb-action="refresh-dashboard"`, `bb-action="create-job"` pattern
  - Maintained modern UI design whilst switching to proven vanilla JavaScript approach

### Enhanced

- **Template + Data Binding Foundation**: Established framework for flexible dashboard development
  - Dashboard now demonstrates template approach where HTML layout is customisable
  - JavaScript automatically scans for `bb-action` and `bb-data-*` attributes to provide functionality
  - Event delegation system allows any HTML element with `bb-action` to trigger Blue Banded Bee features
  - Sets foundation for future template binding system where users control layout design

### Fixed

- **Production Dashboard Stability**: Resolved Web Components authentication and loading issues
  - Dashboard now uses proven vanilla JavaScript patterns instead of experimental Web Components
  - Removed complex component lifecycle management in favour of direct API integration
  - Eliminated dependency on Web Components build pipeline for core dashboard functionality

### Technical Details

- Consolidated `dashboard-new.html` and `dashboard.html` into single vanilla JavaScript implementation
- Added `setupAttributeHandlers()` function with event delegation for `bb-action` attributes
- Maintained API integration with `/v1/dashboard/stats` and `/v1/jobs` endpoints
- Preserved modern grid layout and responsive design from Web Components version

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

## [0.3.2] ��� 2025-04-21

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
