# API Reference

## Simple list of endpoints

### Create a crawl job by scanning sitemap

curl "http://localhost:8080/site?domain=teamharvey.co"
curl "https://blue-banded-bee.fly.dev/site?domain=teamharvey.co"
curl "http://localhost:8080/site?domain=teamharvey.co&max=100"
curl "https://blue-banded-bee.fly.dev/site?domain=teamharvey.co&max=100"

curl "http://localhost:8080/site?domain=teamharvey.co&find_links=true"
curl "https://blue-banded-bee.fly.dev/site?domain=teamharvey.co&find_links=true"

### Check crawl job status

curl "http://localhost:8080/job-status?job_id=job_123abc"
curl "https://blue-banded-bee.fly.dev/job-status?job_id=job_123abc"

### Reset DB schema

curl "http://localhost:8080/reset-db"
curl "https://blue-banded-bee.fly.dev/reset-db"

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
    "max_pages": 2,
    "concurrency": 5
  }
}
```

### Get Job Status

```http
GET /api/v1/job-status/{jobId}
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
