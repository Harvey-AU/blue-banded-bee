# Roadmap

## ✅ Stage 0: Project Setup & Infrastructure

### ✅ Development Environment Setup

- [x] Initialise GitHub repository
- [x] Set up branch protection
- [x] Resolve naming issues and override branch protection for admins
- [x] Create dev/prod branches
- [x] Set up local development environment
- [x] Add initial documentation

### ✅ Go Project Structure

- [x] Initialise Go project
- [x] Set up dependency management
- [x] Create project structure
- [x] Add basic configs
- [x] Set up testing framework

### ✅ Production Infrastructure Setup

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

## ✅ Stage 1: Core Setup & Basic Crawling

### ✅ Core API Implementation

- [x] Initialise Go project structure and dependencies
- [x] Set up basic API endpoints
- [x] Set up environment variables and configs
- [x] Implement basic health checks and monitoring
- [x] Add basic error monitoring with Sentry
- [x] Set up endpoint performance tracking
- [x] Add graceful shutdown handling
- [x] Implement configuration validation

### ✅ Enhance Crawler Results

- [x] Set up Colly crawler configuration
- [x] Implement concurrent crawling logic
- [x] Add basic error handling
- [x] Add rate limiting (fixed client IP detection)
- [x] Add retry logic
- [x] Handle different response types/errors
- [x] Implement cache validation checks
- [x] Add crawler-specific error tracking
- [x] Set up crawler performance monitoring

### ✅ Set up Turso for storing results

- [x] Design database schema
- [x] Set up Turso connection and config
- [x] Implement data models and queries
- [x] Add basic error handling
- [x] Add retry logic
- [x] Add database performance monitoring
- [x] Set up query error tracking

## ✅ Stage 2: Multi-domain Support & Job Queue Architecture

### ✅ Job Queue Architecture

- [x] Design job and task data structures
- [x] Implement persistent job storage in database
- [x] Create worker pool for concurrent URL processing
- [x] Add job management API (create, start, cancel, status)
- [x] Implement database retry logic for job operations to handle transient errors
- [x] Enhance error reporting and monitoring

### ✅ Sitemap Integration

- [x] Implement sitemap.xml parser
- [x] Add URL filtering based on path patterns
- [x] Handle sitemap index files
- [x] Process multiple sitemaps
- [x] Implement robust URL normalisation in sitemap processing
- [x] Add improved error handling for malformed URLs

### ✅ Link Discovery & Crawling

- [x] Extract links from crawled pages
- [x] Filter links to stay within target domain
- [x] Basic link discovery logic
- [x] Queue discovered links for processing

### ✅ Job Management API

- [x] Create job endpoints (create/list/get/cancel)
- [x] Add progress calculation and reporting
- [x] Store recent crawled pages in job history
- [x] Implement multi-domain support

## ✅ Stage 3: PostgreSQL Migration & Performance Optimisation

### ✅ Fly.io Production Setup

- [x] Set up production environment on Fly.io
- [x] Deploy and test rate limiting in production
- [x] Configure auto-scaling rules
- [x] Set up production logging
- [x] Implement monitoring alerts
- [x] Configure backup strategies (Supabase handles automatically)

### ✅ Performance Optimisation

- [x] Implement caching layer
- [x] Optimise database queries
- [x] Configure rate limiting with proper client IP detection
- [x] Add performance monitoring
- [x] Made decision to switch to postgres at this point

### ✅ PostgreSQL Migration

#### ✅ PostgreSQL Setup and Infrastructure

- [x] Set up PostgreSQL on Fly.io
  - [x] Create database instance
  - [x] Configure connection settings
  - [x] Configure security settings

#### ✅ Database Layer Replacement

- [x] Implement PostgreSQL schema
  - [x] Convert SQLite schema to PostgreSQL syntax
  - [x] Add proper indexes
  - [x] Implement connection pooling
- [x] Replace database access layer
  - [x] Update db package to use PostgreSQL
  - [x] Add health checks and monitoring
  - [x] Implement efficient error handling

#### ✅ Task Queue and Worker Redesign

- [x] Implement PostgreSQL-based task queue
  - [x] Use row-level locking with SELECT FOR UPDATE SKIP LOCKED
  - [x] Optimise for concurrent access
  - [x] Plan task prioritisation implementation (docs created)
- [x] Redesign worker pool
  - [x] Create single global worker pool
  - [x] Implement optimised task acquisition

#### ✅ URL Processing Improvements

- [x] Enhanced sitemap processing
  - [x] Implement robust URL normalisation
  - [x] Add support for relative URLs in sitemaps
  - [x] Improve error handling for malformed URLs
- [x] Improve URL validation
  - [x] Better handling of URL variations
  - [x] Consistent URL formatting throughout the codebase

#### ✅ Code Refactoring

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

