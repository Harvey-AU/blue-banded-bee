# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**VERY IMPORTANT:** Please write in British (Australian) English, not American English.

## Project Initialisation

Before beginning any work on this project, please first read and understand the complete initialisation guide:

[INIT.md](./INIT.md)

All work should follow the guidelines, workflow, and standards outlined in INIT.md.

## Quick Commands

### Debugging

```bash
# Enable debug mode by setting in .env
DEBUG=true
LOG_LEVEL=debug
```

## Architecture Overview

Blue Banded Bee is a web cache warming service built in Go, primarily focused on Webflow sites. It follows a worker pool architecture for efficient URL crawling and cache warming.

### Core Components

1. **Worker Pool System**

   - Concurrent task processing with multiple workers
   - Job management that breaks down jobs into tasks
   - Automatic recovery of stalled or failed tasks

2. **Database Layer (PostgreSQL)**

   - Uses a normalised schema with reference tables
   - Employs row-level locking with FOR UPDATE SKIP LOCKED
   - Optimised connection pooling (25 max open, 10 max idle)

3. **Crawler System**

   - Concurrent URL processing with rate limiting
   - Cache validation and performance monitoring
   - Response time and cache status tracking

4. **Job Queue**
   - Persistent job and task tracking
   - Progress monitoring and statistics
   - Recovery mechanisms for failed tasks

### Key Concepts

1. **Jobs and Tasks**

   - A Job is a collection of URLs from a single domain to be crawled
   - Tasks are individual URL processing units within a job
   - Workers concurrently process tasks from the queue

2. **Database Schema**

   - Normalised schema with domain, page, job, and task tables
   - Tasks reference domains and pages for efficient storage
   - PostgreSQL-specific features like FOR UPDATE SKIP LOCKED

3. **Request Processing**
   - Token bucket rate limiting (5 requests/second default)
   - IP-based rate limiting with proxy support
   - Configurable concurrency and depth settings

## Code Organisation

- `cmd/app/` - Main application entry point
- `cmd/test_jobs/` - Job queue testing utility
- `internal/api/` - HTTP handlers and middleware
- `internal/auth/` - Authentication logic
- `internal/crawler/` - Web crawling functionality
- `internal/db/` - Database access layer
- `internal/jobs/` - Job queue and worker implementation
- `internal/util/` - Shared utilities
- `web/` - Frontend Web Components for Webflow integration
  - `web/src/` - Component source code
  - `web/dist/` - Built components (served via `/js/` endpoint)
  - `web/examples/` - Integration examples and documentation

## Development Notes

1. **Database Configuration**

   - PostgreSQL connection settings are managed through environment variables
   - Connection pooling is preconfigured with optimal settings
   - Always use PostgreSQL-style numbered parameters ($1, $2) in queries

2. **Worker Configuration**

   - Worker pools are configurable through environment variables
   - Default: 5 concurrent workers, 1-minute recovery interval

3. **Testing Approach**

   - Unit tests are in \*\_test.go files next to implementation
   - Integration tests require RUN_INTEGRATION_TESTS=true flag
   - Use test job utility for job queue testing

4. **Error Handling**
   - Structured logging with zerolog
   - Sentry integration for error tracking
   - Graduated retry with exponential backoff

## Standards & Workflow

### Documentation First

- Review relevant documentation before proposing or making changes
- Consider documentation the source of truth for design decisions
- When building new functionality, propose architecture within the codebase and database for my review before proceeding.
- Always check if existing documentation has a technical outline for the requested task.

### Preserve Functionality

- Never remove or modify existing functionality without explicit permission
- When working on adjacent/related features, question if existing code is necessary/valuable
- Propose changes in an additive manner unless removal is justified
- Maintain backward compatibility unless explicitly directed otherwise

### Documentation Maintenance

- Update documentation immediately after any code changes
- When making architectural recommendations, incorporate them into existing docs (Roadmap, Architecture)
- Document new learnings, insights, or discovered edge cases
- Maintain proper documentation hierarchy under `docs/`

### Code Investigation Workflow

1. First, locate and read relevant configuration files
2. Second, check actual code implementation of related functionality
3. Only after steps 1-2, formulate a response based on evidence
4. When debugging, always show findings from relevant files first

### Git and Version Control Policy

**Git operations are allowed and encouraged**

- Use `git add`, `git commit`, and `git push` to deploy changes
- Keep commit messages simple: 5-6 words, no AI generation references
- Deploy via GitHub Actions (push to GitHub, not direct `fly deploy`)
- Use the established commit message format without Claude attribution

## Communication & Problem-Solving

### Communication Style

- **Always keep explanations concise and direct** - avoid lengthy technical justifications
- **Explain the "why" behind technical decisions** when asked
- **Ask before assuming** - explain what's happening and why it's appropriate
- **Fix root causes** rather than working around problems with additional complexity

### Tech Stack Leverage

**Consider all capabilities of the current tech stack before building new code:**

- **Supabase**: Auth, real-time database, edge functions, file storage, database functions
- **Sentry**: Error tracking, performance monitoring, alerting
- **Go/Fly.io**: Core application logic, worker pools, scaling
- **PostgreSQL**: Database functions, triggers, row-level security

**Always propose multiple solution options** explaining trade-offs between custom code vs existing platform capabilities.

### Problem-Solving Approach

- **Don't overcomplicate solutions** - prefer simple, direct fixes
- **Address actual problems** rather than creating workarounds
- **Use incremental solutions** that can be understood step-by-step
- **Question existing code** when working on related features - don't assume everything is necessary
