<p align="center">
  <a href="https://go-lynx.cn/" target="_blank">
    <img width="120" src="https://avatars.githubusercontent.com/u/150900434?s=250&u=8f8e9a5d1fab6f321b4aa350283197fc1d100efa&v=4" alt="Lynx Logo">
  </a>
</p>

<h1 align="center">Go-Lynx</h1>
<p align="center">
  <strong>The Plug-and-Play Go Microservices Framework</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/go-lynx/lynx"><img src="https://pkg.go.dev/badge/github.com/go-lynx/lynx/v2" alt="GoDoc"></a>
  <a href="https://codecov.io/gh/go-lynx/lynx"><img src="https://codecov.io/gh/go-lynx/lynx/master/graph/badge.svg" alt="codeCov"></a>
  <a href="https://goreportcard.com/report/github.com/go-lynx/lynx"><img src="https://goreportcard.com/badge/github.com/go-lynx/lynx" alt="Go Report Card"></a>
  <a href="https://github.com/go-lynx/lynx/blob/main/LICENSE"><img src="https://img.shields.io/github/license/go-lynx/lynx" alt="License"></a>
  <a href="https://discord.gg/2vq2Zsqq"><img src="https://img.shields.io/discord/1174545542689337497?label=chat&logo=discord" alt="Discord"></a>
  <a href="https://github.com/go-lynx/lynx/releases"><img src="https://img.shields.io/github/v/release/go-lynx/lynx" alt="Release"></a>
  <a href="https://github.com/go-lynx/lynx/stargazers"><img src="https://img.shields.io/github/stars/go-lynx/lynx" alt="Stars"></a>
</p>

## 📊 Event System

Lynx provides a unified event system for inter-plugin communication:

### Event Types

Lynx supports various event types:

```go
// Add event listener
listener := &MyEventListener{}
runtime.AddListener(listener, nil)

// Add plugin-specific listener
runtime.AddPluginListener("my-plugin", listener, nil)

// Emit events
runtime.EmitPluginEvent("my-plugin", "started", map[string]any{
    "timestamp": time.Now().Unix(),
})
```

### Event Filtering

Lynx supports event filtering to process only relevant events:

```yaml
event_filters:
  - type: "started"
    plugin: "http"
  - type: "stopped"
    plugin: "grpc"
```

## 📈 Monitoring and Observability

Lynx provides comprehensive monitoring and observability features:

### Metrics

Lynx integrates with Prometheus for metrics collection:

```yaml
metrics:
  enabled: true
  endpoint: "/metrics"
  namespace: "lynx"
  subsystem: "http"
  labels:
    - "service"
    - "instance"
```

### Tracing

Lynx integrates with OpenTelemetry for distributed tracing:

```yaml
tracing:
  enabled: true
  provider: "otlp"
  endpoint: "localhost:4317"
  service_name: "demo"
  sample_rate: 0.1
```

### Logging

Lynx provides structured logging with Zap:

```yaml
logging:
  level: "info"
  format: "json"
  output: "stdout"
  caller: true
  stacktrace: true
```

### Health Checks

Lynx includes health checks for monitoring service health:

```yaml
health:
  enabled: true
  endpoint: "/health"
  checks:
    - name: "database"
      timeout: 5s
    - name: "redis"
      timeout: 2s
```

## 🚀 Production-Ready Features

Lynx includes several production-ready features:

### Graceful Shutdown

Lynx supports graceful shutdown to ensure all requests are processed before shutting down:

```yaml
graceful_shutdown:
  timeout: 30s
  wait_for_ongoing_requests: true
  max_wait_time: 60s
```

### Rate Limiting

Lynx includes rate limiting to prevent abuse:

```yaml
rate_limit:
  enabled: true
  rate_per_second: 100
  burst_limit: 200
```

### Retry Policies

Lynx supports retry policies for handling transient failures:

