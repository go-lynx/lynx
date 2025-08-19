<p align="center">
  <a href="https://go-lynx.cn/" target="_blank">
    <img width="120" src="https://avatars.githubusercontent.com/u/150900434?s=250&u=8f8e9a5d1fab6f321b4aa350283197fc1d100efa&v=4" alt="Lynx Logo">
  </a>
</p>

<h1 align="center">Go-Lynx</h1>
<p align="center">
  <strong>即插即用的 Go 微服务框架</strong>
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

## 🚀 什么是 Lynx？

**Lynx** 是一款革命性的开源微服务框架，它彻底改变了开发者构建分布式系统的方式。基于 **Seata**、**Polaris** 和 **Kratos** 的坚实基础，Lynx 提供无缝的即插即用体验，让您专注于业务逻辑，而我们将处理基础设施的复杂性。

### 🎯 为什么选择 Lynx？

- **⚡ 零配置**：几分钟内即可开始，最小化设置
- **🔌 插件驱动**：模块化架构，支持热插拔组件
- **🛡️ 企业级就绪**：生产级可靠性和安全性
- **📈 可扩展**：专为高性能微服务构建
- **🔄 云原生**：专为现代云环境设计

---

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                    Lynx 框架                                │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   HTTP      │  │   gRPC      │  │   数据库    │       │
│  │   插件      │  │   插件      │  │   插件      │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │  服务发现   │  │   限流      │  │  分布式事务 │       │
│  │  与注册     │  │   熔断      │  │   管理      │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Polaris   │  │   Seata     │  │   Kratos    │       │
│  │ (服务发现)  │  │(分布式事务) │  │ (框架核心)  │       │
│  └─────────────┘  └─────────────┘  └─────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## ✨ 核心特性

### 🔍 服务发现与注册
- **自动服务注册**：无缝注册您的服务
- **智能服务发现**：动态服务发现，支持健康检查
- **多版本支持**：同时部署多个服务版本
- **负载均衡**：智能流量分发

### 🔐 安全与通信
- **加密内网通信**：服务间端到端加密
- **认证与授权**：内置安全机制
- **TLS 支持**：安全传输层通信

### 🚦 流量管理
- **限流控制**：智能限流防止服务过载
- **熔断器**：自动故障容错和恢复
- **流量路由**：智能路由，支持蓝绿和灰度部署
- **降级机制**：故障时优雅降级

### 💾 分布式事务
- **ACID 合规**：确保跨服务数据一致性
- **自动回滚**：优雅处理事务失败
- **性能优化**：分布式事务最小开销

### 🔌 插件架构
- **热插拔**：无需代码更改即可添加或移除功能
- **可扩展**：轻松集成第三方工具
- **模块化设计**：清晰的关注点分离

---

## 🛠️ 技术栈

Lynx 基于经过验证的开源技术：

