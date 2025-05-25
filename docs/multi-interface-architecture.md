# Multi-Interface Architecture

This document outlines the architecture for supporting multiple interfaces to Blue Banded Bee, including Webflow website, Webflow Designer extension, Slack integration, and other potential interfaces.

## Core Architecture Principles

```
                                                 ┌───────────────────┐
                                                 │                   │
                                                 │  Supabase Auth    │
                                                 │  & Database       │
                                                 │                   │
                                                 └───────────────────┘
                                                          ▲
                                                          │
                          ┌──────────────────────────────┼──────────────────────────────┐
                          │                              │                              │
                          ▼                              ▼                              ▼
            ┌───────────────────────────┐  ┌───────────────────────────┐  ┌───────────────────────────┐
            │                           │  │                           │  │                           │
            │  Go Backend Core API      │  │  Job Processing Engine    │  │  Webhook Dispatcher       │
            │                           │  │                           │  │                           │
            └───────────────────────────┘  └───────────────────────────┘  └───────────────────────────┘
                          ▲                              │                              │
                          │                              │                              │
                          └──────────────────────────────┼──────────────────────────────┘
                                                         │
                                                         ▼
         ┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────┐
         │             │             │             │             │             │
         ▼             ▼             ▼             ▼             ▼             ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│             │ │             │ │             │ │             │ │             │ │             │
│  Webflow    │ │  Webflow    │ │   Slack     │ │   CLI       │ │   Mobile    │ │   Future    │
│  Website    │ │  Designer   │ │ Integration │ │   Tool      │ │    App      │ │ Integrations│
│             │ │  Extension  │ │             │ │             │ │             │ │             │
└─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘
```

## API-First Approach

The foundation of our multi-interface architecture is an API-first design approach:

1. **Complete API Coverage**: Every operation possible through any interface must be supported via the API
2. **Consistent Response Format**: Standard JSON structure across all endpoints
3. **Interface-Agnostic Design**: API doesn't assume any specific frontend technology
4. **Documentation-Driven**: OpenAPI spec defines the contract between backend and all interfaces

## Core Components

### 1. Go Backend Core API

The Go backend API serves as the foundation of all interfaces and provides:

- RESTful endpoints for all operations
- Authentication and authorization
- Resource management (jobs, domains, users)
- Standardized error handling
- Rate limiting and security controls

Example API organisation:

```
/api/v1/
  /auth
    /login                # Authentication endpoints
    /refresh
  /jobs
    /                     # Job CRUD operations
    /{id}                 # Single job operations
    /{id}/tasks           # Tasks within a job
    /{id}/stats           # Job statistics
  /domains
    /                     # Domain management
  /users
    /                     # User management
    /{id}/usage           # Usage statistics
  /webhooks
    /register             # Register webhook endpoints
```

### 2. Authentication System

The authentication system supports multiple authentication methods:

1. **JWT Authentication** (Primary)

   - Used by Webflow website and other browser-based interfaces
   - Tokens issued by Supabase Auth
   - Short expiry with refresh token pattern

2. **API Key Authentication**

   - Used by integrations like Slack and CLI tools
   - Long-lived keys with fine-grained permissions
   - Key management through user dashboard

3. **OAuth Integration**
   - Used for Webflow Designer extension
   - Proper scopes for different operations
   - Required for Webflow Marketplace submission

### 3. Webhook Dispatcher

The webhook system allows real-time notifications to external systems:

- Job status changes trigger webhooks
- Configurable event types
- Delivery confirmation and retry logic
- Signing of payloads for verification

Example webhook payload:

```json
{
  "event": "job.completed",
  "timestamp": "2023-05-18T12:34:56Z",
  "data": {
    "job_id": "abc123",
    "domain": "example.com",
    "status": "completed",
    "total_tasks": 150,
    "completed_tasks": 150,
    "stats": {
      "avg_response_time": 234,
      "cache_hit_ratio": 0.85
    }
  },
  "signature": "sha256=..."
}
```

### 4. Shared Response Format

All API responses follow a consistent format:

```json
{
  "status": "success",
  "data": {
    // Response data varies by endpoint
  },
  "meta": {
    "timestamp": "2023-05-18T12:34:56Z",
    "version": "1.0.0"
  }
}
```

Error responses:

```json
{
  "status": "error",
  "error": {
    "code": "invalid_request",
    "message": "Invalid job configuration",
    "details": {
      "field": "max_pages",
      "issue": "Must be a positive integer"
    }
  },
  "meta": {
    "timestamp": "2023-05-18T12:34:56Z",
    "version": "1.0.0"
  }
}
```

## Interface-Specific Implementations

### 1. Webflow Website

The Webflow website interface is implemented as described in the previous architecture documents:

- JavaScript application embedded in Webflow pages
- Web Components for UI elements
- Realtime updates via Supabase Realtime
- JWT authentication stored in localStorage

