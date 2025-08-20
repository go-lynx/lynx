# Factory 包改进说明

## 概述

`factory` 包负责 Lynx 框架中插件的创建和管理。经过重构，我们改进了代码的命名、接口设计和职责分离，使其更加清晰和易于使用。

## 文件结构

### 1. `interfaces.go` - 接口定义
- **功能**: 定义插件管理的核心接口
- **主要组件**:
  - `Registry` 接口：插件注册管理
  - `Creator` 接口：插件创建功能
  - `Factory` 接口：完整的插件管理功能

### 2. `registry.go` - 插件注册表
- **功能**: 实现插件注册和配置映射管理
- **主要组件**:
  - `PluginRegistry` 结构体：插件注册表实现
  - `GlobalPluginRegistry()` 函数：获取全局注册表实例
  - 插件注册、注销、查询功能

### 3. `typed_factory.go` - 类型安全工厂
- **功能**: 提供类型安全的插件创建和管理
- **主要组件**:
  - `TypedFactory` 结构体：类型安全的插件工厂
  - `RegisterTypedPlugin()` 函数：注册类型安全的插件
  - `GetTypedPlugin()` 函数：获取类型安全的插件实例
  - `GlobalTypedFactory()` 函数：获取全局类型安全工厂

## 主要改进

### 1. 命名优化
- **原命名**: `lynx_factory.go`, `plugin_factory.go`
- **新命名**: `registry.go`, `interfaces.go`, `typed_factory.go`
- **改进**: 使命名更加直观，职责更清晰

### 2. 接口设计优化
- **原问题**: `PluginFactory` 接口职责过重
- **新方案**: 
  - `Registry` 接口：专注于注册管理
  - `Creator` 接口：专注于创建功能
  - `Factory` 接口：组合上述两个接口

### 3. 类型命名优化
- **原命名**: `LynxPluginFactory`, `TypedPluginFactory`
- **新命名**: `PluginRegistry`, `TypedFactory`
- **改进**: 避免与 Lynx 框架本身混淆，使命名更简洁

### 4. 职责分离
- **注册表**: 专注于插件的注册、注销和查询
- **工厂**: 专注于插件的创建和类型安全
- **接口**: 定义清晰的契约

### 5. 并发安全
- `TypedFactory` 使用读写锁保护并发访问
- 提供线程安全的插件管理

## 使用示例

### 基本注册表使用
```go
// 获取全局注册表
registry := factory.GlobalPluginRegistry()

// 注册插件
registry.RegisterPlugin("http_server", "http", func() plugins.Plugin {
    return &httpServerPlugin{}
})

// 创建插件
plugin, err := registry.CreatePlugin("http_server")
```

### 类型安全工厂使用
```go
// 获取全局类型安全工厂
typedFactory := factory.GlobalTypedFactory()

// 注册类型安全的插件
factory.RegisterTypedPlugin(typedFactory, "redis", "cache", func() *redis.Plugin {
    return redis.New()
})

// 获取类型安全的插件实例
redisPlugin, err := factory.GetTypedPlugin[*redis.Plugin](typedFactory, "redis")
```

## 向后兼容性

为了保持向后兼容性，我们保留了以下功能：
- `TypedFactory` 实现了 `Factory` 接口
- 提供了兼容旧接口的方法

## 接口层次结构

```
Factory (完整功能)
├── Registry (注册管理)
│   ├── RegisterPlugin()
│   ├── UnregisterPlugin()
│   ├── GetPluginRegistry()
│   └── HasPlugin()
└── Creator (创建功能)
    └── CreatePlugin()
```

## 设计原则

1. **单一职责**: 每个接口和结构体都有明确的职责
2. **接口隔离**: 将大接口拆分为更小的专用接口
3. **类型安全**: 提供泛型支持，确保类型安全
4. **并发安全**: 使用适当的锁机制保护共享状态
5. **向后兼容**: 保持现有代码的兼容性
