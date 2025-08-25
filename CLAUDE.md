# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

Go-Lynx是一个插件化的Go微服务框架，基于Seata、Polaris和Kratos构建。框架采用分层Runtime架构设计，提供统一的资源管理、事件系统和插件生命周期管理。

## 架构结构

### 核心模块
- `app/` - 框架核心组件
  - `events/` - 统一事件系统，支持插件间通信和事件处理
  - `cache/` - 缓存管理系统，基于Ristretto实现
  - `tls/` - TLS证书管理和文件监控
  - `utils/` - 工具包，包含各种实用函数
  - `plugin_manager.go` - 插件管理器，负责插件生命周期
  - `runtime.go` - Runtime接口和实现

### 插件系统
- `plugins/` - 插件实现目录
  - `service/` - 服务类插件（HTTP、gRPC、OpenIM）
  - `mq/` - 消息队列插件（Kafka、RabbitMQ、RocketMQ、Pulsar）
  - `nosql/` - NoSQL插件（Redis、MongoDB、Elasticsearch）
  - `sql/` - SQL数据库插件（MySQL、PostgreSQL、MSSQL）
  - `polaris/` - 服务发现和治理
  - `tracer/` - 分布式链路追踪
  - `swagger/` - API文档生成
  - `dtx/` - 分布式事务（DTM、Seata）

### 启动和配置
- `boot/` - 应用启动和配置管理
- `cmd/lynx/` - CLI工具，用于项目创建和管理

## 开发命令

### 常用Make命令
```bash
# 初始化开发环境
make init

# 生成protobuf配置文件 
make config

# 查看所有可用命令
make help

# 发布模块版本
make release MODULES_VERSION=v2.0.0 MODULES="plugins/xxx plugins/yyy"
```

### 测试命令
```bash
# 运行所有测试（注意：某些测试可能因依赖问题失败）
go test -v ./...

# 运行特定包的测试
go test -v ./app/cache/
go test -v ./app/events/
go test -v ./plugins/polaris/

# 运行基准测试
go test -bench=. -benchmem ./app/events/
```

### CLI工具
```bash
# 安装CLI工具
go install github.com/go-lynx/lynx/cmd/lynx@latest

# 创建新项目
lynx new my-service

# 创建多个项目
lynx new service1 service2 service3

# 使用特定配置
lynx new demo --module github.com/acme/demo --post-tidy
```

## 分层Runtime架构

项目采用四层架构设计：

1. **应用层** (Application Layer) - LynxApp、Boot、Control Plane
2. **插件管理层** (Plugin Management Layer) - PluginManager、TypedPluginManager、PluginFactory
3. **Runtime层** (Runtime Layer) - Runtime接口、TypedRuntimeImpl、SimpleRuntime
4. **资源管理层** (Resource Management Layer) - Private/Shared Resources、Resource Info

### 资源管理
- **私有资源** - 每个插件独立管理的资源
- **共享资源** - 所有插件共享的资源  
- **类型安全** - 支持泛型的类型安全资源访问
- **生命周期管理** - 完整的资源跟踪和清理

### 事件系统
- **插件隔离** - 插件命名空间事件避免冲突
- **事件过滤** - 支持多种过滤条件
- **历史记录** - 完整的事件历史查询
- **并发安全** - 线程安全的事件处理

## 开发约定

### 插件开发
- 每个插件必须实现`plugins.Plugin`接口
- 插件配置使用protobuf定义，放在`conf/`目录
- 插件需要支持热插拔和生命周期管理
- 使用`plug.go`文件作为插件入口点

### 配置管理
- 使用protobuf定义配置结构
- 配置文件支持YAML格式
- 每个插件的示例配置放在`conf/example_config.yml`

### 测试规范
- 单元测试文件以`_test.go`结尾
- 集成测试使用`integration_test.go`命名
- 性能测试使用`benchmark_test.go`或`performance_test.go`
- 压力测试使用`stress_test.go`

### 代码质量
- 使用Qodana进行代码质量检查
- 遵循Go标准代码规范
- 支持并发安全的设计模式
- 错误处理要完整和一致

## 第三方依赖管理

项目使用Go modules管理依赖，每个插件可以有独立的go.mod文件：
- 核心框架依赖：Kratos、Zerolog、Prometheus等
- 插件特定依赖：各插件根据需要引入专用依赖
- 测试依赖：Testify用于单元测试

## 文档和示例

- 架构文档：`docs/layered_runtime_architecture.md` 
- 示例代码：`examples/`目录
- 插件文档：每个插件目录下的`README.md`
- Grafana仪表盘：`grafana/`目录包含监控配置

## 注意事项

- 运行测试时某些包可能因为依赖问题失败，这是正常现象
- 插件系统支持热更新，开发时注意资源隔离
- 使用事件系统时注意避免循环依赖
- 分布式事务插件需要额外的外部服务支持