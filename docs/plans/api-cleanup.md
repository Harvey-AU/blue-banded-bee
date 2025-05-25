# API Endpoint Cleanup Guide

## Overview

This document outlines the API endpoints that need cleanup or consolidation to improve maintainability and reduce duplication. The goal is to standardize error handling, simplify endpoint implementations, and ensure consistent behavior across the API.

## API Endpoint Assessment

### Current Issues

1. **Debug/Test Endpoints**: Several endpoints exist purely for debugging or testing purposes that should be removed or properly documented
2. **Inconsistent Error Handling**: Different endpoints handle errors in different ways
3. **Duplicate Route Handlers**: Some functionality is duplicated across multiple endpoints
4. **Undocumented API Endpoints**: Many endpoints lack proper documentation

## Endpoints to Cleanup

### 1. Legacy/Debug Endpoints

| Endpoint | Issue | Action |
|----------|-------|--------|
| `/debug/queue` | Debug endpoint that exposes internal queue state | Move to admin-only access or remove |
| `/debug/vars` | Exposes runtime variables | Restrict access or remove |
| `/test/crawler` | Test endpoint for crawler | Remove or move to test package |
| `/v1/ping` | Duplicate of health check | Consolidate with `/health` |

### 2. Endpoints with Inconsistent Error Handling

| Endpoint | Issue | Action |
|----------|-------|--------|
| `/v1/jobs` | Inconsistent error formats | Standardize error responses |
| `/v1/site` | Missing proper status codes | Implement proper HTTP status codes |
| `/v1/domains` | Mixed error formats | Use consistent error structure |

### 3. Endpoints to Consolidate

| Original Endpoints | Consolidated Endpoint | Notes |
|-------------------|----------------------|-------|
| `/v1/job/create` & `/v1/site` | `/v1/jobs` | Use proper HTTP methods (POST) |
| `/v1/job/status/:id` & `/v1/job/:id` | `/v1/jobs/:id` | Consolidate to RESTful pattern |
| `/health` & `/v1/ping` | `/health` | Keep one health check endpoint |

## Implementation Plan

1. **Create Standard Error Handler**
   - Implement a standard error response structure
   - Add proper HTTP status code mapping
   - Ensure all errors include request ID for tracing

```go
// Standard error response
type ErrorResponse struct {
    Status  int    `json:"status"`
    Message string `json:"message"`
    Code    string `json:"code,omitempty"`
    RequestID string `json:"request_id"`
}

// Standard error handler
func StandardErrorHandler(w http.ResponseWriter, err error, status int, code string) {
    reqID := middleware.GetRequestID(r.Context())
    errResp := ErrorResponse{
        Status: status,
        Message: err.Error(),
        Code: code,
        RequestID: reqID,
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(errResp)
}
```

2. **Implement RESTful Endpoints**
   - Replace current endpoints with proper RESTful design
   - Use appropriate HTTP methods (GET, POST, PUT, DELETE)
   - Document API consistently

## New API Structure

```
/health - Health check
/metrics - Prometheus metrics endpoint (protected)

/v1/jobs
  GET / - List all jobs
  POST / - Create new job
  GET /:id - Get job details
  PUT /:id - Update job 
  DELETE /:id - Cancel/delete job
  
/v1/domains
  GET / - List domains
  GET /:id - Get domain details
  GET /:id/stats - Get domain statistics
  
/v1/admin (protected)
  GET /queue-stats - Queue statistics
  POST /cleanup - Run cleanup job
  POST /reset - Reset queue (admin only)
```

## Documentation Improvements

1. Add Swagger/OpenAPI documentation for all endpoints
2. Include request/response examples
3. Document error codes and meanings
4. Add rate limit information

## Metrics Endpoints

Implement proper metrics exposure through a dedicated endpoint:

```
/metrics - Prometheus format metrics
```

Include:
- Queue depths
- Worker counts
- Processing rates
- Database connection stats
- Response times
- Error rates

## Implementation Priority

1. Standardize error handling
2. Consolidate duplicate endpoints
3. Remove debug/test endpoints
4. Implement RESTful API structure
5. Add comprehensive documentation