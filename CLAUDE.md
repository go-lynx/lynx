# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

Lynx是一个革命性的开源微服务框架，基于Seata、Polaris和Kratos构建，提供即插即用的微服务开发体验。

### 核心特性
- 零配置启动：最少配置快速开始
- 插件驱动：模块化架构，热插拔组件
- 企业级：生产级可靠性和安全性
- 云原生：为现代云环境设计

## 项目结构

```
├── app/                    # 核心应用层
│   ├── conf/              # 应用配置
│   ├── factory/           # 工厂接口和注册
│   ├── kratos/            # Kratos集成
│   ├── log/               # 日志系统
│   ├── observability/     # 可观测性
│   ├── subscribe/         # 订阅和TLS
│   ├── tls/               # TLS配置
│   └── util/              # 工具库
├── boot/                  # 启动管理
├── cmd/lynx/              # CLI工具
├── docs/                  # 文档
├── examples/              # 示例代码
├── grafana/               # 监控仪表板
├── plugins/               # 插件系统
│   ├── dtx/              # 分布式事务
│   ├── mq/               # 消息队列
│   ├── nosql/            # NoSQL数据库
│   ├── polaris/          # 服务发现
│   ├── service/          # 服务协议
│   ├── sql/              # SQL数据库
│   └── tracer/           # 链路追踪
└── third_party/          # 第三方协议
```

## 常用命令

### 开发环境初始化
```bash
make init                    # 安装开发工具和依赖
```

### 构建和生成
```bash
make config                  # 生成protobuf配置文件
```

### 测试
```bash
go test ./...                # 运行所有测试
go test -v ./plugins/...     # 测试特定模块
```

### 模块管理
```bash
# 为模块打标签
make tag MODULES_VERSION=v2.0.0 MODULES="plugins/sql/mysql plugins/sql/pgsql"

# 推送标签
make push-tags MODULES_VERSION=v2.0.0 MODULES="plugins/sql/mysql plugins/sql/pgsql"

# 发布（打标签并推送）
make release MODULES_VERSION=v2.0.0 MODULES="plugins/sql/mysql plugins/sql/pgsql"
```

### CLI工具使用
```bash
# 创建新项目
lynx new my-service

# 创建多个项目
lynx new service1 service2 service3

# 国际化支持
LYNX_LANG=zh lynx new demo    # 中文
LYNX_LANG=en lynx new demo    # 英文

# 日志级别控制
LYNX_LOG_LEVEL=debug lynx new demo
LYNX_QUIET=1 lynx new demo    # 静默模式
LYNX_VERBOSE=1 lynx new demo  # 详细模式
```

## 架构核心概念

### 分层Runtime架构
Lynx采用分层Runtime设计，包含四个主要层次：
1. **应用层**: LynxApp、Boot、Control Plane
2. **插件管理层**: Plugin Manager、TypedPlugin Manager、Plugin Factory
3. **运行时层**: Runtime Interface、TypedRuntime Impl、Simple Runtime
4. **资源管理层**: Private Resources、Shared Resources、Resource Info

### 插件系统
- **插件生命周期**: LoadPlugins → Initialize → Start → Stop → Cleanup
- **资源管理**: 私有资源和共享资源分离管理
- **事件系统**: 插件隔离的事件处理
- **上下文管理**: 每个插件独立的运行时上下文

### 核心组件
- **服务发现**: 基于Polaris的服务注册与发现
- **分布式事务**: 基于Seata的ACID事务
- **服务协议**: HTTP和gRPC服务支持
- **数据存储**: MySQL、PostgreSQL、Redis、MongoDB、Elasticsearch
- **消息队列**: Kafka、RabbitMQ、RocketMQ
- **链路追踪**: OpenTelemetry集成

## 配置管理

### 协议定义位置
- `app/conf/`: 应用主配置
- `plugins/*/conf/`: 各插件配置定义
- 所有配置都使用protobuf定义

### 示例配置结构
```yaml
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

## 开发指南

### 添加新插件
1. 在`plugins/`下创建插件目录
2. 实现插件接口：`Plugin`、生命周期方法
3. 添加配置protobuf定义
4. 实现`plug.go`文件进行插件注册
5. 添加README和示例配置

### 测试约定
- 单元测试：`*_test.go`
- 集成测试：`*_integration_test.go`
- 性能测试：`*_performance_test.go`

### 代码规范
- 使用Go 1.24.3
- 遵循Go官方代码规范
- 所有公开接口必须有文档注释
- 使用protobuf进行配置定义
- 错误处理使用kratos错误包装

### 工具链
- protoc: Protocol Buffer编译器
- wire: 依赖注入代码生成
- kratos: 微服务框架工具
- protoc-gen-*: 各种protobuf生成器

## 监控和可观测性

### Grafana仪表板
- `grafana/health/`: 健康检查仪表板
- `grafana/service/http/`: HTTP服务监控
- `grafana/sql/pgsql/`: PostgreSQL监控
- `grafana/nosql/redis/`: Redis监控
- `grafana/mq/kafka/`: Kafka监控

### 指标系统
- Prometheus集成
- 自定义指标支持
- 插件级别的指标收集

## 故障排除

### 常见问题
1. **protobuf生成失败**: 运行`make init`安装必要工具
2. **模块依赖问题**: 检查各插件的go.mod文件
3. **配置解析错误**: 验证protobuf定义和YAML格式

### 调试工具
- 使用kratos的日志系统
- 支持不同日志级别
- 内置性能监控和资源统计

## 多模块管理

该项目采用多模块架构，每个插件都是独立的Go模块：
- 主模块：`github.com/go-lynx/lynx`
- 插件模块：`github.com/go-lynx/lynx/plugins/<name>`
- CLI模块：`github.com/go-lynx/lynx/cmd/lynx`

使用`make tag`和`make release`命令管理模块版本。