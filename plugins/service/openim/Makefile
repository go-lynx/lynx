# OpenIM Service Plugin Makefile
# Provides commands for building, testing, and managing the OpenIM plugin

.PHONY: help build clean test lint format proto

# Default target
help:
	@echo "OpenIM Service Plugin - Available commands:"
	@echo "  build   - Build the plugin"
	@echo "  clean   - Clean build artifacts"
	@echo "  test    - Run tests"
	@echo "  lint    - Run linter"
	@echo "  format  - Format code"
	@echo "  proto   - Generate protobuf code"
	@echo "  example - Build and run integration example"

# Build the plugin
build:
	@echo "Building OpenIM plugin..."
	go build -o bin/openim-plugin .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
format:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Generate protobuf code (if protoc is available)
proto:
	@echo "Generating protobuf code..."
	@if command -v protoc >/dev/null 2>&1; then \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			conf/*.proto; \
	else \
		echo "protoc not found, skipping protobuf generation"; \
	fi

# Build and run integration example
example: build
	@echo "Running integration example..."
	./bin/openim-plugin

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Check for security vulnerabilities
security:
	@echo "Checking for security vulnerabilities..."
	go list -json -deps . | nancy sleuth

# Show plugin information
info:
	@echo "OpenIM Service Plugin Information:"
	@echo "  Version: v1.0.0"
	@echo "  Description: OpenIM instant messaging service plugin for lynx framework"
	@echo "  Configuration prefix: lynx.openim"
	@echo "  Plugin name: openim.service"
	@echo "  Weight: 15"

# Development mode - watch for changes and rebuild
dev:
	@echo "Starting development mode..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found, install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running in manual mode - use 'make build' to rebuild"; \
		while true; do \
			sleep 1; \
		done; \
	fi

# Docker build (if Dockerfile exists)
docker-build:
	@echo "Building Docker image..."
	@if [ -f Dockerfile ]; then \
		docker build -t lynx-openim-plugin .; \
	else \
		echo "Dockerfile not found"; \
	fi

# Docker run (if Dockerfile exists)
docker-run:
	@echo "Running Docker container..."
	@if [ -f Dockerfile ]; then \
		docker run -it --rm lynx-openim-plugin; \
	else \
		echo "Dockerfile not found"; \
	fi
