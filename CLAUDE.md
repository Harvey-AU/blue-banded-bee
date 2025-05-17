# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Note:** Please write in British (Australian) English, not American English.

## Project Initialisation

Before beginning any work on this project, please first read and understand the complete initialization guide:

[INIT.md](./INIT.md)

All work should follow the guidelines, workflow, and standards outlined in INIT.md.

## Quick Commands

### Development

```bash
# Run the service with hot reloading
air

# Run the service without hot reloading
go run ./cmd/app/main.go

# Build the Docker container
docker build -t blue-banded-bee .

# Run the Docker container
docker run -p 8080:8080 --env-file .env blue-banded-bee
```

### Testing

```bash
# Run all tests
go test ./... -v

# Run integration tests
RUN_INTEGRATION_TESTS=true go test ./... -v

# Check test coverage
go test ./... -cover
```

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

   - Uses a normalized schema with reference tables
   - Employs row-level locking with FOR UPDATE SKIP LOCKED
   - Optimized connection pooling (25 max open, 10 max idle)

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

   - Normalized schema with domain, page, job, and task tables
   - Tasks reference domains and pages for efficient storage
   - PostgreSQL-specific features like FOR UPDATE SKIP LOCKED

3. **Request Processing**
   - Token bucket rate limiting (5 requests/second default)
   - IP-based rate limiting with proxy support
   - Configurable concurrency and depth settings

## Code Organisation

- `cmd/app/` - Main application entry point
- `cmd/test_jobs/` - Job queue testing utility
- `internal/common/` - Shared utilities
- `internal/crawler/` - Web crawling functionality
- `internal/db/` - Database access layer
- `internal/jobs/` - Job queue and worker implementation

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

### Preserve Functionality

- Never remove or modify existing functionality without explicit permission
- Propose changes in an additive manner
- Maintain backward compatibility unless explicitly directed otherwise

### Documentation Maintenance

- Update documentation immediately after any code changes
- Document new learnings, insights, or discovered edge cases
- Maintain proper documentation hierarchy under `docs/`

### Code Investigation Workflow

1. First, locate and read relevant configuration files
2. Second, check actual code implementation of related functionality
3. Only after steps 1-2, formulate a response based on evidence
4. When debugging, always show findings from relevant files first
