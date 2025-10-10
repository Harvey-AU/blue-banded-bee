#!/bin/bash
# Setup Git hooks for Blue Banded Bee
# This configures Git to use hooks from the .githooks/ directory

set -e

echo "🔧 Setting up Git hooks..."

# Configure Git to use .githooks directory
git config core.hooksPath .githooks

echo "✅ Git hooks configured successfully!"
echo ""
echo "Active hooks:"
echo "  📝 pre-commit: Auto-formats Go, Markdown, YAML, and JSON files"
echo ""
echo "To commit without running hooks (not recommended):"
echo "  git commit --no-verify"
