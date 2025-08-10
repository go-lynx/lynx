# Lynx Tracer Plugin

Lynx 链路追踪插件，基于 OpenTelemetry 实现分布式链路追踪功能。

## 功能特性

- ✅ OpenTelemetry 标准兼容
- ✅ 导出协议：OTLP gRPC、OTLP HTTP
- ✅ 传输能力：TLS（含双向）、超时、重试、压缩（gzip）、自定义 Header
- ✅ 批处理：可配置队列、批大小、导出超时与调度延迟
- ✅ 传播器：W3C tracecontext、baggage、B3（单/多头）、Jaeger
- ✅ 采样器：AlwaysOn/AlwaysOff/TraceIDRatio/ParentBased-TraceIDRatio
- ✅ 资源与限额：service.name/attributes 与 SpanLimits（属性/事件/链接/长度）
- ✅ 优雅关闭与资源清理

## 快速开始

### 1. 最小配置（gRPC，推荐）

在你的应用配置文件中添加以下配置：

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      protocol: PROTOCOL_OTLP_GRPC
      insecure: true
      batch:
        enabled: true
      propagators: [W3C_TRACE_CONTEXT, W3C_BAGGAGE]
```

### 2. 启动应用

```bash
go run main.go
```

### 3. 查看链路追踪

访问你的链路追踪系统（如 Jaeger、Zipkin 等）查看追踪数据。

## 配置说明

### 模块化（modular）配置项

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4317"
    config:
      protocol: PROTOCOL_OTLP_GRPC | PROTOCOL_OTLP_HTTP
      http_path: /v1/traces           # 仅 HTTP 时使用
      insecure: true                  # 或使用 TLS
      tls:
        ca_file: /path/ca.pem
        cert_file: /path/client.crt
        key_file: /path/client.key
        insecure_skip_verify: false
      headers:
        Authorization: Bearer ${OTEL_TOKEN}
      timeout: 10s
      retry:
        enabled: true
        initial_interval: 500ms
        max_interval: 5s
      batch:
        enabled: true
        max_queue_size: 2048
        scheduled_delay: 200ms
        export_timeout: 30s
        max_batch_size: 512
      sampler:
        type: SAMPLER_TRACEID_RATIO   # 也支持 ALWAYS_ON/ALWAYS_OFF/PARENT_BASED_TRACEID_RATIO
        ratio: 0.1
      propagators: [W3C_TRACE_CONTEXT, W3C_BAGGAGE, B3, B3_MULTI, JAEGER]
      resource:
        service_name: my-service
        attributes:
          env: prod
          team: core
      limits:
        attribute_count_limit: 128
        attribute_value_length_limit: 2048
        event_count_limit: 128
        link_count_limit: 128
```

### HTTP 导出示例（OTLP/HTTP）

```yaml
lynx:
  tracer:
    enable: true
    addr: "otel-collector:4318"
    config:
      protocol: PROTOCOL_OTLP_HTTP
      http_path: /v1/traces
      insecure: true
      compression: COMPRESSION_GZIP
      batch:
        enabled: true
      propagators: [B3, W3C_BAGGAGE]
```

## 环境配置

建议在不同环境下基于“模块化配置”调整 exporter 地址与采样策略（config.sampler），不再使用旧版 ratio 字段。

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

### OTLP gRPC/HTTP 导出器

支持以下特性：

- 压缩：gzip
- 超时：可配置 timeout
- TLS：单向或双向
- 重试：初始/最大重试间隔
- 批处理：队列/批大小/导出超时/调度延迟

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

### 日志输出

插件会输出详细的日志信息：

```
[INFO] Initializing link monitoring component
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
- 设置正确的 `addr` 配置项（gRPC: 4317 / HTTP: 4318 + http_path）
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

- ✅ Modular 配置（协议/TLS/重试/压缩/批处理/传播器/资源/限额）
- ✅ 导出器支持 OTLP HTTP
- ✅ 采样器与传播器可配置
- ✅ 优雅关闭机制

### v1.0.0

- ✅ 基础链路追踪功能
- ✅ OTLP gRPC 导出器
- ✅ 可配置采样率

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License