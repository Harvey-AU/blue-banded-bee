# Testing

Blue Banded Bee prioritises fast, isolated unit tests by default, with opt-in integration tests against a real Supabase test database. CI is automated via GitHub Actions.

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

- **Unit tests (default)**: Use mocks and table-driven tests. No build tags required. Run with `-short` and `-race`.
- **Integration tests (opt-in)**: Use real test database; must be tagged with `//go:build integration`.
- **Test Database**: Supabase branch configured in `.env.test`.
- **CI Pipeline**: Automated testing on every push; split fast unit job and separate integration job (see Test Plan).

## Documentation

- [Test Plan](../TEST_PLAN.md) — includes Immediate Actions and Standards quick guide, coverage gaps, and priorities
- [Setup Guide](setup.md) — Configure test environment
- [Writing Tests](writing-tests.md) — Guidelines and patterns
- [CI/CD Pipeline](ci-cd.md) — GitHub Actions configuration
- [Troubleshooting](troubleshooting.md) — Common issues

## Conventions and Tips

- Use `testify` (`assert`/`require`) consistently.
- Prefer table-driven tests and descriptive subtest names.
- Mark helper functions with `t.Helper()`; use `t.Cleanup()` for teardown.
- Use `t.Parallel()` for independent subtests to speed up execution.

## Current Coverage

Overall: **23.5%** (as of 2025-08-08)

Key packages:
- `internal/cache`: 100% ✅
- `internal/util`: 81.1% ✅
- `internal/crawler`: 63.2% ⚠️
- `internal/api`: 34.5% ⚠️
- `internal/db`: 14.3% ❌ (improved from 10.5%)
- `internal/jobs`: 5.0% ❌ (improved from 1.1%)

See [Codecov dashboard](https://codecov.io/github/harvey-au/blue-banded-bee) for detailed coverage reports.
