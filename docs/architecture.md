# System Architecture

## System Components

┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ GitHub │ │ Fly.io │ │ Cloudflare │
│ Actions │────▶│ Server │────▶│ Cache │
└─────────────┘ └─────────────┘ └─────────────┘
│
│
┌─────▼─────┐
│ Turso │
│ Database │
└───────────┘

## Component Details

### 1. Application Server (Fly.io)

- Go HTTP server
- Handles API requests
- Manages crawling operations
- Implements rate limiting

### 2. Database (Turso)

- SQLite-compatible
- Stores crawl results
- Manages historical data
- Provides analytics capabilities

### 3. Cache Layer (Cloudflare)

- Caches crawled content
- Provides cache status
- Improves response times
- Reduces origin load

### 4. CI/CD (GitHub Actions)

- Automated testing
- Deployment automation
- Secret management
- Version control

## Data Flow

1. Request Flow

   ```
   Client → API Server → URL Crawler → Target Website
                     ↓
                  Database
   ```

2. Deployment Flow
   ```
   GitHub → GitHub Actions → Fly.io → Production
   ```

## Security Considerations

1. Authentication

   - Development endpoints require tokens
   - Database authentication
   - API token management

2. Rate Limiting

   - Per-IP limits
   - Global rate limiting
   - Concurrent request limits

3. Error Handling
   - Sanitized error messages
   - Secure logging practices
   - Monitoring and alerts

## Scalability

1. Horizontal Scaling

   - Stateless application design
   - Database connection pooling
   - Cache distribution

2. Performance Optimization

   - Concurrent crawling
   - Connection pooling
   - Response caching

3. Monitoring
   - Response time tracking
   - Error rate monitoring
   - Cache effectiveness metrics
