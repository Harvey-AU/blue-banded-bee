# API Reference

## Simple list of endpoints

### Create a crawl job by scanning sitemap

http://localhost:8080/site?domain=teamharvey.co&workers=5
https://blue-banded-bee.fly.dev/site?domain=teamharvey.co&workers=5

### Test crawl a single URL

http://localhost:8080/test-crawl?url=https://teamharvey.co
https://blue-banded-bee.fly.dev/test-crawl?url=https://teamharvey.co

### Get job status

http://localhost:8080/job_status?id=job_123abc
https://blue-banded-bee.fly.dev/job_status?id=job_123abc

### Cancel a job

http://localhost:8080/cancel_job?id=job_123abc
https://blue-banded-bee.fly.dev/cancel_job?id=job_123abc

### List all jobs

http://localhost:8080/jobs?limit=20&offset=0
https://blue-banded-bee.fly.dev/jobs?limit=20&offset=0

### Get task details

http://localhost:8080/task?id=task_123
https://blue-banded-bee.fly.dev/task?id=task_123

### List tasks for a job

http://localhost:8080/job_tasks?job_id=job_123abc&limit=20&offset=0
https://blue-banded-bee.fly.dev/job_tasks?job_id=job_123abc&limit=20&offset=0

### Retry a failed task

http://localhost:8080/retry_task?id=task_123
https://blue-banded-bee.fly.dev/retry_task?id=task_123

## Job Management

### Create Job

```http
POST /api/v1/jobs
```

Creates a new crawling job.

**Request Body:**

```json
{
  "domain": "teamharvey.co",
  "urls": ["https://teamharvey.co/page1", "..."],
  "options": {
    "maxDepth": 2,
    "concurrency": 5
  }
}
```

### Get Job Status

```http
GET /api/v1/jobs/{jobId}
```

Returns job status and progress.

**Response:**

```json
{
  "id": "job_123",
  "status": "running",
  "progress": {
    "total": 100,
    "completed": 45,
    "failed": 2
  },
  "stats": {
    "avgResponseTime": 250,
    "cacheHitRate": 0.75
  }
}
```

### Cancel Job

```http
POST /api/v1/jobs/{jobId}/cancel
```

Cancels an active job.

### List Jobs

```http
GET /api/v1/jobs
```

Lists all jobs with pagination.

## Task Management

### Get Task Details

```http
GET /api/v1/tasks/{taskId}
```

Returns detailed task information.

### Retry Task

```http
POST /api/v1/tasks/{taskId}/retry
```

Retries a failed task.

## Monitoring

### Health Check

```http
GET /health
```

Returns service health status.

### Metrics

```http
GET /api/v1/metrics
```

Returns system metrics and statistics.

## Error Responses

All endpoints return standard error responses:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {}
  }
}
```

````

```markdown:docs/deployment.md
# Deployment Guide

## Prerequisites
- Fly.io account
- Turso database
- Sentry.io account
- Environment variables configured

## Configuration

### Worker Pool Settings
```env
WORKER_POOL_SIZE=5
WORKER_TIMEOUT=300
RECOVERY_INTERVAL=60
MAX_RETRIES=3
````

### Rate Limiting

```env
RATE_LIMIT_PER_SECOND=10
RATE_LIMIT_BURST=20
```

### Database Configuration

```env
PGHOST=localhost
PGPORT=5432
PGDATABASE=postgres
PGUSER=postgres
PGPASSWORD=your_password
PGSSLMODE=disable
```

## Deployment Steps

1. **Initial Setup**

   ```bash
   flyctl launch
   flyctl secrets set
   ```

2. **Database Migration**

   ```bash
   flyctl ssh console
   ./migrate up
   ```

3. **Deploy Application**
   ```bash
   flyctl deploy
   ```

## Scaling

### Worker Pool Scaling

- Minimum: 3 workers
- Recommended: 5 workers
- Scale based on queue size

### Memory Requirements

- Base: 512MB
- Per Worker: ~100MB
- Recommended: 1GB minimum

## Monitoring

### Health Checks

- Endpoint: `/health`
- Interval: 30s
- Timeout: 5s

### Metrics

- Response times
- Cache hit rates
- Error rates
- Queue depth

### Alerts

- Worker pool health
- Database connectivity
- High error rates
- Queue backlog

## Maintenance

### Database

- Regular VACUUM
- Index optimization
- Connection pool management

### Logs

- Retention: 7 days
- Error tracking in Sentry
- Performance monitoring

````

```markdown:docs/development.md
# Development Guide

## Setup

### Prerequisites
- Go 1.21+
- Docker
- Make

### Local Environment
1. Clone repository
2. Copy `.env.example` to `.env`
3. Configure local environment
4. Run development server

## Development Server

### Start Local Server
```bash
make dev
````

### Run Tests

```bash
make test
```

## Worker Pool Development

### Local Testing

```bash
# Start worker pool
make worker

# Monitor tasks
make monitor
```

### Debug Configuration

```go
// worker/config.go
debug: true
logLevel: "debug"
recoveryInterval: "10s"
```

### Testing Scenarios

#### Recovery Testing

1. Start worker pool
2. Create test job
3. Simulate failures
4. Verify recovery

#### Performance Testing

1. Configure test job
2. Monitor metrics
3. Analyze results

## Database

### Local Database

```bash
make db-setup
make db-migrate
```

### Test Data

```bash
make db-seed
```

## Testing

### Unit Tests

```bash
make test-unit
```

### Integration Tests

```bash
make test-integration
```

### Load Tests

```bash
make test-load
```

## Debugging

### Logs

- Development: stdout
- Structured logging
- Debug level available

### Metrics

- Prometheus format
- Grafana dashboards
- Custom metrics

## Code Style

### Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

### Pre-commit Hooks

```bash
make install-hooks
```
