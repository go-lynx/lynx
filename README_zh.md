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
┌─────────────────────────────────────────────────────────────────┐
│                    Lynx 应用层                                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ LynxApp     │  │ Boot        │  │ Control     │           │
│  │             │  │             │  │ Plane       │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    插件管理层                                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Plugin      │  │ Plugin      │  │ Plugin      │           │
│  │ Manager     │  │ Lifecycle   │  │ Topology    │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    运行时层                                    │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Runtime     │  │ Event       │  │ Config      │           │
│  │ Interface   │  │ System      │  │ Provider    │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    资源管理层                                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │
│  │ Private     │  │ Shared      │  │ Resource    │           │
│  │ Resources   │  │ Resources   │  │ Info        │           │
│  └─────────────┘  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────────────────────────┘
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

### 🛠️ 错误处理与恢复
- **集中式错误管理**：统一的错误处理框架
- **自动恢复机制**：智能错误恢复策略
- **熔断保护**：防止级联故障
- **错误分类**：精确的错误类型和严重性分级

### 📡 事件系统
- **事件驱动架构**：基于事件的组件通信
- **事件过滤**：灵活的事件订阅机制
- **事件历史**：可追踪的事件记录
- **插件事件**：插件生命周期事件管理

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

### 5. 安全配置

#### TLS 配置
```yaml
lynx:
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
    ca_file: "/path/to/ca.pem"
    insecure_skip_verify: false  # 生产环境必须设为 false
```

#### 认证配置
```yaml
lynx:
  auth:
    enabled: true
    type: "jwt"  # 支持 jwt, oauth2, basic
    jwt:
      secret_key: "your-secret-key"
      expires_in: "24h"
    oauth2:
      client_id: "your-client-id"
      client_secret: "your-client-secret"
      token_url: "https://auth.example.com/token"
```

#### 授权配置
```go
// 基于角色的访问控制示例
func setupAuth(app *lynx.App) {
    app.Use(middleware.Auth(middleware.AuthConfig{
        Skipper: func(ctx context.Context) bool {
            // 跳过不需要认证的路径
            return false
        },
        Validator: func(ctx context.Context, token string) (bool, error) {
            // 验证 token 并检查权限
            return true, nil
        },
    }))
}
```

### 6. 错误处理

#### 错误类型
```go
// 定义错误类型
const (
    ErrorCategoryNetwork    ErrorCategory = "network"
    ErrorCategoryDatabase   ErrorCategory = "database"
    ErrorCategoryConfig     ErrorCategory = "configuration"
    ErrorCategoryPlugin     ErrorCategory = "plugin"
    ErrorCategoryResource   ErrorCategory = "resource"
    ErrorCategorySecurity   ErrorCategory = "security"
    ErrorCategoryTimeout    ErrorCategory = "timeout"
    ErrorCategoryValidation ErrorCategory = "validation"
    ErrorCategorySystem     ErrorCategory = "system"
)
```

#### 错误恢复
```go
// 注册自定义恢复策略
func registerRecoveryStrategies(erm *app.ErrorRecoveryManager) {
    // 数据库错误恢复策略
    dbStrategy := &CustomRecoveryStrategy{
        name:    "database_recovery",
        timeout: 5 * time.Second,
    }
    erm.RegisterRecoveryStrategy("database", dbStrategy)
}

// 使用错误恢复管理器
func handleError(erm *app.ErrorRecoveryManager, err error) {
    erm.RecordError(
        "database_connection",
        app.ErrorCategoryDatabase,
        err.Error(),
        "user_service",
        app.ErrorSeverityHigh,
        map[string]any{
            "operation": "query",
            "table":    "users",
        },
    )
}
```

#### 熔断器配置
```yaml
lynx:
  circuit_breaker:
    enabled: true
    threshold: 5        # 打开熔断器前的错误数量
    timeout: 30s        # 熔断器保持打开的时间
    half_open_timeout: 5s  # 半开状态等待时间
```

---

## 📡 事件系统

