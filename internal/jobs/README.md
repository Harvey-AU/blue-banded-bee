# Job Queue System

This package implements a robust job queue system for the crawler with the following features:

## Components

- **Job Management**: Create, start, pause, and cancel crawling jobs
- **Task Queue**: Persistent queue of URLs to be processed
- **Worker Pool**: Concurrent workers that process URLs from the queue
- **Status Tracking**: Monitor job progress and task status
- **Error Handling**: Retry logic for both HTTP requests and database operations

## Database Schema

The system uses two main tables:

- **jobs**: Stores job metadata, status, and progress
- **tasks**: Stores individual URLs to crawl and their results

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
