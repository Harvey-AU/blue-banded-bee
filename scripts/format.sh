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

# Format docs/config files
echo ""
echo "📝 Formatting docs and config files..."
if command -v prettier &> /dev/null; then
    prettier --write "**/*.{md,yml,yaml,json}"
    echo "  ✅ prettier complete"
else
    echo "  ⚠️  prettier not installed - skipping docs/config formatting"
    echo "     Install with: npm install -g prettier"
fi

echo ""
echo "✨ Formatting complete!"
