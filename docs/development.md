# Development Guide

## Setup

### Prerequisites

- Go 1.21+
- [Air](https://github.com/cosmtrek/air) for hot reloading
- Docker (optional, for containerized development)

### Local Environment Setup

1. Fork and clone repository:

```bash
git clone https://github.com/[your-username]/blue-banded-bee.git
cd blue-banded-bee
```

2. Copy environment file:

```bash
cp .env.example .env
```

3. Configure your `.env` file with required credentials:

- `DATABASE_URL`
- `DATABASE_AUTH_TOKEN`
- Other optional settings

## Development Server

### Start with Hot Reloading

```bash
air
```

### Start Without Hot Reloading

```bash
go run src/main.go
```

## Testing

### Run All Tests

```bash
go test ./... -v
```

### Run Integration Tests

```bash
RUN_INTEGRATION_TESTS=true go test ./... -v
```

### Test Coverage

```bash
go test ./... -cover
```

## Docker Development

### Build Container

```bash
docker build -t blue-banded-bee .
```

### Run Container

```bash
docker run -p 8080:8080 --env-file .env blue-banded-bee
```

## Debugging

### Local Debug Configuration

1. Set in `.env`:

```env
DEBUG=true
LOG_LEVEL=debug
```

2. Watch logs:

```bash
air # Logs will show in console
```

### API Testing

The service will be available at:

- Local: http://localhost:8080
- Health check: http://localhost:8080/health

## Worker Pool Development

### Configuration

The worker pool uses these defaults:

- 5 concurrent workers
- 1-minute recovery interval
- 5 requests/second rate limit

### Testing Worker Pool

1. Start the service
2. Create a test job via API
3. Monitor logs for worker activity
4. Check job status via API

## Database

### Local Database

The service uses Turso as the database. Make sure your `.env` contains:

```env
DATABASE_URL=libsql://your-db-name.turso.io
DATABASE_AUTH_TOKEN=your_auth_token
```

## Common Issues

### Windows Users

Use the Windows-specific build commands in `.air.toml`:

```toml
cmd = "go build -o ./tmp/main.exe ./src"
bin = "tmp/main.exe"
```

### Mac/Linux Users

Use the Unix-specific build commands in `.air.toml`:

```toml
cmd = "go build -o ./tmp/main ./src"
bin = "tmp/main"
```

## Contributing

Please see [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed contribution guidelines.

1. Fork the repository
2. Create your feature branch
3. Add/update tests
4. Update documentation
5. Submit pull request
