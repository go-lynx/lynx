# Lynx Swagger Plugin

## Overview

The Lynx Swagger plugin provides automated API documentation generation and Swagger UI service functionality for the Lynx microservices framework. By parsing annotations in code, it automatically generates Swagger documentation that conforms to OpenAPI 2.0 specifications and provides an interactive Web UI interface.

## Features

- 🚀 **Automatic Documentation Generation**: Scans code annotations to automatically generate Swagger JSON documentation
- 📝 **Annotation-Driven**: Supports rich Swagger annotation formats
- 🔄 **Hot Updates**: Automatically regenerates documentation when files change
- 🌐 **Built-in UI Service**: Provides Swagger UI interface with online debugging support
- ⚙️ **Flexible Configuration**: Supports multiple configuration options to meet different needs
- 🔌 **Plugin Design**: Seamlessly integrates with the Lynx framework

## Quick Start

### 1. Import Plugin

```go
import (
    _ "github.com/go-lynx/lynx/plugins/swagger"
)
```

### 2. Configuration File

Add Swagger configuration in `config.yaml`:

```yaml
lynx:
  swagger:
    enabled: true
    info:
      title: "My API"
      version: "1.0.0"
      description: "API description"
    gen:
      enabled: true
      auto_generate: true
      scan_dirs:
        - "./internal/controller"
      output_path: "./docs/swagger.json"
    ui:
      enabled: true
      port: 8080
      path: "/swagger"
```

### 3. Add Annotations

Add Swagger annotations to controller methods:

```go
// CreateUser creates a user
// @Summary Create new user
// @Description Create a new user account
// @Tags User Management
// @Accept json
// @Produce json
// @Param user body UserRequest true "User information"
// @Success 201 {object} UserResponse "Created successfully"
// @Failure 400 {object} ErrorResponse "Request parameter error"
// @Router /api/v1/users [post]
func (c *UserController) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation logic
}
```

### 4. Start Application

```go
func main() {
    app := lynx.New()
    app.Run(context.Background())
}
```

Visit `http://localhost:8080/swagger` to view the API documentation.

## Supported Annotations

### General Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| @Summary | Interface summary | `@Summary Create user` |
| @Description | Detailed description | `@Description Create a new user account` |
| @Tags | Interface grouping | `@Tags User Management` |
| @Accept | Request format | `@Accept json` |
| @Produce | Response format | `@Produce json` |
| @Router | Route information | `@Router /api/v1/users [post]` |

### Parameter Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| @Param | Parameter definition | `@Param id path int true "User ID"` |
| | | `@Param user body UserRequest true "User information"` |
| | | `@Param page query int false "Page number" default(1)` |
