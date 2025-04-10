## Stage 0: Project Setup & Infrastructure (6-10 hrs) ✅

### Development Environment Setup (2-3 hrs) ✅

- [x] Initialise GitHub repository
- [x] Set up branch protection
- [x] Resolve naming issues and override branch protection for admins
- [x] Create dev/prod branches
- [x] Set up local development environment
- [x] Add initial documentation

### Go Project Structure (2-3 hrs) ✅

- [x] Initialise Go project
- [x] Set up dependency management
- [x] Create project structure
- [x] Add basic configs
- [x] Set up testing framework

### Production Infrastructure Setup (2-4 hrs) ✅

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

## Stage 1: Core Setup & Basic Crawling (15-25 hrs) 🟡

### Core API Implementation (3-5 hrs) ✅

- [x] Initialise Go project structure and dependencies
- [x] Set up basic API endpoints
- [x] Set up environment variables and configs
- [x] Implement basic health checks and monitoring
- [x] Add basic error monitoring with Sentry
- [x] Set up endpoint performance tracking
- [x] Add graceful shutdown handling
- [x] Implement configuration validation

### Enhance Crawler Results (8-12 hrs) ✅

- [x] Set up Colly crawler configuration
- [x] Implement concurrent crawling logic
- [x] Add basic error handling
- [x] Add rate limiting (fixed client IP detection)
- [x] Add retry logic
- [x] Handle different response types/errors
- [x] Implement cache validation checks
- [x] Add crawler-specific error tracking
- [x] Set up crawler performance monitoring

### Set up Turso for storing results (4-8 hrs) ✅

- [x] Design database schema
- [x] Set up Turso connection and config
- [x] Implement data models and queries
- [x] Add basic error handling
- [x] Add retry logic
- [x] Add database performance monitoring
- [x] Set up query error tracking

## Stage 2: Multi-domain Support & Job Queue Architecture

### Job Queue Architecture

- [x] Design job and task data structures
- [x] Implement persistent job storage in database
- [x] Create worker pool for concurrent URL processing
- [x] Add job management API (create, start, cancel, status)
- [x] Implement database retry logic for job operations to handle transient errors
- [ ] Enhance error reporting and monitoring

### Sitemap Integration (2-3 hrs)

- [ ] Implement sitemap.xml parser
- [ ] Add URL filtering based on path patterns
- [ ] Handle sitemap index files
- [ ] Process multiple sitemaps
- [ ] Feed discovered URLs into job queue

### Link Discovery & Crawling (2-3 hrs)

- [ ] Extract links from crawled pages
- [ ] Filter links to stay within target domain
- [ ] Add depth control for crawling
- [ ] Queue discovered links for processing

### Job Management API (1-2 hrs)

- [ ] Create job endpoints (create/list/get/cancel)
- [ ] Add progress calculation and reporting
- [ ] Store recent crawled pages in job history
- [ ] Implement multi-domain support

## Stage 3: Deployment & Monitoring (8-12 hrs)

### Fly.io Production Setup (4-6 hrs)

- [x] Set up production environment on Fly.io
- [x] Deploy and test rate limiting in production
- [ ] Configure auto-scaling rules
- [ ] Set up production logging
- [ ] Implement monitoring alerts
- [ ] Configure backup strategies

### Performance Optimization (4-6 hrs)

- [ ] Implement caching layer
- [ ] Optimize database queries
- [ ] Set up CDN for static assets
- [x] Configure rate limiting with proper client IP detection
- [ ] Add performance monitoring
- [ ] Implement database maintenance routines:
  - [ ] Data archiving for older records
  - [ ] Database index optimization
  - [ ] Periodic VACUUM operations
  - [ ] Duplicate record detection and cleanup
  - [ ] Error and orphaned data cleanup

## Stage 4: Auth & User Management (10-16 hrs)

### Implement Clerk authentication (4-6 hrs)

- [ ] Set up Clerk project configuration
- [ ] Implement auth middleware
- [ ] Add social login providers
- [ ] Set up user session handling
- [ ] Implement auth error handling

### Connect user data to Turso (2-4 hrs)

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

## Stage 5: Billing & Subscriptions (8-12 hrs)

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

## Stage 6: Webflow Integration & Launch (8-16 hrs)

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

## Key Risk Areas:

- [ ] Crawler edge cases and error handling
- [ ] Production deployment stability on Fly.io
- [ ] Multi-domain job management and resource utilisation
- [ ] Auth integration complexity
- [ ] Paddle webhook handling
- [ ] Webflow API limitations
- [ ] Performance under load