Key additions:

- API client library for interaction with Go backend
- Integration with webhook configuration UI

### 2. Webflow Designer Extension

The Webflow Designer Extension requires:

- Standalone extension built with Webflow extension SDK
- OAuth authentication flow for user authorization
- Limited UI focused on job creation and monitoring
- Background authentication refresh
- Extension-specific storage for settings

Key components:

- Progress indicators in the Designer
- Simplified job creation workflow
- Site-specific default settings

### 3. Slack Integration

The Slack integration allows:

- Creating jobs via Slack commands
- Receiving notifications on job completion
- Interactive buttons for common actions
- Job status queries

Implementation details:

- Slack app using API key authentication
- Webhook registration for receiving notifications
- Slack Block Kit for interactive messages

Example Slack command:

```
/warmcache example.com
```

Example Slack notification:

```
🔄 Job Started: example.com
Status: In progress (0/150 tasks completed)
[View Details] [Cancel Job]
```

### 4. CLI Tool (Future)

A command-line interface for power users:

- Go-based CLI tool
- API key authentication
- Local configuration file
- Job management and monitoring
- Batch operations support

Example usage:

```bash
# Create a new job
bbb job create --domain example.com --find-links --sitemap

# Check job status
bbb job status <job-id>

# Monitor job progress
bbb job watch <job-id>
```

### 5. Mobile App (Future)

Potential mobile app interface:

- React Native or Flutter for cross-platform
- JWT authentication with secure storage
- Push notifications for job updates
- Simplified monitoring interface

## Data Synchronization

When users interact with multiple interfaces, data synchronization becomes important:

1. **Single Source of Truth**: All interfaces read from and write to the same API endpoints
2. **Realtime Updates**: Critical for consistent state across interfaces
3. **Offline Capabilities**: Where appropriate (CLI, mobile)

Synchronization methods:

1. **Polling**: Simple, works everywhere, less efficient
2. **Realtime Subscriptions**: Efficient, requires WebSocket support
3. **Webhooks**: Push-based, good for integrations, requires public endpoint

## Authentication Flow Between Interfaces

When users use multiple interfaces, authentication needs special consideration:

1. **Webflow Website → Webflow Designer Extension**:

   - User logs in on website
   - When launching Designer Extension, use OAuth to authenticate
   - Extension receives its own tokens

2. **Webflow Website → Slack**:

   - User logs in on website
   - User generates API key with Slack-specific permissions
   - User adds API key to Slack app configuration

3. **Webflow Website → CLI Tool**:
   - User logs in on website
   - User generates API key with CLI-specific permissions
   - CLI tool stores key in configuration file

## Security Considerations

Multiple interfaces present additional security challenges:

1. **Interface-Specific Permissions**:

   - API keys with limited scope based on interface
   - Different permission sets for different interfaces

2. **Token Storage**:

   - Browser: localStorage with proper expiry
   - Desktop: Encrypted local storage
   - Mobile: Secure enclave where available

3. **Cross-Origin Concerns**:

   - Proper CORS configuration
   - Content Security Policy implementation
   - Anti-CSRF measures

4. **Rate Limiting and Abuse Prevention**:
   - Per-interface rate limits
   - Anomaly detection for unusual patterns
   - IP-based and token-based limiting

## Implementation Plan

To implement this multi-interface architecture:

1. **Phase 1: Core API Development**

   - Design and document complete API
   - Implement with interface-agnostic approach
   - Set up authentication methods
   - Create webhook system

2. **Phase 2: Primary Interface (Webflow Website)**

   - Implement as previously documented
   - Add API client library
   - Ensure compatibility with future interfaces

3. **Phase 3: Webflow Designer Extension**

   - Develop extension using Webflow SDK
   - Implement OAuth flow
   - Create streamlined UI

4. **Phase 4: Slack Integration**

   - Develop Slack app
   - Implement commands and notifications
   - Add interactive elements

5. **Future Phases: Additional Interfaces**
   - CLI tool
   - Mobile app
   - Partner integrations

## Testing Strategy

Testing multi-interface systems requires:

1. **Shared Test Suite**: Core API functionality tested once, applied to all interfaces
2. **Interface-Specific Tests**: UI, UX, and platform-specific behavior
3. **Integration Tests**: Cross-interface workflows

## Monitoring and Analytics

To understand how users interact across interfaces:

1. **Cross-Interface Analytics**:

   - Track which interfaces users prefer
   - Measure feature usage across interfaces
   - Identify cross-interface workflows

2. **Error Tracking**:
   - Interface-specific error reporting
   - Consistent error codes across interfaces
   - Unified logging system

## Conclusion

This multi-interface architecture allows Blue Banded Bee to be accessed from various contexts while maintaining consistency, security, and feature parity. By focusing on an API-first approach with a webhook system and consistent response format, we create a flexible foundation that can adapt to new interface requirements in the future.
