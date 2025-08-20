# Boot 包改进说明

## 概述

`boot` 包负责 Lynx 应用程序的引导和启动过程。经过重构，我们改进了代码的命名、结构和错误处理，使其更加清晰和健壮。

## 文件结构

### 1. `application.go` - 应用程序启动管理
- **功能**: 管理应用程序的完整生命周期
- **主要组件**:
  - `Application` 结构体：应用程序的主要引导结构
  - `Run()` 方法：启动应用程序并管理生命周期
  - `handlePanic()` 方法：处理 panic 并确保资源清理
  - `NewApplication()` 函数：创建新的应用程序实例

### 2. `configuration.go` - 配置加载和验证
- **功能**: 负责配置文件的加载、验证和管理
- **主要组件**:
  - `LoadBootstrapConfig()` 方法：加载引导配置
  - `validateConfig()` 方法：验证配置完整性
  - `setupConfigCleanup()` 方法：设置配置清理
  - `GetName()`, `GetHost()`, `GetVersion()` 方法：获取应用信息

### 3. `config_manager.go` - 配置路径管理
- **功能**: 管理配置路径，避免使用全局变量
- **主要组件**:
  - `ConfigManager` 结构体：配置管理器（单例模式）
  - `GetConfigManager()` 函数：获取配置管理器实例
  - `SetConfigPath()`, `GetConfigPath()` 方法：管理配置路径
  - `GetDefaultConfigPath()` 方法：获取默认配置路径

## 主要改进

### 1. 命名优化
- **原命名**: `strap.go`, `conf.go`, `config.go`
- **新命名**: `application.go`, `configuration.go`, `config_manager.go`
- **改进**: 使命名更加直观，领域更清晰

### 2. 配置路径处理优化
- **原问题**: 硬编码默认路径 `"../../configs"`
- **新方案**: 
  - 支持环境变量 `LYNX_CONFIG_PATH`
  - 默认使用当前目录下的 `./configs`
  - 通过配置管理器统一管理

### 3. 错误处理改进
- **原问题**: 清理设置失败时只记录警告
- **新方案**: 清理设置失败时返回错误，确保资源管理正确

### 4. 资源清理顺序优化
- **原问题**: 清理函数在 panic 处理之前执行
- **新方案**: 先处理 panic，再执行清理，避免访问未初始化资源

### 5. 错误信息增强
- **原问题**: 错误信息过于简单
- **新方案**: 提供更详细的错误上下文信息

### 6. 配置验证
- **新增功能**: 添加配置验证，确保必要配置项存在
- **验证项**: `lynx.name`, `lynx.version`, `lynx.host`

### 7. 模块化改进
- **原问题**: 使用全局变量 `flagConf`
- **新方案**: 使用配置管理器，提高可测试性和模块化程度

## 向后兼容性

为了保持向后兼容性，我们保留了以下别名：
- `type Boot = Application`
- `func NewLynxApplication() = NewApplication()`

## 使用示例

```go
// 创建应用程序实例
app := boot.NewApplication(wireFunc, plugins...)

// 启动应用程序
if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

## 环境变量

- `LYNX_CONFIG_PATH`: 设置默认配置路径
- 命令行参数 `-conf`: 指定配置文件路径

## 测试支持

代码包含测试环境检测，避免在测试时触发 flag 解析冲突。
