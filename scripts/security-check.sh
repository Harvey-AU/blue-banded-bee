#!/bin/bash

echo "=== 🛡️  Running Security Checks ==="
EXIT_CODE=0

echo -e "\n🔍 Running Trivy (Filesystem, Secrets, Config)..."
# Scan for secrets, misconfigs, and vulnerabilities in library code
# Skipping .worktrees to avoid recursion if run from root
# Skipping scripts/auth/config.py - contains publishable anon key (like Stripe pk_*), not a secret
if ! trivy fs --scanners vuln,secret,misconfig \
  --ignore-unfixed \
  --skip-dirs .worktrees \
  --skip-files scripts/auth/config.py \
  .; then
    EXIT_CODE=1
fi

echo -e "\n🔍 Running govulncheck (Go Dependencies)..."
# Checks if we actually call the vulnerable functions
if ! govulncheck ./...; then
    EXIT_CODE=1
fi

echo -e "\n🔍 Running ESLint Security (JS Code)..."
if ! npx eslint "web/**/*.js"; then
    EXIT_CODE=1
fi

echo -e "\n🔍 Running Gosec (via golangci-lint)..."
# Static analysis for Go code security
if ! golangci-lint run ./...; then
    EXIT_CODE=1
fi

if [ $EXIT_CODE -eq 0 ]; then
    echo -e "\n✅ All Security Checks Completed"
else
    echo -e "\n⚠️  Security Checks Failed"
    exit $EXIT_CODE
fi
