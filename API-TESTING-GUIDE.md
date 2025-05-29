# API Testing Guide

## Quick Start - Testing Your APIs

### 1. Start Your API Server

```bash
cd /path/to/blue-banded-bee
go run ./cmd/app/main.go
```

The server will start on `http://localhost:8080`

### 2. Test Basic Endpoints (No Auth Required)

Open your browser or use curl:

```bash
# Health check
curl http://localhost:8080/health

# Database health check  
curl http://localhost:8080/health/db
```

You should see JSON responses like:
```json
{
  "status": "healthy",
  "timestamp": "2023-12-07T10:30:00Z",
  "service": "blue-banded-bee",
  "version": "0.4.0"
}
```

### 3. Test Authentication Flow

#### Option A: Use the Test Page (Easiest)
1. Open `http://localhost:8080/test-login.html` in your browser
2. Click "Continue with Google" (or any social provider)
3. After login, you'll see your user info and can test APIs
4. Click the "Test" buttons to see API responses

#### Option B: Manual Testing
1. Get a JWT token from Supabase (see below)
2. Use tools like Postman or curl with the token

### 4. Understanding JWT Tokens

**What is a JWT token?**
- A "digital passport" that proves who you are
- Contains your user ID, email, etc.
- Expires after some time (usually 1 hour)
- Looks like: `eyJhbGciOiJIUzI1NiIs...` (very long string)

**How to get a token:**
1. Login via test page → Browser developer tools → Console → Type:
   ```javascript
   const { data: { session } } = await supabase.auth.getSession();
   console.log(session.access_token);
   ```
2. Copy the token that appears

**How to use the token:**
- Add to HTTP requests as header: `Authorization: Bearer YOUR_TOKEN_HERE`

### 5. Test Protected Endpoints

Once you have a token:

```bash
# Replace YOUR_TOKEN_HERE with actual token
TOKEN="eyJhbGciOiJIUzI1NiIs..."

# Test user profile
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/v1/auth/profile

# Create a job
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"domain":"example.com","use_sitemap":true,"find_links":true,"max_pages":5}' \
     http://localhost:8080/v1/jobs

# List jobs
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/v1/jobs
```

## Understanding API Responses

### Success Response Format:
```json
{
  "status": "success",
  "data": {
    "id": "job_123",
    "domain": "example.com",
    "status": "pending"
  },
  "message": "Job created successfully",
  "request_id": "req_456"
}
```

### Error Response Format:
```json
{
  "status": 400,
  "message": "Domain is required",
  "code": "BAD_REQUEST",
  "request_id": "req_789"
}
```

## Common Issues & Solutions

### "Unauthorized" Error
- **Problem**: Token is missing or invalid
- **Solution**: Get a fresh token from test page

### "User not found" Error  
- **Problem**: User exists in Supabase but not in your database
- **Solution**: Call `/v1/auth/register` first to create user record

### "Connection refused" Error
- **Problem**: Server not running
- **Solution**: Run `go run ./cmd/app/main.go`

### CORS Errors in Browser
- **Problem**: Browser blocking cross-origin requests
- **Solution**: API includes CORS headers, should work from test page

## Tools for Testing

### 1. Browser (Easiest)
- Use the test page: `http://localhost:8080/test-login.html`
- Use browser dev tools console for custom requests

### 2. Postman (Visual)
- Download: https://www.postman.com/downloads/
- Create requests with nice UI
- Save requests for reuse

### 3. VS Code REST Client
- Install "REST Client" extension
- Use the `api-tests.http` file provided
- Click "Send Request" above each test

### 4. curl (Command Line)
- Built into macOS/Linux
- Good for automation/scripts

## Authentication in Practice

### The Complete Flow:

1. **User logs in** (via Google, Facebook, etc.)
   ```
   Browser → Supabase → Gets JWT token
   ```

2. **Your app gets the token**
   ```javascript
   const { data: { session } } = await supabase.auth.getSession();
   const token = session.access_token;
   ```

3. **Include token in API requests**
   ```javascript
   fetch('/v1/jobs', {
     headers: {
       'Authorization': `Bearer ${token}`,
       'Content-Type': 'application/json'
     },
     method: 'POST',
     body: JSON.stringify({ domain: 'example.com' })
   })
   ```

4. **API validates token**
   ```
   API → Checks token → Allows/denies request
   ```

### Token Lifecycle:
- **Expires**: Tokens expire (usually 1 hour)
- **Refresh**: Supabase automatically refreshes them
- **Storage**: Store in memory (not localStorage for security)

## Next Steps

Once you're comfortable with testing:
1. Build a simple frontend interface
2. Create API key system for integrations
3. Add webhooks for job notifications
4. Build Slack/Webflow integrations

## Troubleshooting Database Issues

If you get database errors:
```bash
# Reset database (development only)
curl -X POST http://localhost:8080/admin/reset-db
```

This clears all data and recreates tables.