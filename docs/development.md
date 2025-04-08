# Development Guide

## Prerequisites

- Go 1.21 or later
- Git
- Turso CLI (for local database)
- Air (for live reload)

## Project Structure

src/
├── main.go # Entry point and HTTP handlers
├── crawler/ # URL crawling logic
└── db/ # Database operations

## Local Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/Harvey-AU/blue-banded-bee.git
   cd blue-banded-bee
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Set up environment:

   ```bash
   cp .env.example .env
   ```

   Required environment variables:

   ```
   APP_ENV=development
   PORT=8080
   LOG_LEVEL=debug
   DATABASE_URL=your_turso_url
   DATABASE_AUTH_TOKEN=your_turso_token
   SENTRY_DSN=your_sentry_dsn
   ```

4. Install Air for live reload:
   ```bash
   go install github.com/air-verse/air@latest
   export PATH=$PATH:$(go env GOPATH)/bin
   ```

## Development Workflow

1. Run with live reload:

   ```bash
   air
   ```

2. Run tests:

   ```bash
   go test ./... -v
   ```

3. Local endpoints:
   - Health check: `http://localhost:8080/health`
   - Test crawl: `http://localhost:8080/test-crawl?url=https://example.com`
   - Recent crawls: `http://localhost:8080/recent-crawls`

## Development Mode Features

- Verbose logging
- Additional debugging endpoints
- Database reset capability
- Detailed error messages

## Security Best Practices

1. Environment Variables

   - Never commit `.env` files
   - Use test credentials for development
   - Rotate tokens regularly

2. Code Security

   - Keep dependencies updated
   - Follow Go security best practices
   - Use prepared statements for DB queries

3. Testing
   - Write security-focused tests
   - Test error conditions
   - Validate input handling

## Testing

- Unit tests: Located alongside source files
- Integration tests: Require database connection
- Run specific tests: `go test ./src/crawler -v`