| 组件 | 技术 | 用途 |
|------|------|------|
| **服务发现** | [Polaris](https://github.com/polarismesh/polaris) | 服务注册与发现 |
| **分布式事务** | [Seata](https://github.com/seata/seata) | 跨服务 ACID 事务 |
| **框架核心** | [Kratos](https://github.com/go-kratos/kratos) | 高性能微服务框架 |
| **开发语言** | [Go](https://golang.org/) | 快速、可靠、并发 |

---

## 🚀 快速开始

### 1. 安装 Lynx CLI
```bash
go install github.com/go-lynx/lynx/cmd/lynx@latest
```

### 2. 创建项目
```bash
# 创建单个项目
lynx new my-service

# 同时创建多个项目
lynx new service1 service2 service3
```

### 3. 编写代码
```go
package main

import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/app/boot"
)

func main() {
    // 就这么简单！Lynx 处理其余一切
    boot.LynxApplication(wireApp).Run()
}
```

### 4. 配置服务
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

## 📊 性能与可扩展性

- **⚡ 高性能**：针对低延迟和高吞吐量优化
- **📈 水平扩展**：轻松跨多个实例扩展
- **🔄 零停机**：滚动更新和优雅关闭
- **📊 监控**：内置指标和可观测性

---

## 🧰 CLI 日志与多语言（i18n）

Lynx CLI 提供统一分级日志与多语言消息输出。

### 日志
- 环境变量
  - `LYNX_LOG_LEVEL`：`error|warn|info|debug`（默认 `info`）
  - `LYNX_QUIET`：`1`/`true` 时仅输出错误
  - `LYNX_VERBOSE`：`1`/`true` 时启用更详细输出
- 命令行参数（优先于环境变量）
  - `--log-level <level>`
  - `--quiet` / `-q`
  - `--verbose` / `-v`

示例：
```bash
# 静默模式
LYNX_QUIET=1 lynx new demo

# 单次运行开启 debug 日志
lynx --log-level=debug new demo
```

### 多语言（i18n）
- 环境变量：`LYNX_LANG`，支持 `zh` 或 `en`
- 所有面向用户的提示与错误均遵循该设置

示例：
```bash
LYNX_LANG=en lynx new demo
LYNX_LANG=zh lynx new demo
```

## 🧭 CLI：lynx new（常用参数）

常用参数：
- `--repo-url, -r`：模板仓库地址（可用环境变量 `LYNX_LAYOUT_REPO` 指定）
- `--branch, -b`：模板仓库分支
- `--ref`：统一指定 commit/tag/branch；优先级高于 `--branch`
- `--module, -m`：新项目的 Go module（如 `github.com/acme/foo`）
- `--force, -f`：覆盖已存在目录且不提示
- `--post-tidy`：生成完成后自动执行 `go mod tidy`
- `--timeout, -t`：创建超时时间（如 `60s`）
- `--concurrency, -c`：并发创建项目的最大数量

示例：
```bash
# 指定 tag 生成
lynx new demo --ref v1.2.3

# 指定 module 并自动 tidy
lynx new demo -m github.com/acme/demo --post-tidy

# 并发创建 4 个项目
lynx new svc-a svc-b svc-c svc-d -c 4
```

## 🎯 应用场景

### 🏢 企业应用
- **微服务迁移**：遗留系统现代化
- **云原生应用**：Kubernetes 和容器原生部署
- **高流量服务**：电商和金融应用

### 🚀 创业公司与成长型公司
- **快速开发**：最小设置快速上市
- **成本优化**：高效资源利用
- **团队生产力**：专注于业务逻辑，而非基础设施

---

## 🤝 贡献

我们欢迎贡献！详情请参阅我们的[贡献指南](CONTRIBUTING.md)。

### 🐛 报告 Bug
发现 Bug？请[提交 Issue](https://github.com/go-lynx/lynx/issues)。

### 💡 建议功能
有想法？我们很乐意听到！[开始讨论](https://github.com/go-lynx/lynx/discussions)。

---

## 📚 文档

- 📖 [用户指南](https://go-lynx.cn/docs)
- 🔧 [API 参考](https://pkg.go.dev/github.com/go-lynx/lynx)
- 🎯 [示例](https://github.com/go-lynx/lynx/examples)
- 🚀 [快速开始](https://go-lynx.cn/docs/quick-start)

---

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

---

## 🌟 Star 历史

[![Star History Chart](https://api.star-history.com/svg?repos=go-lynx/lynx&type=Date)](https://star-history.com/#go-lynx/lynx&Date)

---

<div align="center">
  <p><strong>加入数千名开发者，用 Lynx 构建未来！🚀</strong></p>
  <p>
    <a href="https://discord.gg/2vq2Zsqq">💬 Discord</a> •
    <a href="https://go-lynx.cn/">🌐 官网</a> •
    <a href="https://github.com/go-lynx/lynx/issues">🐛 Issues</a> •
    <a href="https://github.com/go-lynx/lynx/discussions">💡 讨论</a>
  </p>
</div>
