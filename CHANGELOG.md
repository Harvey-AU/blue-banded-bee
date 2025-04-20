# Changelog

All notable changes to the Blue Banded Bee project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2025-04-20

### Changed

- **Major Database Migration**: Fully migrated from SQLite/Turso to PostgreSQL
  - Removed all SQLite dependencies including `github.com/tursodatabase/libsql-client-go`
  - Reorganized database code structure, moving from `internal/db/postgres` to `internal/db`
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
