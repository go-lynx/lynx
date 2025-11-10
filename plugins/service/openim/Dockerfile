# OpenIM Service Plugin Dockerfile
# Multi-stage build for optimal image size

# Build stage
FROM golang:1.25.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the plugin
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o openim-plugin .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S openim && \
    adduser -u 1001 -S openim -G openim

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/openim-plugin .

# Copy configuration files
COPY --from=builder /app/conf/example_config.yml ./conf/

# Change ownership to non-root user
RUN chown -R openim:openim /app

# Switch to non-root user
USER openim

# Expose default OpenIM port
EXPOSE 10002

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:10002/health || exit 1

# Default command
CMD ["./openim-plugin"]

# Labels
LABEL maintainer="Lynx Framework Team"
LABEL description="OpenIM Service Plugin for Lynx Framework"
LABEL version="v1.0.0"
LABEL org.opencontainers.image.source="https://github.com/go-lynx/lynx"
LABEL org.opencontainers.image.description="OpenIM instant messaging service plugin"
LABEL org.opencontainers.image.licenses="MIT"
