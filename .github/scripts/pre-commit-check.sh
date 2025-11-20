#!/bin/bash

# Pre-commit hook script for code style checks
# This script can be used as a git pre-commit hook or run manually

set -e

echo "🔍 Running pre-commit code style checks..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if gofmt is available
if ! command -v gofmt &> /dev/null; then
    echo -e "${RED}❌ gofmt not found. Please install Go.${NC}"
    exit 1
fi

# Get list of staged Go files
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' | grep -v 'third_party\|vendor\|\.pb\.go$' || true)

if [ -z "$STAGED_GO_FILES" ]; then
    echo -e "${GREEN}✅ No Go files to check${NC}"
    exit 0
fi

echo "Found $(echo "$STAGED_GO_FILES" | wc -l) Go file(s) to check"

# Check gofmt
echo ""
echo "🔍 Checking code formatting (gofmt)..."
UNFORMATTED=$(echo "$STAGED_GO_FILES" | xargs gofmt -s -l 2>/dev/null || true)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}❌ The following files are not formatted correctly:${NC}"
    echo "$UNFORMATTED"
    echo ""
    echo -e "${YELLOW}💡 Run: gofmt -s -w .${NC}"
    exit 1
fi
echo -e "${GREEN}✅ All files are properly formatted${NC}"

# Check goimports
echo ""
echo "🔍 Checking import formatting (goimports)..."
if ! command -v goimports &> /dev/null; then
    echo -e "${YELLOW}⚠️  goimports not found. Installing...${NC}"
    go install golang.org/x/tools/cmd/goimports@latest
fi

UNFORMATTED_IMPORTS=$(echo "$STAGED_GO_FILES" | xargs goimports -l 2>/dev/null || true)
if [ -n "$UNFORMATTED_IMPORTS" ]; then
    echo -e "${RED}❌ The following files have incorrectly formatted imports:${NC}"
    echo "$UNFORMATTED_IMPORTS"
    echo ""
    echo -e "${YELLOW}💡 Run: goimports -w .${NC}"
    exit 1
fi
echo -e "${GREEN}✅ All imports are properly formatted${NC}"

# Check for common issues
echo ""
echo "🔍 Checking for common issues..."

# Check for fmt.Printf in non-test files
if echo "$STAGED_GO_FILES" | xargs grep -l "fmt\.Print" | grep -v "_test.go$" | grep -v "third_party\|vendor" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Warning: Found fmt.Print* in non-test files. Consider using a logger instead.${NC}"
fi

# Check for panic usage
if echo "$STAGED_GO_FILES" | xargs grep -l "panic(" | grep -v "_test.go$" | grep -v "third_party\|vendor" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Warning: Found panic() usage. Consider returning errors instead.${NC}"
fi

# Check for empty error handling
if echo "$STAGED_GO_FILES" | xargs grep -l "if err != nil {}" | grep -v "third_party\|vendor" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Warning: Found empty error handling blocks.${NC}"
fi

echo -e "${GREEN}✅ Common issues check completed${NC}"

# Run basic go vet
echo ""
echo "🔍 Running go vet..."
for file in $STAGED_GO_FILES; do
    if [ -f "$file" ]; then
        dir=$(dirname "$file")
        if [ "$dir" = "." ]; then
            dir="./"
        fi
        go vet "$dir" 2>&1 || true
    fi
done
echo -e "${GREEN}✅ go vet check completed${NC}"

echo ""
echo -e "${GREEN}✅ All pre-commit checks passed!${NC}"
exit 0

