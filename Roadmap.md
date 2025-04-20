# Roadmap

- We've just moved to postgres and also using supabase (after briefly using postgres in Fly)
- The main-archive.go.bak contains the original logic/setup for the app pre-postgres
- We've successfully deployed to Fly.io and verified functionality
- Local development works with PostgreSQL running on localhost
- It is running jobs, tasks and recording results
- Before proceeding with furthur functionality/etc, we want to finalise the data schema to allow for separatation of simple tables with references: site (for each domain), page (for each page being crawled), results (To store actual results), tasks to only have task status in them.
- We need to implement a more robust worker pool using PostgreSQL's row-level locking

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

## 🚧 Stage 2: Multi-domain Support & Job Queue Architecture 

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

### Link Discovery & Crawling (2-3 hrs) 

- [x] Extract links from crawled pages
- [x] Filter links to stay within target domain
- [x] Add depth control for crawling
- [ ] Queue discovered links for processing
- [ ] Wire DB `depth` column into enqueue logic for per-task depth control

### Job Management API (1-2 hrs) 

- [x] Create job endpoints (create/list/get/cancel)
- [x] Add progress calculation and reporting
- [x] Store recent crawled pages in job history
- [x] Implement multi-domain support

## 🚧 Stage 3: Deployment & Monitoring (8-12 hrs) 

### Fly.io Production Setup (4-6 hrs) 

- [x] Set up production environment on Fly.io
- [x] Deploy and test rate limiting in production
- [x] Configure auto-scaling rules
- [x] Set up production logging
- [x] Implement monitoring alerts
- [ ] Configure backup strategies

### Performance Optimization (4-6 hrs) 

- [x] Implement caching layer
- [x] Optimize database queries
- [x] Configure rate limiting with proper client IP detection
- [x] Add performance monitoring
- [x] Made decision to switch to postgres at this point

### PostgreSQL Migration (10-15 hrs) 

#### PostgreSQL Setup and Infrastructure (2-3 hrs) 

- [x] Set up PostgreSQL on Fly.io
  - [x] Create database instance
  - [x] Configure connection settings
  - [ ] Set up backup schedule
  - [x] Configure security settings

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
  - [ ] Optimize for concurrent access
  - [ ] Add task prioritization
- [ ] Redesign worker pool
  - [ ] Create single global worker pool
  - [ ] Implement optimized task acquisition
  - [ ] Add proper worker scaling

#### Batch Processing Implementation (2-3 hrs)

- [ ] Create efficient batch operations
  - [ ] Use bulk inserts for results
  - [ ] Implement configurable batch sizes
  - [ ] Add proper error handling for batches

#### Code Cleanup (2-3 hrs)

- [ ] Remove redundant worker pool creation
  - [ ] Eliminate duplicate worker pools in API handlers
  - [ ] Ensure single global worker pool is used consistently
- [ ] Simplify middleware stack
  - [ ] Reduce excessive transaction monitoring
  - [ ] Optimize Sentry integrations
  - [ ] Remove unnecessary wrapping functions
- [ ] Clean up API endpoints
  - [ ] Consolidate or remove debug/test endpoints
  - [ ] Simplify endpoint implementations
  - [ ] Standardize error handling
- [ ] Fix metrics collection
  - [ ] Implement proper metrics exposure
  - [ ] Remove unused metrics tracking
  - [ ] Add relevant PostgreSQL metrics

#### Final Transition (1-2 hrs) 

- [x] Update core endpoints to use new implementation
- [ ] Remove SQLite-specific code
- [ ] Clean up dependencies and imports
- [ ] Update configuration and documentation

## ⚪ Stage 4: Auth & User Management (10-16 hrs)

### Implement Clerk authentication (4-6 hrs)

- [ ] Set up Clerk project configuration
- [ ] Implement auth middleware
- [ ] Add social login providers
- [ ] Set up user session handling
- [ ] Implement auth error handling

### Connect user data to PostgreSQL (2-4 hrs)

- [ ] Design user data schema
- [ ] Implement user profile storage
- [ ] Add user preferences handling
- [ ] Set up user data sync with Clerk

### Set up basic usage tracking (4-6 hrs)

- [ ] Implement usage counters
- [ ] Add usage limits checking
- [ ] Set up usage reset schedule
- [ ] Implement usage notifications
- [ ] Add basic reporting functions

## ⚪ Stage 5: Billing & Subscriptions (8-12 hrs)

### Implement Paddle integration (4-6 hrs)

- [ ] Set up Paddle account and config
- [ ] Implement subscription webhooks
- [ ] Add payment flow integration
- [ ] Set up subscription plans
- [ ] Implement checkout process

### Connect subscription status to user accounts (2-3 hrs)

- [ ] Link subscriptions to users
- [ ] Handle subscription updates
- [ ] Implement plan changes
- [ ] Add subscription status checks

### Add usage limits/tracking (2-3 hrs)

- [ ] Implement plan-based limits
- [ ] Add upgrade prompts
- [ ] Set up usage warnings
- [ ] Implement grace period

## ⚪ Stage 6: Webflow Integration & Launch (8-16 hrs)

### Build Webflow frontend interface (4-8 hrs)

- [ ] Design API integration points
- [ ] Create user dashboard
- [ ] Implement results display
- [ ] Add usage statistics display
- [ ] Create settings interface

### Connect to backend APIs (3-5 hrs)

- [ ] Implement API calls
- [ ] Add error handling
- [ ] Set up response handling
- [ ] Implement loading states
- [ ] Add retry logic

### Set up monitoring (GA) (1-3 hrs)

- [ ] Configure GA tracking
- [ ] Add custom events
- [ ] Set up conversion tracking
- [ ] Implement error tracking
- [ ] Create basic dashboards

---

## What We've Done, In Progress, and Next Steps

### What We've Accomplished

- Successfully migrated from SQLite/Turso to PostgreSQL for core functionality
- Set up PostgreSQL database on Fly.io with proper connection settings
- Implemented schema design optimized for PostgreSQL with proper indexes
- Created basic queue implementation with row-level locking (FOR UPDATE SKIP LOCKED)
- Implemented core APIs to work with PostgreSQL (/health, /pg-health, /test-crawl, /recent-crawls)
- Deployed and verified working on Fly.io production environment

### Current State

- We now have a working basic implementation with PostgreSQL that can:
  - Store and retrieve crawl results
  - Handle database errors properly
  - Connect to production PostgreSQL instance
  - Reset database schema when needed
- The implementation provides the foundation for the full worker/queue system

### Next Steps (in priority order)

1. Implement the full worker pool with PostgreSQL:

   - Create optimized task processing with batching
   - Implement proper concurrency control
   - Add database connection pooling optimization

2. Add job management functionality:

   - Implement create/list/get/cancel operations for jobs
   - Implement task status tracking
   - Add progress calculation

3. Implement efficient batch processing:

   - Use bulk inserts for storing results
   - Add configurable batch sizes
   - Implement proper error handling

4. Clean up code by:

   - Removing redundant worker pool creation
   - Simplifying middleware stack
   - Standardizing error handling
   - Removing SQLite-specific code

5. Run performance tests:
   - Test with high concurrency
   - Measure database performance
   - Verify scaling behavior

---

## Key Risk Areas:

- [ ] Production performance under high concurrency
- [ ] PostgreSQL connection pooling optimization
- [ ] Worker pool scaling
- [ ] Batch processing error handling
- [ ] Deployment stability on Fly.io
- [ ] Auth integration complexity
- [ ] Paddle webhook handling
- [ ] Webflow API limitations
