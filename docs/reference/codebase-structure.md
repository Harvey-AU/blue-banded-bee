# Blue Banded Bee Codebase Structure

This document provides a comprehensive overview of the Blue Banded Bee codebase structure, including directories, key files, and important functions. It serves as a reference for both the developer and AI assistant during collaboration.

## Table of Contents

- [Blue Banded Bee Codebase Structure](#blue-banded-bee-codebase-structure)
  - [Table of Contents](#table-of-contents)
  - [Root Directory](#root-directory)
    - [Core Files](#core-files)
    - [Main Directories](#main-directories)
  - [CMD](#cmd)
    - [cmd/app](#cmdapp)
    - [cmd/pg-test](#cmdpg-test)
    - [cmd/test\_jobs](#cmdtest_jobs)
  - [INTERNAL](#internal)
    - [internal/common](#internalcommon)
    - [internal/crawler](#internalcrawler)
    - [internal/db](#internaldb)
    - [internal/jobs](#internaljobs)
  - [DOCS](#docs)
    - [docs/architecture](#docsarchitecture)
    - [docs/guides](#docsguides)
    - [docs/reference](#docsreference)
  - [.GITHUB](#github)
    - [.github/ISSUE\_TEMPLATE](#githubissue_template)
    - [.github/workflows](#githubworkflows)

## Root Directory

The root directory contains configuration files, documentation, and the main project directories.

### Core Files

- `go.mod` - Go module definition and dependencies
- `go.sum` - Checksums for Go dependencies
- `Dockerfile` - Instructions for building the project's Docker image
- `fly.toml` - Configuration for Fly.io deployment
- `.env.example` - Template for environment variables

### Main Directories

- `cmd/` - Application entry points and command-line tools
- `internal/` - Internal application code and business logic
- `docs/` - Project documentation

## CMD

The `cmd` directory contains the application entry points and command-line tools.

### cmd/app

This is the main application entry point for the Blue Banded Bee service.

- `main.go` - The primary entry point for the application

  - `main()` - Initialises and runs the service
  - `Config` struct - Application configuration from environment variables
  - `RateLimiter` - IP-based rate limiting system
  - `IPRateLimiter` - Per-IP rate limiting implementation
  - `setupLogging()` - Configures the logging system
  - `getEnvWithDefault()` - Retrieves environment variables with fallbacks
  - `getClientIP()` - Extracts client IP from requests

- `main_test.go` - Tests for the main application

### cmd/pg-test

PostgreSQL integration test utility.

- `main.go` - Test utility for PostgreSQL database integration
  - Tests database operations and schema

### cmd/test_jobs

Job queue testing utility.

- `main.go` - Test utility for job queue functionality
  - Creates and processes test jobs

## INTERNAL

The `internal` directory contains the core business logic and implementation of the Blue Banded Bee service.

### internal/common

Common utilities and shared components.

- `queue.go` - Database operation queue implementation
  - `DbQueue` struct - Serializes database operations through workers
  - `DbOperation` struct - Represents a database operation to be executed
  - `QueueProvider` interface - For accessing a DB queue
  - `Execute()` - Adds an operation to the queue and waits for completion
  - `processOperations()` - Handles database operations sequentially

### internal/crawler

Web crawling and site mapping functionality.

- `crawler.go` - Core crawler implementation
  - Handles crawling websites and warming caches
  - HTTP request and response handling
- `sitemap.go` - Sitemap parsing and processing
- `config.go` - Configuration for the crawler
- `types.go` - Type definitions for the crawler

### internal/db

Database access and management.

- `db.go` - Database connection and query functionality
  - Database initialization and connection management
  - SQL query execution and transaction handling
- `worker.go` - Database worker implementation
- `queue.go` - Database operation queue implementation
- `health.go` - Database health checking

### internal/jobs

Job queue and background processing.

- `worker.go` - Worker implementation for processing jobs
  - Job execution and error handling
  - Retry logic and job completion
- `manager.go` - Job queue manager
  - Manages job scheduling and execution
- `db.go` - Database operations for jobs
  - Job storage and retrieval
  - Job status tracking
- `queue_helpers.go` - Helper functions for job queues
- `types.go` - Type definitions for jobs

## DOCS

The `docs` directory contains all project documentation organised by category.

### docs/architecture

Architecture and design documentation.

- `mental-model.md` - Conceptual overview and architectural insights
- `implementation-details.md` - Technical implementation details and optimisations
- `gotchas.md` - Known issues, edge cases, and their solutions
- `quick-reference.md` - Quick reference for parameters and configurations

### docs/guides

User and developer guides.

- `deployment.md` - Instructions for deploying the application
- `development.md` - Guide for local development and setup

### docs/reference

Reference documentation.

- `api-reference.md` - API endpoints and usage documentation
- `database-config.md` - Database configuration and schema documentation
- `codebase-structure.md` - This document, providing an overview of the codebase structure

## .GITHUB

The `.github` directory contains GitHub-specific configuration files.

### .github/ISSUE_TEMPLATE

Templates for GitHub issues.

- `bug_report.md` - Template for reporting bugs
- `feature_request.md` - Template for requesting new features

### .github/workflows

GitHub Actions workflow definitions.

- `fly-deploy.yml` - Workflow for deploying to Fly.io
