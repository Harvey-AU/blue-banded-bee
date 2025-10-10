#!/bin/bash
# Auto-format Go code before committing

set -e

echo "Running goimports..."
goimports -w .

echo "Running gofmt..."
gofmt -w -s .

echo "Formatting complete!"
