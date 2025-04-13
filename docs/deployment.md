# Deployment Guide

## Prerequisites

- Fly.io account
- GitHub account
- Turso database
- Sentry account (optional)
- Environment variables configured

## Environment Setup

### Production Environment

Required environment variables:

```bash
APP_ENV=production
PORT=8080
LOG_LEVEL=info
DATABASE_URL=your_turso_url
DATABASE_AUTH_TOKEN=your_turso_token
SENTRY_DSN=your_sentry_dsn
```

## Configuration

### Environment Variables

```env
# App
APP_ENV=development  # development or production
PORT=8080           # API server port
DEBUG=true          # Enable debug logging
LOG_LEVEL=debug     # debug, info, warn, or error

# Database
DATABASE_URL=libsql://your-db-name.turso.io
DATABASE_AUTH_TOKEN=your_auth_token

# Error Tracking
SENTRY_DSN=your_sentry_dsn
```

### System Defaults

- Worker Pool Size: 5 workers
- Recovery Interval: 1 minute
- Rate Limiting: 5 requests/second with burst

## Fly.io Deployment

1. Install Flyctl:

   ```bash
   curl -L https://fly.io/install.sh | sh
   fly auth login
   ```

2. Initial setup:

   ```bash
   fly launch
   ```

3. Configure secrets:
   ```bash
   fly secrets set DATABASE_URL=your_turso_url
   fly secrets set DATABASE_AUTH_TOKEN=your_turso_token
   fly secrets set SENTRY_DSN=your_sentry_dsn
   ```

## GitHub Actions Deployment

1. Repository secrets:

   - `FLY_API_TOKEN`
   - `TURSO_DATABASE_URL`
   - `TURSO_AUTH_TOKEN`
   - `SENTRY_DSN`

2. Workflow triggers:
   - Push to main branch
   - Manual dispatch

## Monitoring

1. Logs:

   ```bash
   fly logs
   ```

2. Status:
   ```bash
   fly status
   fly metrics
   ```

## Security Considerations

1. Production Security

   - Enable all security headers
   - Use HTTPS only
   - Implement rate limiting
   - Monitor for unusual activity

2. Database Security

   - Regular backups
   - Connection encryption
   - Access control
   - Query optimization

3. Monitoring
   - Error tracking
   - Performance monitoring
   - Security alerts
   - Uptime monitoring

### Troubleshooting

1. Check application health:

   ```bash
   curl https://your-app.fly.dev/health
   ```

2. View deployment history:

   ```bash
   fly history
   ```

3. SSH into instance:
   ```bash
   fly ssh console
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
