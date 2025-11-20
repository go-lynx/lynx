#!/bin/bash

# Script to set up git hooks for code style checking
# Run this script once to install the pre-commit hook

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
GIT_HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "🔧 Setting up git hooks..."

# Create .git/hooks directory if it doesn't exist
mkdir -p "$GIT_HOOKS_DIR"

# Copy pre-commit hook
if [ -f "$SCRIPT_DIR/pre-commit-check.sh" ]; then
    cp "$SCRIPT_DIR/pre-commit-check.sh" "$GIT_HOOKS_DIR/pre-commit"
    chmod +x "$GIT_HOOKS_DIR/pre-commit"
    echo "✅ Pre-commit hook installed successfully"
    echo ""
    echo "The pre-commit hook will now run automatically before each commit."
    echo "To skip the hook, use: git commit --no-verify"
else
    echo "❌ Error: pre-commit-check.sh not found"
    exit 1
fi

echo ""
echo "✅ Git hooks setup completed!"

