# Technical Concepts

## Core Concepts

### Jobs and Tasks

- **Job**: A collection of URLs to be crawled from a single domain
- **Task**: Individual URL processing unit within a job
- **Worker**: Process that executes tasks concurrently

### Task Processing

1. **Task Creation**: URLs are converted to tasks with unique IDs
2. **Task Assignment**: Workers pick up available tasks
3. **Processing**: URL crawling with retry logic
4. **Completion**: Results stored and status updated

### Recovery Mechanisms

- **Task Recovery**: Automatic recovery of stalled tasks
- **Job Recovery**: Restoration of interrupted jobs
- **Error Handling**: Graduated retry with backoff
- **State Management**: Persistent task state tracking

### Worker Pool

- **Concurrent Processing**: Multiple workers handle tasks
- **Resource Management**: Controlled concurrency
- **Load Balancing**: Even task distribution
- **Health Monitoring**: Worker status tracking

### Rate Limiting

- **Client IP Detection**: Accurate client identification
- **Request Throttling**: Controlled request rates
- **Domain Limits**: Per-domain request limits
- **Burst Handling**: Request burst management

## Data Flow

1. **Input Processing**

   - URL validation
   - Domain extraction
   - Task creation

2. **Task Execution**

   - Worker assignment
   - URL crawling
   - Result storage

3. **Result Handling**

   - Cache validation
   - Response analysis
   - Status reporting

4. **Monitoring**
   - Performance tracking
   - Error reporting
   - Status updates

## Cache Warming

### Overview

The cache warmer proactively visits URLs to ensure content is cached and readily available. It tracks response times and cache status to monitor effectiveness.

### Components

1. URL Crawler

   - Uses `colly` library for efficient crawling
   - Respects rate limits and concurrency settings
   - Tracks response metrics

2. Rate Limiting

   - Token bucket algorithm implementation
   - Per-IP address limiting with proper client IP detection
   - Supports X-Forwarded-For and X-Real-IP headers for proxy environments
   - 5 requests per second default limit

3. Metrics Collection
   - Response time tracking
   - Cache status monitoring
   - Error rate tracking
   - Request counts

## Database Design

### Schema

CREATE TABLE jobs (
id TEXT PRIMARY KEY,
domain TEXT NOT NULL,
status TEXT NOT NULL,
progress REAL DEFAULT 0.0,
total_tasks INTEGER DEFAULT 0,
completed_tasks INTEGER DEFAULT 0,
failed_tasks INTEGER DEFAULT 0,
created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
started_at TIMESTAMP WITH TIME ZONE,
completed_at TIMESTAMP WITH TIME ZONE,
concurrency INTEGER DEFAULT 1,
find_links BOOLEAN DEFAULT FALSE,
max_depth INTEGER DEFAULT 0
);

CREATE TABLE tasks (
id TEXT PRIMARY KEY,
job_id TEXT REFERENCES jobs(id),
url TEXT NOT NULL,
status TEXT NOT NULL,
depth INTEGER DEFAULT 0,
source_type TEXT,
source_url TEXT,
created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
started_at TIMESTAMP WITH TIME ZONE,
completed_at TIMESTAMP WITH TIME ZONE,
status_code INTEGER,
response_time INTEGER,
cache_status TEXT,
content_type TEXT,
error TEXT
);

````

### Connection Management

- Connection pooling with a configurable pool size
- Maximum 25 open connections
- Maximum 10 idle connections
- 5-minute connection lifetime
- 2-minute idle connection timeout
- Automatic reconnection

## Error Handling

1. Types of Errors

   - URL validation errors
   - Network errors
   - Database errors
   - Rate limit errors

2. Error Response Format

   ```json
   {
     "error": "Error description",
     "status_code": 400
   }
````

3. Logging and Monitoring
   - Structured logging with zerolog
   - Sentry integration for error tracking
   - Performance metrics collection

## Caching Strategy

1. Cache Status Tracking

   - Monitors Cloudflare cache status
   - Tracks HIT/MISS ratios
   - Records response times

2. Performance Optimisation
   - Concurrent crawling
   - Connection pooling
   - Rate limiting for stability
