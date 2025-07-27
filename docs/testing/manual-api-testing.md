# Manual API Testing

For automated testing, see the main [Testing Documentation](README.md).

## Testing with Browser

1. Start the server: `go run ./cmd/app/main.go`
2. Open `http://localhost:8080/test-login.html`
3. Login with social provider
4. Use the test interface to make API calls

## Testing with curl

### Public Endpoints

```bash
# Health check
curl http://localhost:8080/health

# Database health
curl http://localhost:8080/health/db
```

### Protected Endpoints

Get JWT token from Supabase dashboard or test login page:

```bash
# Create job
curl -X POST http://localhost:8080/v1/jobs \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com", "use_sitemap": true}'

# Get job status
curl http://localhost:8080/v1/jobs/JOB_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Testing with Postman

1. Import the API endpoints
2. Set `Authorization` header with Bearer token
3. Use environment variables for base URL

## Authentication Flow

1. User logs in via Supabase Auth
2. Receives JWT token
3. Include token in `Authorization: Bearer TOKEN` header
4. Server validates token and processes request