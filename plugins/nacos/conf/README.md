# Nacos Plugin Configuration

This directory contains configuration-related files for the Nacos plugin.

## Files

- `nacos.proto` - Protobuf definition for Nacos plugin configuration
- `nacos.pb.go` - Generated Go code from nacos.proto (run `make config` to regenerate)
- `defaults.go` - Default configuration values and constants
- `example_config.yml` - Example configuration file

## Generating Protobuf Files

To regenerate the protobuf Go code, run:

```bash
# From project root
make config

# Or manually from nacos plugin directory
protoc --proto_path=plugins/nacos/conf \
  -I ./third_party \
  -I ./boot \
  -I ./app \
  --go_out=paths=source_relative:plugins/nacos/conf \
  plugins/nacos/conf/nacos.proto
```

## Configuration Structure

The Nacos plugin configuration is defined in `nacos.proto` and includes:

- **Server Configuration**: server_addresses, endpoint, namespace
- **Authentication**: username/password or access_key/secret_key
- **Service Configuration**: service registration and discovery settings
- **Configuration Management**: multi-config loading support
- **Connection Settings**: timeout, logging, cache directories

See `example_config.yml` for a complete configuration example.