```yaml
retry:
  enabled: true
  max_attempts: 3
  initial_interval: 100ms
  max_interval: 1s
  multiplier: 2.0
```

### Dead Letter Queues

Lynx supports dead letter queues for handling failed messages:

```yaml
dead_letter_queue:
  enabled: true
  max_retries: 3
  destination: "dlq-topic"
```

---

Translations: [English](README.md) | [简体中文](README_zh.md)


## 🚀 What is Lynx?

**Lynx** is a revolutionary open-source microservices framework that transforms the way developers build distributed systems. Built on the robust foundations of **Seata**, **Polaris**, and **Kratos**, Lynx delivers a seamless plug-and-play experience that lets you focus on business logic while we handle the infrastructure complexity.

### 🎯 Why Choose Lynx?

- **⚡ Zero Configuration**: Get started in minutes with minimal setup
- **🔌 Plugin-Driven**: Modular architecture with hot-pluggable components
- **🛡️ Enterprise Ready**: Production-grade reliability and security
- **📈 Scalable**: Built for high-performance microservices
- **🔄 Cloud Native**: Designed for modern cloud environments

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Lynx Application Layer                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ LynxApp     │  │ Boot        │  │ Control     │           │
│  │             │  │             │  │ Plane       │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Plugin Management Layer                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Plugin      │  │ TypedPlugin │  │ Plugin      │           │
│  │ Manager     │  │ Manager     │  │ Factory     │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Runtime Layer                               │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Runtime     │  │ TypedRuntime│  │ Simple      │           │
│  │ Interface   │  │ Impl        │  │ Runtime     │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                Resource Management Layer                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Private     │  │ Shared      │  │ Resource    │           │
│  │ Resources   │  │ Resources   │  │ Info        │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

---

## ✨ Key Features

### 🔍 Service Discovery & Registration
- **Automatic Service Registration**: Seamlessly register your services
- **Smart Service Discovery**: Dynamic service discovery with health checks
- **Multi-Version Support**: Deploy multiple service versions simultaneously
- **Load Balancing**: Intelligent traffic distribution

### 🔐 Security & Communication
- **Encrypted Intranet Communication**: End-to-end encryption for service communication
- **Authentication & Authorization**: Built-in security mechanisms
- **TLS Support**: Secure transport layer communication

### 🚦 Traffic Management
- **Rate Limiting**: Prevent service overload with intelligent throttling
- **Circuit Breaker**: Automatic fault tolerance and recovery
- **Traffic Routing**: Intelligent routing with blue-green and canary deployments
- **Fallback Mechanisms**: Graceful degradation during failures

### 💾 Distributed Transactions
- **ACID Compliance**: Ensure data consistency across services
- **Automatic Rollback**: Handle transaction failures gracefully
- **Performance Optimized**: Minimal overhead for distributed transactions

### 🔌 Plugin Architecture
- **Hot-Pluggable**: Add or remove features without code changes
- **Extensible**: Easy integration of third-party tools
- **Modular Design**: Clean separation of concerns

---

## 🛠️ Built With

