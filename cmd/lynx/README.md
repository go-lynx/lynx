# Lynx CLI Tool

<p align="center">
  <strong>Command Line Interface for Lynx Framework</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/go-lynx/lynx/cmd/lynx"><img src="https://pkg.go.dev/badge/github.com/go-lynx/lynx/cmd/lynx" alt="GoDoc"></a>
  <a href="https://github.com/go-lynx/lynx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-lynx/lynx" alt="License"></a>
</p>

---

## 🚀 Overview

The **Lynx CLI Tool** is a powerful command-line interface that simplifies the creation and management of Lynx microservices projects. Built with [Cobra](https://github.com/spf13/cobra), it provides an intuitive way to scaffold new Lynx services with best practices and project templates.

## ✨ Features

- **🚀 Quick Project Creation**: Generate new Lynx service projects in seconds
- **📁 Template-Based**: Uses proven project templates for consistent structure
- **🌍 Multi-Language Support**: Built-in internationalization (Chinese/English)
- **🔧 Configurable**: Customizable project generation with various flags
- **⚡ Fast & Efficient**: Optimized for quick project setup
- **🔄 Concurrent Processing**: Support for multiple project creation

## 🛠️ Installation

### Prerequisites

- Go 1.24.3 or higher
- Git (for cloning templates)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/go-lynx/lynx.git
cd lynx

# Build the CLI tool
go build -o lynx ./cmd/lynx

# Install globally (optional)
go install ./cmd/lynx
```

### Using Go Install

```bash
go install github.com/go-lynx/lynx/cmd/lynx@latest
```

## 📖 Usage

### Basic Commands

```bash
# Show help information
lynx --help

# Show version
lynx --version

# Create a new project
lynx new my-service
```

### Global Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--verbose` | `-v` | Enable verbose logs | `false` |
| `--quiet` | `-q` | Suppress non-error logs | `false` |
| `--log-level` | | Log level: error\|warn\|info\|debug | `info` |
| `--lang` | | Language for messages: zh\|en | `zh` |

### Project Creation

The main command `lynx new` creates a new Lynx service project:

```bash
# Create a project with default settings
lynx new my-service

# Create with custom module path
lynx new my-service --module github.com/myorg/myservice

# Create with specific template branch
lynx new my-service --branch develop

# Force overwrite existing directory
lynx new my-service --force

# Run go mod tidy after creation
lynx new my-service --post-tidy
```

#### New Command Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--repo-url` | `-r` | Layout repository URL | `https://github.com/go-lynx/lynx-layout.git` |
| `--branch` | `-b` | Repository branch | (from env) |
| `--ref` | | Repository reference (commit/tag/branch) | (from env) |
| `--timeout` | `-t` | Operation timeout | `60s` |
| `--module` | `-m` | Go module path for new project | (prompted) |
| `--force` | `-f` | Overwrite existing directory without prompt | `false` |
| `--post-tidy` | | Run 'go mod tidy' after creation | `false` |
| `--concurrency` | `-c` | Max concurrent project creations | `min(4, NumCPU*2)` |

## 🔧 Configuration

### Environment Variables

The CLI tool respects the following environment variables:

- `LYNX_LAYOUT_REPO`: Custom template repository URL
- `LYNX_LANG`: Default language setting
- `LYNX_LOG_LEVEL`: Default log level

### Template Repository

By default, the CLI uses the official Lynx layout template:
- **Repository**: `https://github.com/go-lynx/lynx-layout.git`
- **Customization**: Set `LYNX_LAYOUT_REPO` environment variable

## 📁 Project Structure

When you create a new project with `lynx new`, it generates a complete Lynx service structure:

```
my-service/
├── cmd/
│   └── server/
│       └── main.go
├── configs/
│   └── config.yaml
├── internal/
│   ├── biz/
│   ├── data/
│   ├── server/
│   └── service/
├── api/
├── go.mod
├── go.sum
└── README.md
```

## 🌍 Internationalization

The CLI supports multiple languages:

- **Chinese (zh)**: Default language with localized messages
- **English (en)**: International language support

Set language via:
```bash
# Command line
lynx --lang en

# Environment variable
export LYNX_LANG=en
```

## 🔍 Logging

Control logging verbosity with multiple options:

```bash
# Verbose logging (debug level)
lynx --verbose

# Quiet mode (error level only)
lynx --quiet

# Custom log level
lynx --log-level debug

# Environment variable
export LYNX_LOG_LEVEL=debug
```

## 🚀 Examples

### Create a Simple Service

```bash
# Basic project creation
lynx new user-service

# Interactive prompts will guide you through:
# - Project name
# - Module path
# - Template selection
```

### Create with Custom Configuration

```bash
# Custom module and template
lynx new payment-service \
  --module github.com/mycompany/payment \
  --repo-url https://github.com/mycompany/lynx-template.git \
  --branch custom-features \
  --force \
  --post-tidy
```

### Batch Project Creation

```bash
# Create multiple services
lynx new auth-service user-service payment-service \
  --module github.com/mycompany \
  --concurrency 3
```

## 🧪 Development

### Building

```bash
# Development build
go build -o lynx ./cmd/lynx

# Release build with version
go build -ldflags "-X 'main.release=v1.2.3'" -o lynx ./cmd/lynx
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/project
```

### Adding New Commands

The CLI is built with Cobra and easily extensible:

1. Create new command in `internal/` directory
2. Register in `main.go` init function
3. Follow existing command patterns

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](../../CONTRIBUTING.md) for details.

### Development Setup

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the [MIT License](../../LICENSE).

## 🔗 Related Links

- [Lynx Framework Documentation](https://go-lynx.cn/)
- [Lynx Layout Template](https://github.com/go-lynx/lynx-layout)
- [Go-Lynx Main Repository](https://github.com/go-lynx/lynx)
- [Cobra CLI Framework](https://github.com/spf13/cobra)

## 📞 Support

- **Discord**: [Join our community](https://discord.gg/2vq2Zsqq)
- **Issues**: [GitHub Issues](https://github.com/go-lynx/lynx/issues)
- **Documentation**: [https://go-lynx.cn/](https://go-lynx.cn/)

---

<p align="center">
  Made with ❤️ by the Lynx Community
</p>
