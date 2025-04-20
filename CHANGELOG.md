# Changelog

All notable changes to the Blue Banded Bee project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] – 2025-04-21

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
