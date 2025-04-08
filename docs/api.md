# API Reference

## Overview

The Cache Warmer API provides endpoints for URL crawling, cache status checking, and retrieving crawl history.

## Base URL

https://blue-banded-bee.fly.dev

## Endpoints

### Health Check

```http
GET /health
```

Returns the service status and deployment time.

**Response**

200 OK
Content-Type: text/plain
OK - Deployed at: 2024-04-07T15:30:00Z

### Test Crawl

```http
GET /test-crawl?url=<url>
```

Crawls a specified URL and returns the result.

**Parameters**

- `url` (optional): URL to crawl. Defaults to "https://www.teamharvey.co"

**Success Response**

```json
{
  "url": "https://example.com",
  "response_time_ms": 523,
  "status_code": 200,
  "error": "",
  "cache_status": "HIT",
  "timestamp": 1744065627
}
```

**Error Response**

```json
{
  "url": "https://invalid-url",
  "response_time_ms": 0,
  "status_code": 0,
  "error": "invalid URL format",
  "cache_status": "",
  "timestamp": 1744065627
}
```

### Recent Crawls

```http
GET /recent-crawls
```

Returns the 10 most recent crawl results.

**Response**

```json
[
  {
    "id": 123,
    "url": "https://example.com",
    "response_time_ms": 523,
    "status_code": 200,
    "error": "",
    "cache_status": "HIT",
    "created_at": "2024-04-07T15:30:00Z"
  }
]
```

## Rate Limiting

- Implementation: Token bucket algorithm
- Rate: 5 requests per second per IP address
- Response: Status code 429 when limit exceeded

## Error Codes

- 200: Success
- 400: Invalid request (e.g., malformed URL)
- 401: Unauthorized (development endpoints)
- 429: Too many requests
- 500: Server error

## Response Types

All endpoints return either:

- `text/plain` for health check
- `application/json` for all other endpoints

## Development Mode Features

When `APP_ENV=development`:

- Additional debugging information in responses
- Reset database endpoint available (requires token)
- More verbose error messages
- Detailed logging enabled

## Development Mode Endpoints

When running in development mode (`APP_ENV=development`), additional endpoints are available:

### Reset Database

```http
POST /reset-db
```

Resets the database schema. Requires authentication token that is generated and logged at server startup.

**Headers**

Authorization: Bearer <token from server startup logs>

**Response**

```json
{
  "status": "Database schema reset successfully"
}
```

**Notes**

- Only available in development mode (`APP_ENV=development`)
- Token is generated when server starts and logged to console
- Token changes each time server restarts
