# Roadmap

## Current Status Overview

- PostgreSQL migration completed (now using Supabase after briefly using Postgres in Fly)
- Successfully deployed to Fly.io with verified functionality
- Local development environment established with PostgreSQL
- Currently processing jobs, tasks and recording results
- Worker pool using PostgreSQL's row-level locking implemented
- Enhanced sitemap processing with improved URL handling and normalisation
- Improved link discovery and URL validation across the codebase
- Completed major code refactoring to improve architecture and maintainability
- Removed unnecessary depth functionality from the codebase

## ✅ Stage 0: Project Setup & Infrastructure (6-10 hrs)

### Development Environment Setup (2-3 hrs)

- [x] Initialise GitHub repository
- [x] Set up branch protection
- [x] Resolve naming issues and override branch protection for admins
- [x] Create dev/prod branches
- [x] Set up local development environment
- [x] Add initial documentation

### Go Project Structure (2-3 hrs)

- [x] Initialise Go project
- [x] Set up dependency management
- [x] Create project structure
- [x] Add basic configs
- [x] Set up testing framework

### Production Infrastructure Setup (2-4 hrs)

- [x] Set up dev/prod environments
- [x] Configure environment variables
- [x] Set up secrets management
- [x] Create Dockerfile and container setup
- [x] Configure Fly.io
  - [x] Set up Fly.io account and project
  - [x] Configure deployment settings
  - [x] Set up environment variables in Fly.io
  - [x] Create deployment workflow
  - [x] Add health check endpoint monitoring
- [x] Test production deployment
- [x] Initial Sentry.io connection

## ✅ Stage 1: Core Setup & Basic Crawling (15-25 hrs)

### Core API Implementation (3-5 hrs)

- [x] Initialise Go project structure and dependencies
- [x] Set up basic API endpoints
- [x] Set up environment variables and configs
- [x] Implement basic health checks and monitoring
- [x] Add basic error monitoring with Sentry
- [x] Set up endpoint performance tracking
- [x] Add graceful shutdown handling
- [x] Implement configuration validation

### Enhance Crawler Results (8-12 hrs)

- [x] Set up Colly crawler configuration
- [x] Implement concurrent crawling logic
- [x] Add basic error handling
- [x] Add rate limiting (fixed client IP detection)
- [x] Add retry logic
- [x] Handle different response types/errors
- [x] Implement cache validation checks
- [x] Add crawler-specific error tracking
- [x] Set up crawler performance monitoring

### Set up Turso for storing results (4-8 hrs)

- [x] Design database schema
- [x] Set up Turso connection and config
- [x] Implement data models and queries
- [x] Add basic error handling
- [x] Add retry logic
- [x] Add database performance monitoring
- [x] Set up query error tracking

## ✅ Stage 2: Multi-domain Support & Job Queue Architecture

### Job Queue Architecture

- [x] Design job and task data structures
- [x] Implement persistent job storage in database
- [x] Create worker pool for concurrent URL processing
- [x] Add job management API (create, start, cancel, status)
- [x] Implement database retry logic for job operations to handle transient errors
- [x] Enhance error reporting and monitoring

### Sitemap Integration (2-3 hrs)

- [x] Implement sitemap.xml parser
- [x] Add URL filtering based on path patterns
- [x] Handle sitemap index files
- [x] Process multiple sitemaps
- [x] Implement robust URL normalisation in sitemap processing
- [x] Add improved error handling for malformed URLs

### Link Discovery & Crawling (2-3 hrs)

- [x] Extract links from crawled pages
- [x] Filter links to stay within target domain
- [x] Basic link discovery logic
- [x] Queue discovered links for processing

### Job Management API (1-2 hrs)

- [x] Create job endpoints (create/list/get/cancel)
- [x] Add progress calculation and reporting
- [x] Store recent crawled pages in job history
- [x] Implement multi-domain support

## 🚧 Stage 3: PostgreSQL Migration & Performance Optimisation (3-4 weeks)