Lynx leverages battle-tested open-source technologies:

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Service Discovery** | [Polaris](https://github.com/polarismesh/polaris) | Service registration and discovery |
| **Distributed Transactions** | [Seata](https://github.com/seata/seata) | ACID transactions across services |
| **Framework Core** | [Kratos](https://github.com/go-kratos/kratos) | High-performance microservices framework |
| **Language** | [Go](https://golang.org/) | Fast, reliable, and concurrent |

---

## 🚀 Quick Start

### 1. Install Lynx CLI
```bash
go install github.com/go-lynx/lynx/cmd/lynx@latest
```

### 2. Create Your Project
```bash
# Create a single project
lynx new my-service

# Create multiple projects at once
lynx new service1 service2 service3
```

### 3. Write Your Code
```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/app/boot"
)

func main() {
    // That's it! Lynx handles everything else
    boot.LynxApplication(wireApp).Run()
}
```

### 4. Configure Your Services
```yaml
# config.yml
lynx:
  polaris:
    namespace: "default"
    weight: 100
  http:
    addr: ":8080"
    timeout: "10s"
  grpc:
    addr: ":9090"
    timeout: "5s"
```

## 🔐 Security

Lynx provides comprehensive security features to protect your microservices:

### TLS Configuration

Lynx supports TLS for secure communication between services:

```yaml
tls:
  source_type: local_file  # Options: local_file, memory, etc.
  auth_type: mutual        # Options: no_client_cert, request_client_cert, require_any_client_cert, etc.
  min_version: TLS1.2      # Minimum TLS version
  cert_file: /path/to/cert.pem
  key_file: /path/to/key.pem
  ca_file: /path/to/ca.pem
```

### Authentication

Lynx supports various authentication mechanisms:

- **TLS Mutual Authentication**: Client and server authenticate each other using certificates
- **OAuth2**: Support for OAuth2 authentication flows
- **API Keys**: Simple API key authentication
- **JWT**: JSON Web Token authentication

### Authorization

Lynx provides flexible authorization mechanisms:

- **Role-Based Access Control (RBAC)**: Control access based on roles
- **Attribute-Based Access Control (ABAC)**: Fine-grained access control based on attributes
- **Policy Enforcement**: Enforce access policies at the service level

## ⚠️ Error Handling

Lynx provides a comprehensive error handling framework:

### Error Types

Lynx defines common error types for consistent error handling:

```go
// Common errors
var (
    ErrCacheMiss  = errors.New("cache: key not found")
    ErrCacheSet   = errors.New("cache: failed to set value")
    ErrInvalidTTL = errors.New("cache: invalid TTL")
)
```

### Error Recovery

Lynx includes an error recovery manager for handling and recovering from errors:

```go
// ErrorRecoveryManager provides centralized error handling and recovery
type ErrorRecoveryManager struct {
    // Error tracking
    errorCounts     map[string]int64
    errorHistory    []ErrorRecord
    recoveryHistory []RecoveryRecord

    // Circuit breakers for different error types
    circuitBreakers map[string]*CircuitBreaker

    // Recovery strategies
    recoveryStrategies map[string]RecoveryStrategy
    
    // ... other fields
}
```

### Circuit Breakers

Lynx includes circuit breakers to prevent cascading failures:

```yaml
circuit_breaker:
  enabled: true
  threshold: 5        # Number of errors before opening the circuit
  timeout: 30s        # Time to keep the circuit open
  half_open_timeout: 5s  # Time to wait in half-open state
```

---

## 📊 Performance & Scalability

- **⚡ High Performance**: Optimized for low latency and high throughput
- **📈 Horizontal Scaling**: Easy scaling across multiple instances
- **🔄 Zero Downtime**: Rolling updates and graceful shutdowns
- **📊 Monitoring**: Built-in metrics and observability

---

## 🧰 CLI Logging & I18n

Lynx CLI provides a unified, level-based logger and internationalized messages.

### Logging
- Env vars
  - `LYNX_LOG_LEVEL`: one of `error|warn|info|debug` (default: `info`)
  - `LYNX_QUIET`: suppress non-error outputs when set to `1`/`true`
  - `LYNX_VERBOSE`: enable verbose mode when set to `1`/`true`
- Flags (override env for current command)
  - `--log-level <level>`
  - `--quiet` / `-q`
  - `--verbose` / `-v`

Examples:
```bash
# quiet mode
LYNX_QUIET=1 lynx new demo

# debug logs for one run
lynx --log-level=debug new demo
```

### Internationalization (i18n)
- Env var: `LYNX_LANG` with `zh` or `en`.
- All user-facing messages respect this setting.

Examples:
```bash
LYNX_LANG=en lynx new demo
LYNX_LANG=zh lynx new demo
```

## 🧭 CLI Commands

### 📋 lynx new - Create New Projects

Common flags for `lynx new`:
- `--repo-url, -r`: layout repository URL (env: `LYNX_LAYOUT_REPO`)
- `--branch, -b`: branch name for layout repo
- `--ref`: commit/tag/branch to checkout; takes precedence over `--branch`
- `--module, -m`: go module path for the new project (e.g. `github.com/acme/foo`)
- `--force, -f`: overwrite existing directory without prompt
- `--post-tidy`: run `go mod tidy` after generation
- `--timeout, -t`: creation timeout (e.g. `60s`)
- `--concurrency, -c`: max concurrent project creations

Examples:
```bash
# use a specific tag
lynx new demo --ref v1.2.3

# set module and run mod tidy automatically
lynx new demo -m github.com/acme/demo --post-tidy

# create multiple projects with concurrency 4
lynx new svc-a svc-b svc-c svc-d -c 4
```

### 🔍 lynx doctor - Diagnose Environment & Project Health

The `lynx doctor` command performs comprehensive health checks on your development environment and Lynx project.

#### What It Checks

**Environment Checks:**
- ✅ Go installation and version (minimum Go 1.20+)
- ✅ Go environment variables (GOPATH, GO111MODULE, GOPROXY)
- ✅ Git repository status and uncommitted changes

**Tool Checks:**
- ✅ Protocol Buffers compiler (protoc) installation
- ✅ Wire dependency injection tool availability
- ✅ Required development tools for Lynx projects

**Project Structure:**
- ✅ Validates expected directory structure (app/, boot/, plugins/, etc.)
- ✅ Checks go.mod file existence and validity
- ✅ Verifies Makefile and expected targets

**Configuration:**
- ✅ Scans and validates YAML/YML configuration files
- ✅ Checks configuration syntax and structure

#### Output Formats

- **Text** (default): Human-readable with colors and icons
- **JSON**: Machine-readable for CI/CD integration
- **Markdown**: Documentation-friendly format

#### Command Options

```bash
# Run all diagnostic checks
lynx doctor

# Output in JSON format (for CI/CD)
lynx doctor --format json

# Output in Markdown format
lynx doctor --format markdown > health-report.md

# Check specific category only
lynx doctor --category env      # Environment only
lynx doctor --category tools    # Tools only
lynx doctor --category project  # Project structure only
lynx doctor --category config   # Configuration only

# Auto-fix issues when possible
lynx doctor --fix

# Show detailed diagnostic information
lynx doctor --verbose
```

#### Auto-Fix Capabilities

The `--fix` flag can automatically resolve:
- Missing development tools (installs via `make init` or `go install`)
- go.mod issues (runs `go mod tidy`)
- Other fixable configuration problems

#### Health Status Indicators

- 💚 **Healthy**: All checks passed
- 💛 **Degraded**: Some warnings detected but functional
- 🔴 **Critical**: Errors found that need attention

#### Example Output

```
🔍 Lynx Doctor - Diagnostic Report
==================================================

📊 System Information:
  • OS/Arch: darwin/arm64
  • Go Version: go1.24.4
  • Lynx Version: v2.0.0

🔎 Diagnostic Checks:
--------------------------------------------------
✅ Go Version: Go 1.24 installed
✅ Project Structure: All expected directories found
⚠️ Wire Dependency Injection: Not installed
   💡 Fix available (use --fix to apply)

📈 Summary:
  Total Checks: 9
  ✅ Passed: 7
  ⚠️ Warnings: 2

💛 Overall Health: Degraded
```

### 🚀 lynx run - Quick Development Server

The `lynx run` command provides a convenient way to build and run your Lynx project with optional hot reload for rapid development.

#### Features

- **Automatic Build & Run**: Compiles and executes your project in one command
- **Hot Reload**: Automatically rebuilds and restarts on file changes (with `--watch` flag)
- **Process Management**: Graceful shutdown and restart handling
- **Smart Detection**: Automatically finds main package in project structure
- **Environment Control**: Pass custom environment variables and arguments

#### Command Options

```bash
lynx run [path] [flags]
```

**Flags:**
- `--watch, -w`: Enable hot reload (watch for file changes)
- `--build-args`: Additional arguments for go build
- `--run-args`: Arguments to pass to the running application
- `--verbose, -v`: Enable verbose output
- `--env, -e`: Environment variables (KEY=VALUE)
- `--port, -p`: Override the application port
- `--skip-build`: Skip build and run existing binary

#### Example Usage

```bash
# Run project in current directory
lynx run

# Run with hot reload (auto-restart on file changes)
lynx run --watch

# Run specific project directory
lynx run ./my-service

# Pass custom build flags
lynx run --build-args="-ldflags=-s -w"

# Pass runtime configuration
lynx run --run-args="--config=./configs"

# Set environment variables
lynx run -e PORT=8080 -e ENV=development

# Run existing binary without rebuild
lynx run --skip-build
```

#### Hot Reload Details

When using `--watch` mode, the following files trigger rebuilds:
- Go source files (`.go`)
- Go module files (`go.mod`, `go.sum`)
- Configuration files (`.yaml`, `.yml`, `.json`, `.toml`)
- Environment files (`.env`)
- Protocol buffer files (`.proto`)

Ignored paths:
- `.git`, `.idea`, `vendor`, `node_modules`
- Build directories (`bin`, `dist`, `tmp`)
- Test files (`*_test.go`)

## 🎯 Use Cases

### 🏢 Enterprise Applications
- **Microservices Migration**: Legacy system modernization
- **Cloud-Native Applications**: Kubernetes and container-native deployments
- **High-Traffic Services**: E-commerce and financial applications

### 🚀 Startups & Scale-ups
- **Rapid Development**: Quick time-to-market with minimal setup
- **Cost Optimization**: Efficient resource utilization
- **Team Productivity**: Focus on business logic, not infrastructure

---

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### 🐛 Report Bugs
Found a bug? Please [open an issue](https://github.com/go-lynx/lynx/issues).

### 💡 Suggest Features
Have an idea? We'd love to hear it! [Start a discussion](https://github.com/go-lynx/lynx/discussions).

---

## 📚 Documentation

- 📖 [User Guide](https://go-lynx.cn/docs)
- 🔧 [API Reference](https://pkg.go.dev/github.com/go-lynx/lynx)
- 🎯 [Examples](https://github.com/go-lynx/lynx/examples)
- 🚀 [Quick Start](https://go-lynx.cn/docs/quick-start)

---

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

---

## 🤝 Contributing

We welcome contributions to Lynx! Here's how you can help:

1. **Report Issues**: Report bugs or suggest features by opening an issue.
2. **Submit Pull Requests**: Submit PRs for bug fixes or new features.
3. **Improve Documentation**: Help improve documentation or add examples.
4. **Spread the Word**: Star the repository and share it with others.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## 📚 Documentation

For more detailed documentation, please visit:

- [Lynx Documentation](https://lynx.go-lynx.com)
- [API Reference](https://pkg.go.dev/github.com/go-lynx/lynx)
- [Examples](https://github.com/go-lynx/lynx/tree/main/examples)

## 📜 License

Lynx is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for the full license text.

## ⭐ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=go-lynx/lynx&type=Date)](https://star-history.com/#go-lynx/lynx&Date)

---

<div align="center">
  <p><strong>Join thousands of developers building the future with Lynx! 🚀</strong></p>
  <p>
    <a href="https://discord.gg/2vq2Zsqq">💬 Discord</a> •
    <a href="https://go-lynx.cn/">🌐 Website</a> •
    <a href="https://github.com/go-lynx/lynx/issues">🐛 Issues</a> •
    <a href="https://github.com/go-lynx/lynx/discussions">💡 Discussions</a>
  </p>
</div>
