# OpenIM Service Plugin

The OpenIM Service Plugin is a comprehensive instant messaging service integration for the Lynx framework. It provides a complete solution for implementing real-time communication features in your applications.

## Features

### Core Messaging Capabilities
- **Text Messages**: Support for plain text messaging
- **Rich Media**: Support for images, voice, video, files, and custom message types
- **Group Chats**: Multi-user group conversations with role-based permissions
- **Private Chats**: One-to-one private messaging
- **Message History**: Persistent message storage and retrieval
- **Real-time Delivery**: WebSocket-based real-time message delivery

### Advanced Features
- **User Management**: User registration, authentication, and profile management
- **Online Status**: Real-time user online/offline status tracking
- **Message Encryption**: End-to-end encryption for secure communications
- **Push Notifications**: Mobile push notification support
- **Message Search**: Full-text search across message history
- **File Sharing**: Secure file upload, storage, and sharing

### Platform Support
- **Mobile**: iOS and Android native SDKs
- **Web**: React, Vue, Angular, and vanilla JavaScript support
- **Desktop**: Electron and native desktop applications
- **Cross-platform**: Flutter, React Native, and uni-app support

## Architecture

The plugin follows the Lynx framework's layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                    OpenIM Service Layer                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Client    │  │   Server    │  │   Message Router    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Storage Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │    Redis    │  │    MySQL    │  │      MongoDB        │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Network Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   HTTP/2    │  │  WebSocket  │  │       gRPC          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
lynx:
  openim:
    server:
      addr: "localhost:10002"
      api_version: "v3"
      platform_id: 1
      server_name: "OpenIM Server"
      log_level: "info"
    
    client:
      user_id: "test_user"
      token: "your_jwt_token_here"
      platform_id: 1
      server_addr: "localhost:10002"
      timeout: 30s
      heartbeat_interval: 30s
    
    security:
      tls_enable: false
      auth_enable: true
      jwt_secret: "your_jwt_secret_here"
      jwt_expire: 24h
    
    storage:
      type: "redis"
      addr: "localhost:6379"
      pool_size: 10
      timeout: 5s
```

### Advanced Configuration

See `conf/example_config.yml` for complete configuration options.

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/service/openim"
    "github.com/go-lynx/lynx/plugins/service/openim/conf"
)

func main() {
    // Get the OpenIM service instance
    openimService := openim.GetOpenIMService()
    
    // Send a text message
    msg := &conf.Message{
        Type:     "text",
        Content:  "Hello, World!",
        Sender:   "user1",
        Receiver: "user2",
    }
    
    ctx := context.Background()
    err := openimService.SendMessage(ctx, msg)
    if err != nil {
        panic(err)
    }
}
```

### Custom Message Handlers

```go
// Register custom message handler
openimService.RegisterMessageHandler("custom", func(ctx context.Context, msg *conf.Message) error {
    // Handle custom message type
    log.Infof("Custom message: %s", msg.Content)
    return nil
})

// Register custom event handler
openimService.RegisterEventHandler("user_joined", func(ctx context.Context, event interface{}) error {
    // Handle user joined event
    log.Infof("User joined: %v", event)
    return nil
})
```

### Client Operations

```go
// Get client instance
client := openimService.GetClient()

// Connect to server
err := client.Connect()

// Send message
err = client.SendMessage(msg)

// Disconnect
err = client.Disconnect()
```

### Server Operations

```go
// Get server instance
server := openimService.GetServer()

// Start server
err := server.Start()

// Stop server
err := server.Stop()
```

## API Reference

### ServiceOpenIM

The main service interface providing access to all OpenIM functionality.

#### Methods

- `GetClient() *OpenIMClient` - Returns the OpenIM client instance
- `GetServer() *OpenIMServer` - Returns the OpenIM server instance
- `GetConfig() *conf.OpenIM` - Returns the current configuration
- `SendMessage(ctx context.Context, msg *conf.Message) error` - Sends a message
- `RegisterMessageHandler(msgType string, handler MessageHandler)` - Registers a message handler
- `RegisterEventHandler(eventType string, handler EventHandler)` - Registers an event handler

### OpenIMClient

Client-side operations for connecting to and communicating with OpenIM servers.

### OpenIMServer

Server-side operations for managing OpenIM services.

### Configuration

See `conf/openim.go` for detailed configuration structure definitions.

## Message Types

### Supported Message Types

- **text**: Plain text messages
- **image**: Image messages with URL or base64 data
- **voice**: Voice messages with audio data
- **video**: Video messages with video data
- **file**: File messages with file metadata
- **location**: Geographic location messages
- **card**: Business card messages
- **quote**: Quoted message references
- **custom**: Custom message types for application-specific needs

### Message Structure

```go
type Message struct {
    Type      string `json:"type" yaml:"type"`
    Content   string `json:"content" yaml:"content"`
    Sender    string `json:"sender" yaml:"sender"`
    Receiver  string `json:"receiver" yaml:"receiver"`
    GroupID   string `json:"group_id" yaml:"group_id"`
    Sequence  int64  `json:"sequence" yaml:"sequence"`
    Timestamp int64  `json:"timestamp" yaml:"timestamp"`
    Status    string `json:"status" yaml:"status"`
}
```

## Events

### Supported Events

- **user_online**: User comes online
- **user_offline**: User goes offline
- **message_received**: New message received
- **message_delivered**: Message delivered to recipient
- **user_joined**: User joins a group
- **user_left**: User leaves a group
- **group_created**: New group created
- **group_deleted**: Group deleted

## Storage Backends

### Redis
- Fast in-memory storage for session data
- Message caching and temporary storage
- User online status tracking

### MySQL
- Persistent message storage
- User account management
- Group and relationship data

### MongoDB
- Document-based message storage
- Flexible schema for custom data
- Scalable storage for large message volumes

## Security

### Authentication
- JWT-based authentication
- Configurable token expiration
- Secure token storage

### Encryption
- TLS/SSL support for secure connections
- End-to-end message encryption
- Secure file transfer

### Access Control
- Role-based permissions
- Group access control
- User privacy settings

## Performance

### Scalability
- Horizontal scaling support
- Load balancing capabilities
- Connection pooling

### Optimization
- Message batching
- Efficient storage queries
- Caching strategies

## Monitoring

### Health Checks
- Service health monitoring
- Connection status tracking
- Performance metrics

### Metrics
- Message throughput
- Response times
- Error rates
- Resource usage

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Check server address and port
   - Verify network connectivity
   - Check firewall settings

2. **Authentication Errors**
   - Verify JWT token validity
   - Check token expiration
   - Validate user credentials

3. **Message Delivery Issues**
   - Check recipient online status
   - Verify message format
   - Check storage backend health

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
lynx:
  openim:
    server:
      log_level: "debug"
      log_with_stack: true
```

## Contributing

Contributions are welcome! Please see the main Lynx framework contribution guidelines.

## License

This plugin is part of the Lynx framework and follows the same license terms.

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Documentation](https://lynx.go-lynx.com)
- Community: [Lynx Community](https://community.go-lynx.com)