#### ✅ Code Cleanup

- [x] Remove redundant worker pool creation
  - [x] Eliminate duplicate worker pools in API handlers
  - [x] Ensure single global worker pool is used consistently
- [x] Simplify middleware stack
  - [x] Reduce excessive transaction monitoring
  - [x] Optimise Sentry integrations
  - [x] Remove unnecessary wrapping functions
- [x] Clean up API endpoints
  - [x] Document endpoints to consolidate or remove
  - [x] Plan endpoint implementation simplification
  - [x] Standardise error handling approach
  - [x] Implementation plan completed in [docs/plans/api-cleanup.md](docs/plans/api-cleanup.md)
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

#### ✅ Final Transition

- [x] Update core endpoints to use new implementation
- [x] Remove SQLite-specific code
- [x] Clean up dependencies and imports
- [x] Update configuration and documentation

## ✅ Stage 4: Core Authentication & MVP Interface

### ✅ Implement Supabase Authentication

- [x] Configure Supabase Auth settings
- [x] Implement JWT validation middleware in Go
- [x] Add social login providers configuration (Google, Facebook, Slack, GitHub, Microsoft, Figma, LinkedIn + Email)
- [x] Set up user session handling and token validation
- [x] Implement comprehensive auth error handling
- [x] Create user registration with auto-organisation creation
- [x] Configure custom domain authentication (auth.bluebandedbee.co)
- [x] Implement account linking for multiple auth providers per user

### ✅ Connect user data to PostgreSQL

- [x] Design user data schema with Row Level Security
- [x] Implement user profile storage
- [x] Add user preferences handling
- [x] Configure PostgreSQL policies for data access
- [x] Create database operations for users and organisations

### ✅ Simple Organisation Sharing

Detailed plan available in [docs/organisation-model.md](docs/plans/organisation-model.md)

- [x] Auto-create organisation when user signs up
- [x] Create shared access to all jobs/tasks/reports within organisation

### ✅ API-First Architecture Development (Completed v0.4.2)

- [x] **Comprehensive RESTful API Infrastructure**
  - [x] Standardised response format with request IDs and consistent error handling
  - [x] Interface-agnostic RESTful endpoints (`/v1/*` structure)
  - [x] Comprehensive middleware stack (CORS, logging, rate limiting)
  - [x] Proper HTTP status codes and structured error responses
- [x] **Multi-Interface Authentication Foundations**
  - [x] JWT-based authentication with Supabase integration
  - [x] Authentication middleware for protected endpoints

### ✅ MVP Interface Development (Completed v0.5.3)

- [x] **Dashboard Demonstration Infrastructure**
  - [x] Working vanilla JavaScript dashboard with modern UI design
  - [x] API integration for job statistics and progress tracking (`/v1/dashboard/stats`, `/v1/jobs`)
  - [x] Stable production deployment without Web Components dependencies
  - [x] Responsive design with professional styling and user experience
- [x] **Template + Data Binding Foundation**
  - [x] Architecture documentation for template-based integration approach
  - [x] Attribute-based event handling system (`bb-action`, `bb-data-*`)
  - [x] Event delegation framework for extensible functionality
  - [x] Demonstration of template approach in production dashboard

### ✅ Template + Data Binding Implementation (Completed v0.5.5)

- [x] **Core Data Binding Library**
  - [x] Basic attribute-based event handling (`bb-action="refresh-dashboard"`)
  - [x] JavaScript library for `data-bb-bind` attribute processing
  - [x] Template engine for `data-bb-template` repeated content
  - [x] Authentication integration with conditional element display (`data-bb-auth`)
  - [x] Form handling with `data-bb-form` and validation (`data-bb-validate`)
  - [x] Style and attribute binding (`data-bb-bind-style`, `data-bb-bind-attr`)
- [x] **Enhanced Job Management**
  - [x] Real-time job progress updates via data binding
  - [x] Job creation forms with template-based validation  
  - [x] Error handling and user feedback systems
  - [x] Advanced filtering and search capabilities
- [ ] **User Experience Features**
  - [ ] Account settings and profile management templates
  - [ ] Notification system integration
  - [ ] Performance analytics and trend visualisation
  - [ ] Responsive design compatibility testing

### 🔴 Task prioritisation & URL processing

- [ ] **Task Prioritisation**
  - [ ] Prioritisation by page hierarchy and importance - give pages ranking by no. of pages, i.e. 1700 page site, highest rank is 1700. all pages start at 0 rank, except homepage which is max (1700) give pages higher score based on if it's in the header or homepage, then each time it's in a page assign more rank
  - [ ] Job-level task prioritisation options

- [ ] **URL Processing Enhancements**
  - [ ] Option to treat query strings as same URL
  - [ ] Remove anchor links from link discovery
  - [ ] Path inclusion/exclusion rules

