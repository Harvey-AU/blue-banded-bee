# Roadmap

## Current Status Overview

- PostgreSQL migration completed (now using Supabase after briefly using Postgres in Fly)
- Successfully deployed to Fly.io with verified functionality
- Local development environment established with PostgreSQL
- **Production system fully operational** - processing jobs, tasks and recording results successfully
- Worker pool using PostgreSQL's row-level locking implemented and stable
- Enhanced sitemap processing with improved URL handling and normalisation
- Improved link discovery and URL validation across the codebase
- Completed major code refactoring to improve architecture and maintainability
- Removed unnecessary depth functionality from the codebase
- **Fixed critical production database schema mismatch** (v0.3.8) - task insertion now working properly
- **Complete Supabase Authentication System** (v0.4.0) - Multi-tenant auth with 8 social providers, custom domain, protected endpoints

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

#### ✅ Final Transition

- [x] Update core endpoints to use new implementation
- [x] Remove SQLite-specific code
- [x] Clean up dependencies and imports
- [x] Update configuration and documentation

## ⚪ Stage 4: Core Authentication & MVP Interface

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

### 🔴 API-First Architecture Development (High Priority)

Detailed plan available in [docs/multi-interface-architecture.md](docs/architecture/multi-interface-architecture.md)

- [ ] Design comprehensive API for all interfaces
  - [ ] Define standard response format
  - [ ] Document API with OpenAPI specification
  - [ ] Design interface-agnostic endpoints
  - [ ] Create webhook system for notifications
- [ ] Implement multi-interface authentication foundations
  - [ ] Design JWT-based authentication structure
  - [ ] Create API key system for integrations
  - [ ] Plan OAuth support foundation
- [ ] Develop API client library core
  - [ ] Create core JavaScript client for API interaction
  - [ ] Implement basic authentication handling
  - [ ] Design extensible client architecture

### 🔴 MVP Interface Development (High Priority)

Detailed plan available in [docs/ui-architecture.md](docs/plans/ui-architecture.md)

- [ ] Develop proof-of-concept web application
  - [ ] Create component-based architecture with HTML Custom Elements
  - [ ] Build minimal dashboard for job management
  - [ ] Implement basic job creation form
  - [ ] Create simple job results view with core metrics
- [ ] Interface for creating jobs and getting job statuses in Slack and Webflow app
  - [ ] Integrate Supabase Auth with interface
  - [ ] Implement login/signup functionality
  - [ ] Create session management
  - [ ] Set up secure authentication flow

### 🔴 Marketing Interface (Medium Priority)

- [ ] Create simple Webflow marketing page
  - [ ] Design minimal home page with product explanation
  - [ ] Set up basic navigation structure
  - [ ] Implement call-to-action for early access

### 🔴 Authentication Refinement (Low Priority)

- [ ] **Test and refine multi-provider account linking** - Verify whether same email across different auth providers (Google, Facebook, etc.) creates linked accounts or separate users. Important for user experience but not urgent.

### 🔴 Critical Infrastructure (Low Priority)

- [ ] Set up database backup schedule and automated recovery testing - Essential for production safety
- [ ] Implement member invitation system - Core functionality gap in organisation sharing

### 🔴 Set up basic usage tracking

- [ ] Implement usage counters
- [ ] Add basic limits
- [ ] Set up usage reporting functionality

## ⚪ Stage 5: Billing & Expanded Interface

### 🔴 Implement Paddle integration

- [ ] Set up Paddle account and config
- [ ] Implement subscription webhooks
- [ ] Add payment flow integration
- [ ] Set up subscription plans
- [ ] Implement checkout process

### 🔴 Connect subscription status to organisations

- [ ] Link subscriptions to organisations
- [ ] Handle subscription updates
- [ ] Implement plan changes
- [ ] Add subscription status checks
- [ ] Implement simple organisation-level usage quotas

### 🔴 Complete User Interface Development

- [ ] Expand Webflow site for full marketing
  - [ ] Design and build complete marketing pages (features, pricing, etc.)
  - [ ] Enhance dashboard layout and user account pages
  - [ ] Implement Webflow Memberships integration
