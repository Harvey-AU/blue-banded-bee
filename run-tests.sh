#!/bin/bash
set -euo pipefail

# Tests now automatically load from .env.test via testutil.LoadTestEnv()

echo "=== Running Unit Tests ==="
echo "Running unit tests with -short flag (should complete in <10 seconds)..."
go test -v -race -short -timeout=30s ./...

echo ""
echo "=== Running Integration Tests ==="
echo "Running integration tests with database connection..."
go test -v -race -tags=integration -timeout=5m ./...

echo ""
echo "=== Coverage Report ==="
echo "Generating coverage report for unit tests..."
go test -race -short -coverprofile=coverage.out -coverpkg=./... ./...
go tool cover -func=coverage.out | tail -1