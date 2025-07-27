# Testing

Blue Banded Bee uses integration tests with a real Supabase test database and automated CI through GitHub Actions.

## Quick Start

```bash
# Run all tests locally
./run-tests.sh

# Run specific package tests
go test -v ./internal/jobs

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Structure

- **Integration Tests**: Connect to real test database (most common)
- **Unit Tests**: Use mocks, tagged with `//go:build unit`
- **Test Database**: Supabase branch configured in `.env.test`
- **CI Pipeline**: Automated testing on every push

## Documentation

- [Setup Guide](setup.md) - Configure test environment
- [Writing Tests](writing-tests.md) - Guidelines and patterns
- [CI/CD Pipeline](ci-cd.md) - GitHub Actions configuration
- [Troubleshooting](troubleshooting.md) - Common issues

## Current Coverage

See [Codecov dashboard](https://codecov.io/github/harvey-au/blue-banded-bee) for detailed coverage reports.