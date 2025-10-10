#!/bin/bash
# Install Git hooks for auto-formatting

set -e

HOOK_DIR=".git/hooks"
HOOK_FILE="$HOOK_DIR/pre-commit"

echo "Installing pre-commit hook..."

cat > "$HOOK_FILE" << 'EOF'
#!/bin/bash
# Auto-format Go code before committing

# Only format Go files that are staged for commit
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

if [ -z "$STAGED_GO_FILES" ]; then
    exit 0  # No Go files staged, nothing to do
fi

echo "🔧 Auto-formatting Go files..."

# Format each staged file
for FILE in $STAGED_GO_FILES; do
    # Run goimports (includes gofmt)
    goimports -w "$FILE"

    # Re-add the file to staging (picks up formatting changes)
    git add "$FILE"
done

echo "✅ Formatting complete!"
exit 0
EOF

chmod +x "$HOOK_FILE"

echo "✅ Pre-commit hook installed successfully!"
echo ""
echo "Now when you commit, Go files will be auto-formatted."
echo "To disable: rm .git/hooks/pre-commit"
