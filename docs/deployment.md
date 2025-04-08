# Deployment Guide

## Prerequisites

- Fly.io account
- GitHub account
- Turso database
- Sentry account (optional)

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