### Fly.io Production Setup (4-6 hrs)

- [x] Set up production environment on Fly.io
- [x] Deploy and test rate limiting in production
- [x] Configure auto-scaling rules
- [x] Set up production logging
- [x] Implement monitoring alerts
- [ ] Configure backup strategies

### Performance Optimisation (4-6 hrs)

- [x] Implement caching layer
- [x] Optimise database queries
- [x] Configure rate limiting with proper client IP detection
- [x] Add performance monitoring
- [x] Made decision to switch to postgres at this point

### PostgreSQL Migration (10-15 hrs)

#### PostgreSQL Setup and Infrastructure (2-3 hrs)

- [x] Set up PostgreSQL on Fly.io
  - [x] Create database instance
  - [x] Configure connection settings
  - [x] Configure security settings

#### Critical Database Management (Priority)

- [ ] Set up backup schedule and automated recovery testing
- [ ] Implement data retention policies
- [ ] Create monitoring for database health

#### Database Layer Replacement (3-4 hrs)

- [x] Implement PostgreSQL schema
  - [x] Convert SQLite schema to PostgreSQL syntax
  - [x] Add proper indexes
  - [x] Implement connection pooling
- [x] Replace database access layer
  - [x] Update db package to use PostgreSQL
  - [x] Add health checks and monitoring
  - [x] Implement efficient error handling

#### Task Queue and Worker Redesign (4-5 hrs)

- [x] Implement PostgreSQL-based task queue
  - [x] Use row-level locking with SELECT FOR UPDATE SKIP LOCKED
  - [x] Optimise for concurrent access
  - [x] Plan task prioritisation implementation (docs created)
  - [ ] Implement task prioritisation
- [x] Redesign worker pool
  - [x] Create single global worker pool
  - [x] Implement optimised task acquisition
  - [ ] Add proper worker scaling

#### URL Processing Improvements (2-3 hrs)

- [x] Enhanced sitemap processing
  - [x] Implement robust URL normalisation
  - [x] Add support for relative URLs in sitemaps
  - [x] Improve error handling for malformed URLs
- [x] Improve URL validation
  - [x] Better handling of URL variations
  - [x] Consistent URL formatting throughout the codebase

#### Code Refactoring (2-3 hrs)

- [x] Eliminate duplicate code
  - [x] Move database operations to a unified interface
  - [x] Consolidate similar functions into single implementations
  - [x] Move functions to appropriate packages
- [x] Remove global state
  - [x] Implement proper dependency injection
  - [x] Replace global DB instance with passed parameters
  - [x] Improve transaction management with DbQueue
- [x] Standardise naming conventions
  - [x] Use consistent function names across packages
  - [x] Clarify responsibilities between packages

#### Code Cleanup (2-3 hrs)

- [x] Remove redundant worker pool creation
  - [x] Eliminate duplicate worker pools in API handlers
  - [x] Ensure single global worker pool is used consistently
- [x] Simplify middleware stack
  - [x] Reduce excessive transaction monitoring
  - [x] Optimise Sentry integrations
  - [x] Remove unnecessary wrapping functions
- [x] Clean up API endpoints (plan created)
  - [x] Document endpoints to consolidate or remove
  - [x] Plan endpoint implementation simplification
  - [x] Standardise error handling approach
- [x] Fix metrics collection (plan created)
  - [x] Document metrics to expose
  - [x] Plan for unused metrics tracking removal
  - [x] Identify relevant PostgreSQL metrics to add
- [x] Remove depth functionality
  - [x] Remove `depth` column from `tasks` table
  - [x] Remove `max_depth` column from `jobs` table
  - [x] Update `EnqueueURLs` function to remove depth parameter
  - [x] Update type definitions to remove depth fields
  - [x] Remove depth-related logic from link discovery process
  - [x] Update documentation to remove depth references

#### Final Transition (1-2 hrs)

- [x] Update core endpoints to use new implementation
- [x] Remove SQLite-specific code
- [x] Clean up dependencies and imports
- [x] Update configuration and documentation

