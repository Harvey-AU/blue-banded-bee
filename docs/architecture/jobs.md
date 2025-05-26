# Job Queue System

This package implements a robust job queue system for the crawler with the following features:

## Components

- **Job Management**: Create, start, pause, and cancel crawling jobs
- **Task Queue**: Persistent queue of URLs to be processed
- **Worker Pool**: Concurrent workers that process URLs from the queue
- **Status Tracking**: Monitor job progress and task status
- **Error Handling**: Retry logic for both HTTP requests and database operations

## Database Schema

The system uses a normalised database schema with reference tables:

- **domains**: Stores unique domain names with integer primary keys
- **pages**: Stores page paths with references to their respective domains
- **jobs**: Stores job metadata, status, and progress with references to domains, and tracks separate task counts:
  - `sitemap_tasks`: Number of tasks seeded from sitemaps
  - `found_tasks`: Number of tasks discovered via link extraction
  - `total_tasks`: Combined task count (`sitemap_tasks` + `found_tasks`)
- **tasks**: Stores individual crawl tasks with references to pages

## URL Processing

Instead of storing complete URLs in the tasks table, the system now:

1. Stores domain names in the `domains` table
2. Stores page paths in the `pages` table
3. References these tables from `jobs` and `tasks`
4. Reconstructs full URLs when processing tasks by joining domain names and paths

This approach improves data integrity and reduces redundancy by storing domain names and page paths only once.

## PostgreSQL Compatibility

All SQL queries use PostgreSQL-style numbered parameters:

```go
// Correct parameter style for PostgreSQL
tx.ExecContext(ctx, `UPDATE tasks SET status = $1 WHERE id = $2`, status, id)

// Batch operations use incrementing parameter numbers
placeholders := fmt.Sprintf("($%d, $%d, $%d)", paramIndex, paramIndex+1, paramIndex+2)
```

## Reliability Features

- **Database Retry Logic**: Handles transient SQLite lock errors with exponential backoff
- **Atomic Task Claiming**: Prevents race conditions where multiple workers process the same task
- **Job Progress Tracking**: Updates job statistics as tasks complete
- **Performance Monitoring**: Tracks response times and error rates

## Usage

To use the job system:

1. Create a job with domain and options
2. Start the job to begin processing
3. Monitor job status until completion

See `cmd/test_jobs/main.go` for a usage example.
