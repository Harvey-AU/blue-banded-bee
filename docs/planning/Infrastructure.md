## Architecture Summary

- Frontend: Webflow
- Backend: Go on Fly.io
- Database: Turso
- Queue: Fly Redis
- Auth: Clerk
- Payments: Stripe
- Analytics: Google Analytics
### Rationale

- Simple infrastructure
- Low maintenance
- Good performance
- Room to scale
- Cost-effective start

### We chose these over alternatives (Firebase, Supabase, etc.) because:

- More focused tools
- Only pay for what you need
- Simpler architecture
- Better cost control
- Easier to maintain

## Core Infrastructure

### Language: Go

- Better performance for crawling
- Lower hosting costs
- Great concurrency
- Simple deployment
- Interested in learning

### Hosting: Fly.io

- Simple deployment
- Built-in Redis for queues
- Good monitoring basics
- Reasonable pricing
- Global deployment

### Database: Turso

- Simple to manage
- Edge deployment
- Zero maintenance
- Good for our scale
- Better than managing SQLite

# Key Services:

### Auth: Clerk

- Modern, clean UX
- Simple implementation
- Great social logins
- Good user management
- Better than Webflow auth apps

## Payments: Paddle

- Usage-based billing
- Subscription management
- Better than Webflow ecommerce
- Handles usage limits

## Analytics: Start with GA

- Free to start
- Simple implementation
- Add PostHog later if needed
- Good enough for launch