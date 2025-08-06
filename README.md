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
┌─────────────────────────────────────────────────────────────┐
│                    Lynx Framework                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   HTTP      │  │   gRPC      │  │   Database  │       │
│  │  Plugin     │  │  Plugin     │  │   Plugin    │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │  Service    │  │   Rate      │  │ Distributed │       │
│  │ Discovery   │  │  Limiting   │  │ Transactions│       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Polaris   │  │   Seata     │  │   Kratos    │       │
│  │ (Discovery) │  │(Transactions)│  │ (Framework) │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
└─────────────────────────────────────────────────────────────┘
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

---

## 📊 Performance & Scalability

- **⚡ High Performance**: Optimized for low latency and high throughput
- **📈 Horizontal Scaling**: Easy scaling across multiple instances
- **🔄 Zero Downtime**: Rolling updates and graceful shutdowns
- **📊 Monitoring**: Built-in metrics and observability

---

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

## 🌟 Star History

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