### 事件类型
```go
// 内置事件类型
const (
    // 插件生命周期事件
    EventPluginLoaded   = "plugin.loaded"
    EventPluginStarted  = "plugin.started"
    EventPluginStopped  = "plugin.stopped"
    
    // 资源事件
    EventResourceCreated = "resource.created"
    EventResourceDeleted = "resource.deleted"
    
    // 错误事件
    EventErrorOccurred  = "error.occurred"
    EventErrorResolved  = "error.resolved"
    EventPanicRecovered = "panic.recovered"
)
```

### 事件过滤
```go
// 注册事件监听器并过滤特定事件
func setupEventListeners(runtime app.Runtime) {
    // 创建事件过滤器
    filter := app.EventFilter{
        PluginID: "http_service",
        EventTypes: []string{"error.occurred", "plugin.stopped"},
        Priority:   app.PriorityHigh,
    }
    
    // 注册带过滤器的监听器
    runtime.AddListener(filter, func(event app.PluginEvent) {
        log.Infof("接收到事件: %s, 来源: %s", event.Type, event.PluginID)
    })
}
```

---

## 📊 监控和可观测性

### Metrics
```yaml
lynx:
  metrics:
    enabled: true
    addr: ":9100"
    path: "/metrics"
    namespace: "lynx"
    subsystem: "http"
    labels:
      - "service"
      - "instance"
      - "version"
```

### Tracing
```yaml
lynx:
  tracer:
    enabled: true
    provider: "jaeger"  # 支持 jaeger, zipkin, otlp
    jaeger:
      endpoint: "http://jaeger:14268/api/traces"
      sampler_type: "const"
      sampler_param: 1.0
```

### Logging
```yaml
lynx:
  log:
    level: "info"  # debug, info, warn, error
    format: "json"  # text, json
    output: "stdout"  # stdout, file
    file:
      path: "/var/log/lynx.log"
      max_size: 100  # MB
      max_age: 30  # 天
      max_backups: 10
```

### Health Checks
```go
// 注册健康检查
func registerHealthChecks(app *lynx.App) {
    app.Health().Register("database", func() (bool, error) {
        // 检查数据库连接
        return db.Ping() == nil, nil
    })
    
    app.Health().Register("redis", func() (bool, error) {
        // 检查 Redis 连接
        return redis.Ping() == nil, nil
    })
}
```

---

## 🚀 生产就绪功能

### Graceful Shutdown
```yaml
lynx:
  graceful_shutdown:
    enabled: true
    timeout: 30s  # 优雅关闭超时时间
    signals:  # 触发关闭的信号
      - "SIGINT"
      - "SIGTERM"
```

### Rate Limiting
```yaml
lynx:
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst: 50
```

### Retry Policies
```yaml
lynx:
  retry:
    enabled: true
    max_attempts: 3
    initial_interval: "100ms"
    max_interval: "1s"
    multiplier: 2.0
    randomization_factor: 0.5
```

### Dead Letter Queues
```yaml
lynx:
  mq:
    dead_letter:
      enabled: true
      exchange: "lynx.dlx"
      routing_key: "lynx.dlq"
      ttl: "86400s"  # 24小时
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

## 🧭 CLI 命令

### 📋 lynx new - 创建新项目

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

### 🔍 lynx doctor - 诊断环境与项目健康状态

`lynx doctor` 命令对您的开发环境和 Lynx 项目执行全面的健康检查。

#### 检查内容

**环境检查：**
- ✅ Go 安装和版本（最低要求 Go 1.20+）
- ✅ Go 环境变量（GOPATH、GO111MODULE、GOPROXY）
- ✅ Git 仓库状态和未提交的更改

**工具检查：**
- ✅ Protocol Buffers 编译器（protoc）安装
- ✅ Wire 依赖注入工具可用性
- ✅ Lynx 项目所需的开发工具

**项目结构：**
- ✅ 验证预期的目录结构（app/、boot/、plugins/ 等）
- ✅ 检查 go.mod 文件的存在和有效性
- ✅ 验证 Makefile 和预期的目标

**配置：**
- ✅ 扫描和验证 YAML/YML 配置文件
- ✅ 检查配置语法和结构

#### 输出格式

- **Text**（默认）：人类可读，带颜色和图标
- **JSON**：机器可读，适用于 CI/CD 集成
- **Markdown**：文档友好格式

#### 命令选项

```bash
# 运行所有诊断检查
lynx doctor

