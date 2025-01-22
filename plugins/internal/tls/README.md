# Certificate Plugin for Lynx Framework

This plugin provides certificate management functionality for the Lynx framework.

## Features

- TLS certificate management
- Certificate loading from configuration
- Support for root CA
- Dynamic certificate updates

## Configuration

The plugin can be configured through the Lynx configuration system. Here's an example configuration:

```yaml
lynx:
  application:
    tls:
      fileName: "cert"
      group: "DEFAULT_GROUP"
```

## Dependencies

- github.com/go-kratos/kratos/v2
- github.com/go-lynx/lynx
- entgo.io/ent