- [ ] Develop simple organisation interface
  - [ ] Add member invitation and management
  - [ ] Create organisation dashboard with shared resources
  - [ ] Add organisation-level reporting
- [ ] Enhance embedded JavaScript application
  - [ ] Expand job creation interface with advanced options
  - [ ] Create comprehensive job results visualisation
  - [ ] Implement account settings and profile management
  - [ ] Develop notification system for completed jobs
- [ ] Add usage limits/tracking
  - [ ] Implement plan-based limits
  - [ ] Add upgrade prompts
  - [ ] Set up usage warnings
  - [ ] Implement grace period

## ⚪ Stage 6: Multi-Interface Expansion & Launch

Detailed plan available in [docs/webflow-integration-plan.md](docs/plans/webflow-integration-plan.md) and [docs/multi-interface-architecture.md](docs/architecture/multi-interface-architecture.md)

### 🔴 Complete API Client Libraries

- [ ] Extend API client library for multiple interfaces
  - [ ] Enhance core JavaScript client
  - [ ] Create interface-specific adapters
  - [ ] Implement advanced authentication handling

### 🔴 Webflow Designer Extension

- [ ] Register as a Webflow developer
- [ ] Create a Data Client App with OAuth support
- [ ] Set up proper scopes and permissions
- [ ] Develop Designer Extension with progress indicators
- [ ] Create error reporting interface
- [ ] Implement real-time updates via API client library
- [ ] Build configuration panel for scheduler settings
- [ ] Implement OAuth authentication for extension-server communication

#### 🔴 Critical Database Management (Priority)

- [ ] Set up backup schedule and automated recovery testing
- [ ] Implement data retention policies
- [ ] Create monitoring for database health

### 🔴 Automatic Trigger & Scheduling

- [ ] Implement webhook subscription for the `site_publish` event
- [ ] Build secure endpoint to receive webhook POST requests
- [ ] Verify webhook signatures using `x-webflow-signature` headers
- [ ] Create configuration UI for scheduling options
- [ ] Implement cron-like scheduler for recurring runs

### 🔴 Slack Integration

- [ ] Create Slack app and configure commands
- [ ] Implement API key authentication for Slack integration
- [ ] Develop interactive message components
- [ ] Set up webhook notifications for job events
- [ ] Create documentation for Slack app installation

### 🔴 Launch & Documentation

- [ ] Complete marketplace submission process
- [ ] Create user documentation and help resources
- [ ] Set up support channels
- [ ] Implement analytics for usage tracking
- [ ] Create onboarding flow for new users

## ⚪ Stage 7: Feature Refinement & Scaling

### 🔴 Features

- [ ] Enable 'Don't treat query strings as unique URLs'
- [ ] Remove #anchor links as 'new page found' in find_links functionality

### 🔴 Security & Audit Enhancements

- [ ] **Login IP tracking** - Record IP addresses for all authentication events
- [ ] **Session limits** - Implement concurrent session limits per user account
- [ ] **Active job limits** - Prevent organisations from overwhelming system with excessive jobs
- [ ] **Audit logging system** - Track login history, account changes, password resets
- [ ] **Suspicious activity detection** - Monitor for unusual access patterns
- [ ] **Compliance features** - GDPR data export, account deletion audit trails

### 🔴 Clean up

- [ ] [docs/api-cleanup.md](docs/plans/api-cleanup.md)
- [ ] Database backup/recovery

### 🔴 Task prioritisation

Rough idea here: [docs/task-prioritisation.md](docs/plans/task-prioritisation.md) - don't follow this precisely.

- [ ] Prioritisation of tasks by hierarchy in page
- [ ] Prioritisation of tasks at job level

### 🔴 Supabase Advanced Integration

Detailed plan available in [docs/supabase-integration-strategy.md](./docs/supabase-integration-strategy.md)

- [ ] Implement PostgreSQL functions for core operations (task acquisition, job progress)
- [ ] Create database triggers for automated state management
- [ ] Set up Supabase Realtime for job/task monitoring
- [ ] Implement Row Level Security policies for multi-tenant usage
- [ ] Create Edge Functions for webhook handling and scheduled tasks
- [ ] Optimise database interactions with native PostgreSQL features

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
