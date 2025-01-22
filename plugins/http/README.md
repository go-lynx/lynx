# HTTP Plugin for Lynx Framework

This plugin provides HTTP server functionality for the Lynx framework.

## Installation

```bash
go get github.com/go-lynx/plugin-http
```

## Features

- HTTP/HTTPS server support
- Middleware integration (tracing, logging, rate limiting, validation)
- TLS support
- Custom response encoding
- Health checking
- Event emission

## Configuration

The plugin can be configured through the Lynx configuration system. Here's an example configuration:

```yaml
lynx:
  http:
    network: tcp
    addr: :8080
    timeout: 1s
    tls:
      enabled: false
      cert: path/to/cert.pem
      key: path/to/key.pem
```

## Usage

```go
import (
    "github.com/go-lynx/plugin-http"
    "github.com/go-lynx/plugin-http/conf"
)

func main() {
    // Create HTTP plugin with custom configuration
    httpPlugin := http.New(
        http.Weight(100),
        http.Config(&conf.Http{
            Network: "tcp",
            Addr:    ":8080",
        }),
    )

    // Register the plugin with Lynx
    app.Lynx().RegisterPlugin(httpPlugin)
}
```

## Events

The plugin emits the following events:

- `EventPluginStarted`: When the HTTP server is successfully initialized
- `EventPluginStopping`: When the server is about to stop
- `EventPluginStopped`: When the server has been stopped

## Health Checks

The plugin provides health check information including:
- Server status
- Address and network configuration
- TLS status
- Current connections (if available)

## Dependencies

- github.com/go-kratos/kratos/v2 v2.8.3
- github.com/go-lynx/lynx v1.0.0

## License

Apache License 2.0
