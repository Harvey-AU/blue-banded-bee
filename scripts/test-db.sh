#!/bin/bash
# Load test environment and run tests

# Load both env files
set -a
source .env
source .env.test
set +a

# Export TEST_DATABASE_URL for tests to use
export DATABASE_URL="$TEST_DATABASE_URL"

# Run tests
echo "Running tests against test database..."
go test ./internal/jobs/... -v "$@"