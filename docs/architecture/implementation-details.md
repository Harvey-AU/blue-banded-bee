# Architecture

## System Overview

Blue Banded Bee is built with a worker pool architecture for efficient URL crawling and cache warming. The system consists of several key components:

## Core Components

### Worker Pool System

- **Concurrent Processing**: Multiple workers process tasks simultaneously
- **Job Management**: Jobs are broken down into tasks and distributed across workers
- **Recovery System**: Automatic recovery of stalled or failed tasks
- **Task Monitoring**: Real-time monitoring of task progress and status

### Database Layer (PostgreSQL)

- Stores jobs, tasks, and results
- Uses row-level locking for efficient concurrent access
- Efficient batch operations for better performance
- Maintains job history and statistics

### API Layer

- RESTful endpoints for job management
- Real-time status updates
- Health monitoring endpoints

### Crawler System

- Concurrent URL processing
- Rate limiting and retry logic
- Cache validation
- Response monitoring

## Job Lifecycle

1. **Job Creation**

   - Job created with initial URLs
   - URLs broken down into tasks
   - Job status set to PENDING

2. **Task Processing**

   - Workers pick up pending tasks using FOR UPDATE SKIP LOCKED
   - URLs are crawled with retry logic
   - Results stored in database
   - Task status updated

3. **Job Completion**

   - All tasks completed
   - Final statistics calculated
   - Job marked as COMPLETED

4. **Recovery Handling**
   - Stalled tasks detected
   - Tasks reassigned to workers
   - Failed tasks tracked and reported

## System Monitoring

- **Health Checks**: Regular service health monitoring
- **Task Progress**: Real-time task completion tracking
- **Error Tracking**: Sentry integration for error reporting
- **Performance Metrics**: Response times and cache status monitoring

## Security

- Rate limiting per client IP
- Request validation
- Error handling and sanitization
- Secure configuration management
- JWT-based authentication with Supabase Auth
- Row Level Security for database access control

## Deployment Architecture

- Fly.io hosting
- PostgreSQL database for efficient concurrent access
- Cloudflare caching layer
- Sentry error tracking