# 以 JSON 格式输出（用于 CI/CD）
lynx doctor --format json

# 以 Markdown 格式输出
lynx doctor --format markdown > health-report.md

# 仅检查特定类别
lynx doctor --category env      # 仅环境
lynx doctor --category tools    # 仅工具
lynx doctor --category project  # 仅项目结构
lynx doctor --category config   # 仅配置

# 自动修复可能的问题
lynx doctor --fix

# 显示详细诊断信息
lynx doctor --verbose
```

#### 自动修复功能

`--fix` 标志可以自动解决：
- 缺失的开发工具（通过 `make init` 或 `go install` 安装）
- go.mod 问题（运行 `go mod tidy`）
- 其他可修复的配置问题

#### 健康状态指示器

- 💚 **健康**：所有检查通过
- 💛 **降级**：检测到一些警告但功能正常
- 🔴 **严重**：发现需要关注的错误

#### 输出示例

```
🔍 Lynx Doctor - 诊断报告
==================================================

📊 系统信息：
  • 操作系统/架构：darwin/arm64
  • Go 版本：go1.24.4
  • Lynx 版本：v2.0.0

🔎 诊断检查：
--------------------------------------------------
✅ Go 版本：已安装 Go 1.24
✅ 项目结构：找到所有预期目录
⚠️ Wire 依赖注入：未安装
   💡 可用修复（使用 --fix 应用）

📈 摘要：
  总检查数：9
  ✅ 通过：7
  ⚠️ 警告：2

💛 整体健康状态：降级
```

### 🚀 lynx run - 快速开发服务器

`lynx run` 命令提供了一种便捷的方式来构建和运行您的 Lynx 项目，并支持热重载以实现快速开发。

#### 功能特性

- **自动构建和运行**：一个命令即可编译并执行项目
- **热重载**：文件更改时自动重新构建和重启（使用 `--watch` 标志）
- **进程管理**：优雅的关闭和重启处理
- **智能检测**：自动在项目结构中查找主包
- **环境控制**：传递自定义环境变量和参数

#### 命令选项

```bash
lynx run [path] [flags]
```

**标志：**
- `--watch, -w`：启用热重载（监视文件更改）
- `--build-args`：go build 的附加参数
- `--run-args`：传递给运行应用程序的参数
- `--verbose, -v`：启用详细输出
- `--env, -e`：环境变量（KEY=VALUE）
- `--port, -p`：覆盖应用程序端口
- `--skip-build`：跳过构建并运行现有二进制文件

#### 使用示例

```bash
# 在当前目录运行项目
lynx run

# 启用热重载（文件更改时自动重启）
lynx run --watch

# 运行特定项目目录
lynx run ./my-service

# 传递自定义构建标志
lynx run --build-args="-ldflags=-s -w"

# 传递运行时配置
lynx run --run-args="--config=./configs"

# 设置环境变量
lynx run -e PORT=8080 -e ENV=development

# 运行现有二进制文件而不重新构建
lynx run --skip-build
```

#### 热重载详情

使用 `--watch` 模式时，以下文件会触发重新构建：
- Go 源文件（`.go`）
- Go 模块文件（`go.mod`、`go.sum`）
- 配置文件（`.yaml`、`.yml`、`.json`、`.toml`）
- 环境文件（`.env`）
- Protocol Buffer 文件（`.proto`）

忽略的路径：
- `.git`、`.idea`、`vendor`、`node_modules`
- 构建目录（`bin`、`dist`、`tmp`）
- 测试文件（`*_test.go`）

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

### 开发工作流
1. Fork 仓库
2. 创建功能分支
3. 提交更改
4. 运行测试
5. 提交 Pull Request

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
