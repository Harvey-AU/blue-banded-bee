#!/bin/bash
set -e

echo "=== 🛡️  Running Security Checks ==="

echo -e "\n🔍 Running Trivy (Filesystem, Secrets, Config)..."
# Scan for secrets, misconfigs, and vulnerabilities in library code
# Skipping .worktrees to avoid recursion if run from root
trivy fs --scanners vuln,secret,misconfig \
  --ignore-unfixed \
  --skip-dirs .worktrees \
  .

echo -e "\n🔍 Running govulncheck (Go Dependencies)..."
# Checks if we actually call the vulnerable functions
govulncheck ./...

echo -e "\n🔍 Running Gosec (via golangci-lint)..."
# Static analysis for Go code security
golangci-lint run --disable-all -E gosec ./...

echo -e "\n✅ All Security Checks Completed"
