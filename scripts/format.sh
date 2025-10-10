#!/bin/bash
# Auto-format all code and documentation files

set -e

echo "🔧 Formatting all files..."
echo ""

# Format Go files
echo "📝 Formatting Go files..."
if command -v goimports &> /dev/null; then
    goimports -w .
    echo "  ✅ goimports complete"
else
    gofmt -w -s .
    echo "  ✅ gofmt complete (install goimports for import formatting)"
fi

# Format docs/config/web files
echo ""
echo "📝 Formatting docs, config, and web files..."
if command -v prettier &> /dev/null; then
    prettier --write "**/*.{md,yml,yaml,json,html,css,js}"
    echo "  ✅ prettier complete"
else
    echo "  ⚠️  prettier not installed - skipping docs/config/web formatting"
    echo "     Install with: npm install -g prettier"
fi

echo ""
echo "✨ Formatting complete!"
