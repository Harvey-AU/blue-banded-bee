# System Architecture

## Overview

The Blue Banded Bee service is designed with simplicity, performance, and scalability in mind. Our architecture prioritizes maintainability, cost-effectiveness, and efficient resource utilization.

## Technology Stack & Rationale

### Backend: Go on Fly.io

- **Why Go**: Better performance for crawling, excellent concurrency, low resource usage, simple deployment
- **Why Fly.io**: Simple deployment, global distribution, built-in Redis for potential queue needs, reasonable pricing

### Database: Turso

- **Why Turso**: Edge deployment, zero maintenance, excellent performance, libSQL compatibility
- **Benefits**: Simpler than managing SQLite, cost-effective for our scale, reliable

### Authentication: Clerk (Planned)

- **Why Clerk**: Modern UX, simple implementation, great social logins, comprehensive user management
- **Benefits**: Better than Webflow's native auth, easier to integrate

### Payments: Paddle (Planned)

- **Why Paddle**: Usage-based billing, subscription management, handles international taxes
- **Benefits**: Better than Webflow ecommerce for SaaS, handles usage limits well

### Analytics: Google Analytics (Planned)

- **Why GA**: Free to start, simple implementation, adequate for initial launch
- **Future**: Consider PostHog if more detailed analytics are needed

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

### 1. Application Server

- Go HTTP server handling API requests
- Manages concurrent crawling operations
- Implements rate limiting and request throttling
- Provides error tracking via Sentry

### 2. Database

- SQLite-compatible Turso database
- Stores crawl results and historical data
- Manages user data and usage metrics
- Provides analytics capabilities

### 3. Cache Layer

- Cloudflare caches crawled content
- Provides cache status information
- Improves response times and reduces origin load

### 4. CI/CD Pipeline

- GitHub Actions for automated testing
- Deployment automation to Fly.io
- Secret management and version control

## Data Flow

1. **Request Flow**

   ```
   Client → API Server → URL Crawler → Target Website
                      ↓
                   Database
   ```

2. **Deployment Flow**
   ```
   GitHub → GitHub Actions → Fly.io → Production
   ```

## Security Considerations

1. **Authentication**

   - Development endpoints require tokens
   - Database authentication
   - API token management

2. **Rate Limiting**

   - Per-IP limits with proper client IP detection
   - Global rate limiting
   - Concurrent request limits

3. **Error Handling**
   - Sanitized error messages
   - Secure logging practices
   - Monitoring and alerts

## Scalability

1. **Horizontal Scaling**

   - Stateless application design
   - Database connection pooling
   - Cache distribution

2. **Performance Optimization**

   - Concurrent crawling
   - Connection pooling
   - Response caching

3. **Monitoring**
   - Response time tracking
   - Error rate monitoring
   - Cache effectiveness metrics

## Infrastructure Evolution

The current architecture is designed to be simple and maintainable while providing room to scale:

- **Initial phase**: Focus on core crawling functionality with minimal components
- **Growth phase**: Add auth, user management, and billing capabilities
- **Scale phase**: Enhance with more sophisticated queuing, caching, and analytics

This approach allows us to start with a cost-effective infrastructure while having a clear path for growth.