### 🔴 Webflow App Integration

- [ ] **Webflow Developer Registration**
  - [ ] Register as Webflow developer and create App
  - [ ] Set up OAuth integration with existing Supabase Auth system
- [ ] **Webflow App Development**
  - [ ] App opens within Webflow Designer interface
  - [ ] User login with existing Supabase Auth (no separate auth)
  - [ ] Show last crawl status for current Webflow site
  - [ ] "Crawl Now" button to trigger immediate cache warming
  - [ ] "Auto-crawl on publish" toggle for webhook setup
  - [ ] Webhook integration to trigger crawls on site publish

### 🔴 Slack Integration

- [ ] **Slack Application Development**
  - [ ] Create Slack app with slash commands (`/crawl sitedomain.com`)
  - [ ] Integrate with existing Supabase Auth system (no separate API keys)
  - [ ] Simple job creation workflow via slash commands
- [ ] **Threading & Progress Updates**
  - [ ] Create thread for each job with initial status
  - [ ] Post progress updates as thread replies
  - [ ] Final completion summary with link to main website
  - [ ] Error notifications with basic troubleshooting info
- [ ] **Commands & Help**
  - [ ] `/crawl [domain]` - Start cache warming job
  - [ ] `/crawl help` - Show available commands
  - [ ] Simple installation and setup documentation

## ⚪ Stage 5: Subscriptions & Monetisation

### 🔴 Payment Infrastructure

- [ ] **Paddle Integration**
  - [ ] Set up Paddle account and configuration
  - [ ] Implement subscription webhooks and payment flow
  - [ ] Create subscription plans and checkout process
- [ ] **Subscription Management**
  - [ ] Link subscriptions to organisations
  - [ ] Handle subscription updates and plan changes
  - [ ] Add subscription status checks
- [ ] **Usage Tracking & Quotas**
  - [ ] Implement usage counters and basic limits
  - [ ] Set up usage reporting functionality
  - [ ] Implement organisation-level usage quotas

## ⚪ Stage 6: Platform Optimisation & Advanced Features

### 🔴 Supabase Platform Integration

- [ ] **Real-time Features**
  - [ ] Replace polling with WebSocket subscriptions for live job progress
  - [ ] Live presence indicators for multi-user organisations
  - [ ] Real-time dashboard updates without page refresh
- [ ] **Database Optimisation**
  - [ ] Move CPU-intensive analytics queries to PostgreSQL functions
  - [ ] Optimise task acquisition with database-side logic
  - [ ] Enhance Row Level Security policies for multi-tenant usage
- [ ] **File Storage & Edge Functions**
  - [ ] Store crawler logs, sitemap caches, and error reports in Supabase Storage
  - [ ] Create Edge Functions for webhook handling and scheduled tasks
  - [ ] Handle Webflow publish events via Edge Functions

### 🔴 API & Integration Enhancements

- [ ] **API Client Libraries**
  - [ ] Enhance core JavaScript client with advanced authentication
  - [ ] Create interface-specific adapters
  - [ ] Document API with OpenAPI specification
- [ ] **Webhook System**
  - [ ] Implement webhook subscription for `site_publish` events
  - [ ] Verify webhook signatures using `x-webflow-signature` headers
  - [ ] Create webhook system for job completion notifications
- [ ] **API Key Management**
  - [ ] Create API key system for integrations
  - [ ] Implement scoped permissions for different interfaces

### 🔴 Infrastructure & Operations

- [ ] **Database Management**
  - [ ] Set up backup schedule and automated recovery testing
  - [ ] Implement data retention policies
  - [ ] Create comprehensive database health monitoring
- [ ] **Scheduling & Automation**
  - [ ] Create configuration UI for scheduling options
  - [ ] Implement cron-like scheduler for recurring runs
  - [ ] Automatic cache warming based on Webflow publish events

## ⚪ Stage 7: Feature Refinement & Launch Preparation

### 🔴 Security & Compliance

- [ ] **Enhanced Authentication**
  - [ ] Test and refine multi-provider account linking
  - [ ] Member invitation system for organisations
- [ ] **Audit & Security Features**
  - [ ] Login IP tracking and session limits
  - [ ] Active job limits per organisation
  - [ ] Audit logging for account changes and access history
  - [ ] GDPR compliance features (data export, deletion audit trails)
  - [ ] Suspicious activity detection and monitoring

### 🔴 Launch & Marketing

- [ ] **Marketing Infrastructure**
  - [ ] Simple Webflow marketing page with product explanation
  - [ ] Basic navigation structure and call-to-action
  - [ ] User documentation and help resources
- [ ] **Launch Preparation**
  - [ ] Complete marketplace submission process
  - [ ] Set up support channels and user onboarding
  - [ ] Implement usage analytics and tracking