## ⚪ Stage 4: Auth & User Management (2-3 weeks)

### Implement Clerk authentication (3-5 days)

- [ ] Set up Clerk project configuration
- [ ] Implement auth middleware
- [ ] Add social login providers
- [ ] Set up user session handling
- [ ] Implement auth error handling

### Connect user data to PostgreSQL (2-3 days)

- [ ] Design user data schema
- [ ] Implement user profile storage
- [ ] Add user preferences handling
- [ ] Set up user data sync with Clerk

### Set up basic usage tracking (2-3 days)

- [ ] Implement usage counters
- [ ] Add usage limits checking
- [ ] Set up usage reset schedule
- [ ] Implement usage notifications
- [ ] Add basic reporting functions

## ⚪ Stage 5: Billing & Subscriptions (2-3 weeks)

### Implement Paddle integration (3-5 days)

- [ ] Set up Paddle account and config
- [ ] Implement subscription webhooks
- [ ] Add payment flow integration
- [ ] Set up subscription plans
- [ ] Implement checkout process

### Connect subscription status to user accounts (2-3 days)

- [ ] Link subscriptions to users
- [ ] Handle subscription updates
- [ ] Implement plan changes
- [ ] Add subscription status checks

### Add usage limits/tracking (2-3 days)

- [ ] Implement plan-based limits
- [ ] Add upgrade prompts
- [ ] Set up usage warnings
- [ ] Implement grace period

## ⚪ Stage 6: Webflow Integration & Launch

Detailed plan available in [docs/webflow-integration-plan.md](./docs/webflow-integration-plan.md)

### Webflow App Registration (2-3 weeks)

- [ ] Register as a Webflow developer
- [ ] Create a Data Client App with OAuth support
- [ ] Set up proper scopes and permissions
- [ ] Implement secure authentication flow

### Automatic Trigger & Scheduling (3-4 weeks)

- [ ] Implement webhook subscription for the `site_publish` event
- [ ] Build secure endpoint to receive webhook POST requests
- [ ] Verify webhook signatures using `x-webflow-signature` headers
- [ ] Create configuration UI for scheduling options
- [ ] Implement cron-like scheduler for recurring runs

### Designer Extension UI (3-4 weeks)

- [ ] Develop Designer Extension with progress indicators
- [ ] Create error reporting interface
- [ ] Implement real-time updates via WebSocket or polling
- [ ] Build configuration panel for scheduler settings
- [ ] Add authentication between extension and server

### Launch & Documentation (1-2 weeks)

- [ ] Complete marketplace submission process
- [ ] Create user documentation and help resources
- [ ] Set up support channels
- [ ] Implement analytics for usage tracking
- [ ] Create onboarding flow for new users

## ⚪ Stage 7: Feature refinment & scaling

### Features

- [ ] Enable 'Don't treat query strings as unique URLs'
- [ ] Remove #anchor links as 'new page found' in find_links funtionality

### Clean up

- [ ] [docs/api-cleanup.md](./docs/api-cleanup.md)
- [ ] Database backup/recovery

### Task prioritisation

Rough idea here: [docs/task-prioritisation.md](./docs/task-prioritisation.md) - don't follow this precisely.

- [ ] Prioritisation of tasks by heirarchy in page
- [ ] Prioritisation of tasks at job level

---

## Key Risk Areas

### Immediate (Current Sprint)

- [ ] **Database security & backups** - Critical to prevent data loss
  - Mitigation: Implement automated backup schedule and periodic recovery testing

### Short-term (Next 1-2 Sprints)

- [ ] **Production performance under high concurrency**
- [ ] **PostgreSQL connection pooling optimisation**
- [ ] **Worker pool scaling**

### Medium-term (Future Sprints)

- [ ] **Deployment stability on Fly.io**
- [ ] **Auth integration complexity**
- [ ] **Batch processing error handling**

### Long-term (Future Stages)

- [ ] **Paddle webhook handling**
- [ ] **Webflow API limitations**
