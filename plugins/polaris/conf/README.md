# Polaris Configuration Files

## Overview

Polaris is Tencent's open-source cloud-native service discovery and governance center. This plugin supports configuring Polaris SDK connection parameters through configuration files.

## Configuration File Structure

### 1. Lynx Application Configuration File

Add Polaris plugin configuration to your application configuration file:

```yaml
lynx:
  polaris:
    namespace: "default"                    # Namespace
    token: "your-polaris-token"            # Authentication token (optional)
    weight: 100                            # Service weight
    ttl: 30                                # Service TTL (seconds)
    timeout: "10s"                         # Operation timeout
    config_path: "./conf/polaris.yaml"     # SDK configuration file path (optional)
```

### 2. Polaris SDK Configuration File (polaris.yaml)

This is the standard Polaris SDK configuration file for configuring SDK connection parameters:

```yaml
global:
  serverConnector:
    protocol: grpc
    addresses:
      - 127.0.0.1:8091  # Polaris service address
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: 127.0.0.1:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - 127.0.0.1:8093  # Polaris config center address
```

## Configuration Items Description

### Lynx Configuration Items

- `namespace`: Polaris namespace for isolating resources from different environments or businesses
- `token`: Authentication token for accessing Polaris services (optional)
- `weight`: Service instance weight for load balancing
- `ttl`: Service instance TTL for heartbeat detection
- `timeout`: Timeout for Polaris service requests
- `config_path`: Path to Polaris SDK configuration file (optional)

### Polaris SDK Configuration Items

#### global Global Configuration
- `serverConnector`: Service connector configuration
  - `protocol`: Connection protocol (grpc/http)
  - `addresses`: List of Polaris service addresses
- `statReporter`: Statistics reporter configuration
  - `enable`: Whether to enable statistics reporting
  - `chain`: Statistics reporting chain
  - `plugin`: Statistics reporting plugin configuration

#### config Config Center
- `configConnector`: Config center connector
  - `addresses`: List of config center addresses

## Usage Examples

### 1. Basic Configuration

```yaml
# Application configuration file (config.yaml)
lynx:
  polaris:
    namespace: "default"
    config_path: "./conf/polaris.yaml"
```

```yaml
# Polaris SDK configuration file (conf/polaris.yaml)
global:
  serverConnector:
    protocol: grpc
    addresses:
      - 127.0.0.1:8091
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: 127.0.0.1:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - 127.0.0.1:8093
```

### 2. Production Environment Configuration

```yaml
# Application configuration file
lynx:
  polaris:
    namespace: "production"
    token: "your-production-token"
    weight: 100
    ttl: 30
    timeout: "5s"
    config_path: "./conf/polaris-prod.yaml"
```

```yaml
# Production environment Polaris configuration
global:
  serverConnector:
    protocol: grpc
    addresses:
      - polaris-server-1:8091
      - polaris-server-2:8091
      - polaris-server-3:8091
  statReporter:
    enable: true
    chain:
      - prometheus
    plugin:
      prometheus:
        type: push
        address: prometheus-server:9091
        interval: 10s

config:
  configConnector:
    addresses:
      - polaris-config-1:8093
      - polaris-config-2:8093
```

## Important Notes

1. **Configuration File Path**: Ensure the file path specified by `config_path` exists and is readable
2. **Service Addresses**: Modify service addresses according to your Polaris deployment
3. **Namespace**: Ensure you use the correct namespace
4. **Authentication Token**: It's recommended to use authentication tokens in production environments
5. **Network Connection**: Ensure the application can access Polaris services

## Reference Documentation

- [Tencent Polaris Official Documentation](https://polarismesh.cn/docs)
- [Polaris SDK Configuration Guide](https://polarismesh.cn/docs/使用指南/服务发现/服务发现SDK/Go-SDK/)
- [Polaris Deployment Guide](https://polarismesh.cn/docs/使用指南/服务发现/服务发现SDK/Go-SDK/)

## Troubleshooting

### Common Issues

1. **Configuration File Not Found**
   - Check if the `config_path` is correct
   - Ensure the file exists and has read permissions

2. **Connection Failed**
   - Check if Polaris service addresses are correct
   - Verify network connectivity
   - Validate authentication token

3. **Configuration Parsing Error**
   - Check if YAML format is correct
   - Verify configuration item names

### Debugging Methods

1. Check application logs to understand connection status
2. Use Polaris console to check service registration status
3. Validate configuration file format and content
