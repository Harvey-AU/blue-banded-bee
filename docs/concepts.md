# Technical Concepts

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

```sql
CREATE TABLE crawl_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    response_time INTEGER NOT NULL,
    status_code INTEGER,
    error TEXT,
    cache_status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

### Connection Management

- Connection pooling
- Maximum 10 open connections
- 5-minute connection lifetime
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
   ```

3. Logging and Monitoring
   - Structured logging with zerolog
   - Sentry integration for error tracking
   - Performance metrics collection

## Caching Strategy

1. Cache Status Tracking

   - Monitors Cloudflare cache status
   - Tracks HIT/MISS ratios
   - Records response times

2. Performance Optimization
   - Concurrent crawling
   - Connection pooling
   - Rate limiting for stability
