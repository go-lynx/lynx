# Lynx Tracer Plugin

Lynx 链路追踪插件，基于 OpenTelemetry 实现分布式链路追踪功能。

## 功能特性

- ✅ **OpenTelemetry 标准**：完全兼容 OpenTelemetry 标准
- ✅ **多种导出器**：支持 OTLP gRPC 导出器
- ✅ **灵活采样**：支持可配置的采样率
- ✅ **优雅关闭**：支持优雅关闭和资源清理
- ✅ **配置验证**：完整的配置验证和错误处理
- ✅ **健康检查**：内置健康检查机制

## 快速开始

### 1. 基本配置

在你的应用配置文件中添加以下配置：

```yaml
lynx:
  tracer:
    enable: true
    addr: "localhost:4317"
    ratio: 0.1
```

### 2. 启动应用

```bash
go run main.go
```

### 3. 查看链路追踪

访问你的链路追踪系统（如 Jaeger、Zipkin 等）查看追踪数据。

## 配置说明

### 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enable` | bool | `false` | 是否启用链路追踪 |
| `addr` | string | `"localhost:4317"` | 导出器地址 |
| `ratio` | float | `1.0` | 采样率 (0.0-1.0) |

### 详细配置

```yaml
lynx:
  tracer:
    # 是否启用链路追踪功能
    enable: true
    
    # 跟踪数据导出的目标端点地址
    # 通常是 OpenTelemetry Collector 等跟踪数据收集器的地址
    addr: "localhost:4317"
    
    # 跟踪采样率，取值范围为 0.0 到 1.0
    # 0.0 表示不采样，1.0 表示对所有请求进行采样
    ratio: 0.1
```

## 环境配置

### 开发环境

```yaml
lynx:
  tracer:
    enable: true
    addr: "localhost:4317"
    ratio: 1.0  # 开发环境全量采样
```

### 测试环境

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector-test:4317"
    ratio: 0.5  # 测试环境 50% 采样
```

### 生产环境

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector-prod:4317"
    ratio: 0.1  # 生产环境 10% 采样
```

## 使用示例

### 在代码中使用

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/app"
    "go.opentelemetry.io/otel"
)

func main() {
    // 启动 Lynx 应用
    app := app.New()
    
    // 获取 tracer
    tracer := otel.Tracer("my-service")
    
    // 创建 span
    ctx, span := tracer.Start(context.Background(), "my-operation")
    defer span.End()
    
    // 你的业务逻辑
    // ...
}
```

### 在 HTTP 服务中使用

```go
package main

import (
    "net/http"
    "github.com/go-lynx/lynx/app"
    "go.opentelemetry.io/otel"
)

func main() {
    app := app.New()
    
    http.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        tracer := otel.Tracer("http-server")
        ctx, span := tracer.Start(r.Context(), "handle-request")
        defer span.End()
        
        // 处理请求
        w.Write([]byte("Hello, World!"))
    })
    
    http.ListenAndServe(":8080", nil)
}
```

## 支持的导出器

### OTLP gRPC 导出器

默认使用 OTLP gRPC 导出器，支持以下特性：

- **压缩**：使用 gzip 压缩
- **超时**：30 秒连接超时
- **TLS**：支持 TLS 加密（当前配置为不加密）

### 支持的收集器

- **OpenTelemetry Collector**
- **Jaeger**
- **Zipkin**
- **Prometheus**（通过 Collector）

## 采样策略

### 采样率说明

| 采样率 | 说明 | 适用场景 |
|--------|------|----------|
| 0.0 | 不采样 | 性能测试 |
| 0.1 | 10% 采样 | 生产环境 |
| 0.5 | 50% 采样 | 测试环境 |
| 1.0 | 全量采样 | 开发环境 |

### 采样建议

- **开发环境**：使用 1.0 全量采样，便于调试
- **测试环境**：使用 0.5 采样，平衡性能和可观测性
- **生产环境**：使用 0.1-0.3 采样，避免性能影响

## 监控和调试

### 健康检查

插件内置健康检查机制，可以通过以下方式检查状态：

```go
// 检查 tracer 是否正常
if err := tracer.HealthCheck(); err != nil {
    log.Errorf("Tracer health check failed: %v", err)
}
```

### 日志输出

插件会输出详细的日志信息：

```
[INFO] Initializing tracing component with ratio: 0.100000, addr: localhost:4317
[INFO] Tracing component successfully initialized
[INFO] Tracer provider shutdown successfully
```

## 故障排除

### 常见问题

#### 1. 连接失败

**问题**：无法连接到收集器
```
failed to create OTLP exporter: context deadline exceeded
```

**解决方案**：
- 检查收集器地址是否正确
- 确认网络连接是否正常
- 检查防火墙设置

#### 2. 采样率无效

**问题**：采样率配置无效
```
sampling ratio must be between 0 and 1, got 1.5
```

**解决方案**：
- 确保采样率在 0.0-1.0 范围内
- 检查配置文件格式

#### 3. 地址配置错误

**问题**：启用追踪但未配置地址
```
tracer address is required when tracing is enabled
```

**解决方案**：
- 设置正确的 `addr` 配置项
- 或者禁用追踪功能

### 调试模式

启用详细日志输出：

```go
// 设置日志级别
log.SetLevel(log.DebugLevel)
```

## 性能考虑

### 性能影响

- **采样率**：采样率越高，性能影响越大
- **网络延迟**：导出器网络延迟会影响应用性能
- **内存使用**：追踪数据会占用一定内存

### 优化建议

1. **合理设置采样率**：生产环境建议使用 0.1-0.3
2. **使用本地收集器**：减少网络延迟
3. **配置超时**：避免长时间阻塞
4. **监控资源使用**：定期检查内存和 CPU 使用情况

## 版本历史

### v2.0.0

- ✅ 添加优雅关闭机制
- ✅ 完善配置验证
- ✅ 改进错误处理
- ✅ 添加健康检查
- ✅ 增加测试覆盖

### v1.0.0

- ✅ 基础链路追踪功能
- ✅ OTLP gRPC 导出器
- ✅ 可配置采样率

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License 