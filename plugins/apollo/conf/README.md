# Apollo Plugin Configuration

This directory contains configuration-related files for the Apollo plugin.

## Files

- `apollo.proto` - Protobuf definition for Apollo plugin configuration
- `apollo.pb.go` - Generated Go code from apollo.proto (run `make config` to regenerate)
- `defaults.go` - Default configuration values and constants
- `example_config.yml` - Example configuration file

## Generating Protobuf Files

To regenerate the protobuf Go code, run:

```bash
# From project root
make config

# Or manually
protoc --proto_path=plugins/apollo/conf -I ./third_party --go_out=paths=source_relative:plugins/apollo/conf plugins/apollo/conf/apollo.proto
```

## Configuration Structure

The Apollo plugin configuration is defined in `apollo.proto` and includes:

- Basic configuration (app_id, cluster, namespace, meta_server)
- Authentication (token)
- Timeouts and intervals
- Feature flags (cache, metrics, retry, circuit breaker, etc.)
- Service configuration for multi-namespace loading

See `example_config.yml` for a complete configuration example.

