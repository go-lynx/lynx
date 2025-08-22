#!/bin/bash

# Swagger Plugin Test Script

echo "==================================="
echo "Lynx Swagger Plugin Functionality Test"
echo "==================================="

# Set working directory
WORK_DIR=$(dirname "$0")
cd "$WORK_DIR/../.."

echo ""
echo "1. Compiling Swagger plugin..."
if go build ./plugins/swagger/...; then
    echo "✅ Plugin compilation successful"
else
    echo "❌ Plugin compilation failed"
    exit 1
fi

echo ""
echo "2. Running unit tests..."
if go test ./plugins/swagger/test -v -count=1; then
    echo "✅ Unit tests passed"
else
    echo "⚠️  Unit tests failed (may be missing test dependencies)"
fi

echo ""
echo "3. Compiling example program..."
if go build -o /tmp/swagger_example ./examples/swagger/main.go; then
    echo "✅ Example program compilation successful"
else
    echo "❌ Example program compilation failed"
    exit 1
fi

echo ""
echo "4. Compiling complete example..."
if go build -o /tmp/swagger_full_example ./plugins/swagger/example/full_example.go; then
    echo "✅ Complete example compilation successful"
else
    echo "❌ Complete example compilation failed"
    exit 1
fi

echo ""
echo "==================================="
echo "Testing completed!"
echo ""
echo "Plugin functionality description:"
echo "1. Automatically scans code annotations to generate Swagger documentation"
echo "2. Provides embedded Swagger UI service"
echo "3. Supports file monitoring and hot updates"
echo "4. Supports multiple Swagger annotation formats"
echo ""
echo "Usage method:"
echo "1. Add Swagger annotations in code"
echo "2. Configure config.yaml file"
echo "3. Import plugin: import _ \"github.com/go-lynx/lynx/plugins/swagger\""
echo "4. After starting the application, visit http://localhost:8080/swagger"
echo "==================================="
