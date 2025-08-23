# TLS Module for Lynx Framework

This module provides enhanced TLS certificate management for the Lynx framework, supporting multiple certificate sources, file monitoring, and hot reloading.

## Features

### 🚀 **Multi-Source Certificate Loading**
- **Local File Source**: Load certificates from local files (recommended for production)
- **Memory Source**: Load certificates from memory content (useful for testing)
- **Control Plane Source**: Load certificates from control plane (legacy support)

### 🔄 **File Monitoring & Hot Reload**
- Real-time monitoring of certificate file changes
- Automatic certificate reloading without service restart
- Configurable monitoring intervals
- MD5 hash-based change detection for reliability

### ⚙️ **Advanced Configuration**
- Flexible TLS configuration options
- Support for different authentication types
- Configurable TLS versions and cipher suites
- Session cache management
- Hostname verification options

### 🛡️ **Security & Validation**
- Comprehensive configuration validation
- Certificate format validation
- File accessibility checks
- Error handling and logging

## Quick Start

### 1. Basic Local File Configuration

```yaml
lynx:
  tls:
    source_type: "local_file"
    local_file:
      cert_file: "/etc/ssl/certs/server.crt"
      key_file: "/etc/ssl/private/server.key"
      root_ca_file: "/etc/ssl/certs/ca.crt"
      watch_files: true
      reload_interval: "5s"
```

### 2. Memory Source Configuration

```yaml
lynx:
  tls:
    source_type: "memory"
    memory:
      cert_data: |
        -----BEGIN CERTIFICATE-----
        Your certificate content here...
        -----END CERTIFICATE-----
      key_data: |
        -----BEGIN PRIVATE KEY-----
        Your private key content here...
        -----END PRIVATE KEY-----
```

### 3. Advanced Configuration

```yaml
lynx:
  tls:
    source_type: "local_file"
    local_file:
      cert_file: "/etc/ssl/certs/domain.crt"
      key_file: "/etc/ssl/private/domain.key"
      root_ca_file: "/etc/ssl/certs/chain.crt"
      watch_files: true
      reload_interval: "10s"
      cert_format: "pem"
    common:
      auth_type: 4  # Require and verify client certificate
      verify_hostname: true
      min_tls_version: "1.3"
      session_cache_size: 128
```

## Configuration Reference

### Source Types

| Source Type | Description | Use Case |
|-------------|-------------|----------|
| `local_file` | Load from local files | Production, development |
| `memory` | Load from memory content | Testing, embedded |
| `control_plane` | Load from control plane | Legacy systems |

### Authentication Types

| Type | Value | Description |
|------|-------|-------------|
| No Client Auth | 0 | No client authentication required |
| Request Client Cert | 1 | Request but don't require |
| Require Any Client Cert | 2 | Accept any client certificate |
| Verify Client Cert | 3 | Verify if provided |
| Require & Verify | 4 | Require and verify client certificate |

> **Note**: These values are defined in the protobuf configuration and represent standard TLS client authentication types.

### TLS Versions

| Version | Description | Security |
|---------|-------------|----------|
| 1.0 | TLS 1.0 | ❌ Not recommended |
| 1.1 | TLS 1.1 | ❌ Not recommended |
| 1.2 | TLS 1.2 | ✅ Recommended minimum |
| 1.3 | TLS 1.3 | ✅ Recommended, most secure |

> **Note**: These version strings are used directly in configuration and represent the minimum TLS version to support.

## File Monitoring

The file monitoring system provides real-time certificate updates:

- **Change Detection**: Monitors file modification time, size, and MD5 hash
- **Hot Reload**: Automatically reloads certificates when changes are detected
- **Configurable Intervals**: Set monitoring frequency (1s to 5 minutes)
- **Non-blocking**: Change notifications don't block the main application

### Monitoring Configuration

```yaml
local_file:
  watch_files: true           # Enable file monitoring
  reload_interval: "5s"       # Check for changes every 5 seconds
```

## Error Handling

The module provides comprehensive error handling:

- **Configuration Validation**: Validates all configuration parameters
- **File Validation**: Checks file existence and accessibility
- **Certificate Validation**: Validates certificate format and compatibility
- **Graceful Degradation**: Falls back to legacy methods when possible

## Performance Considerations

- **Lazy Loading**: Certificates are loaded only when needed
- **Efficient Monitoring**: Uses lightweight file stat operations
- **Hash-based Detection**: MD5 hashing for reliable change detection
- **Configurable Caching**: Session cache size can be tuned

## Security Best Practices

1. **Use Strong TLS Versions**: Prefer TLS 1.2 or 1.3
2. **Enable File Monitoring**: Use `watch_files: true` for production
3. **Secure File Permissions**: Ensure certificate files have proper permissions
4. **Regular Updates**: Keep certificates up to date
5. **Validation**: Enable hostname verification in production

## Migration from Legacy

The module maintains full backward compatibility:

```yaml
# Old configuration (still works)
lynx:
  tls:
    file_name: "tls-config"
    group: "security"

# New configuration (recommended)
lynx:
  tls:
    source_type: "control_plane"
    file_name: "tls-config"
    group: "security"
```

## Troubleshooting

### Common Issues

1. **File Not Found**: Check file paths and permissions
2. **Permission Denied**: Ensure application has read access to certificate files
3. **Invalid Format**: Verify certificate files are in correct PEM/DER format
4. **Monitoring Failures**: Check file watcher permissions and disk space

### Debug Logging

Enable debug logging to troubleshoot issues:

```go
import "github.com/go-lynx/lynx/app/log"

// Set log level to debug
log.SetLevel(log.DebugLevel)
```

## Examples

See `conf/example_config.yml` for complete configuration examples.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This module is part of the Lynx framework and follows the same license terms.
