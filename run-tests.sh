#!/bin/bash

# Tests now automatically load from .env.test via testutil.LoadTestEnv()

echo "Running database connection test..."
go test -v ./internal/jobs -run TestDatabaseConnection

echo ""
echo "Running all tests..."
go test -v ./...