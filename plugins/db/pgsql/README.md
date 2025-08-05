# PgSQL 数据库插件

这是 Lynx 框架的 PgSQL 数据库连接插件，提供了完整的数据库连接管理功能。

## 功能特性

### ✅ 已实现的功能

1. **配置验证**: 自动验证配置参数的有效性
2. **错误处理**: 优雅的错误处理，避免 panic
3. **重试机制**: 连接失败时自动重试
4. **连接池监控**: 实时监控连接池状态
5. **健康检查**: 全面的数据库健康检查
6. **优雅关闭**: 安全的连接关闭机制
7. **配置更新**: 支持运行时配置更新
8. **详细日志**: 提供详细的调试和监控日志
9. **统计信息**: 提供连接池统计信息
10. **状态查询**: 提供连接状态查询接口

## 配置说明

### 基本配置

```yaml
lynx:
  pgsql:
    driver: "postgres"
    source: "postgres://username:password@host:port/database?sslmode=disable"
    min_conn: 10
    max_conn: 50
    max_idle_time: "30s"
    max_life_time: "300s"
```

### 配置参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `driver` | string | "postgres" | 数据库驱动名称 |
| `source` | string | "postgres://admin:123456@127.0.0.1:5432/demo?sslmode=disable" | 数据库连接字符串 |
| `min_conn` | int | 10 | 最小连接数（空闲连接数） |
| `max_conn` | int | 20 | 最大连接数 |
| `max_idle_time` | duration | "10s" | 连接最大空闲时间 |
| `max_life_time` | duration | "300s" | 连接最大生命周期 |

### 连接字符串格式

```
postgres://username:password@host:port/database?param1=value1&param2=value2
```

常用参数：
- `sslmode`: SSL 模式 (disable, require, verify-ca, verify-full)
- `connect_timeout`: 连接超时时间
- `statement_timeout`: 语句超时时间
- `application_name`: 应用名称

## 使用方法

### 1. 获取数据库驱动

```go
import (
    "github.com/go-lynx/lynx/plugins/db/pgsql"
    "entgo.io/ent/dialect/sql"
)

// 获取数据库驱动
driver := pgsql.GetDriver()
if driver == nil {
    // 处理错误
    return
}

// 使用 ent 创建客户端
client := ent.NewClient(ent.Driver(driver))
```

### 2. 健康检查

```go
// 执行健康检查
err := pgsql.CheckHealth()
if err != nil {
    log.Errorf("Database health check failed: %v", err)
}
```

### 3. 获取连接池统计信息

```go
// 获取连接池统计信息
stats := pgsql.GetStats()
if stats != nil {
    log.Infof("Connection pool stats: open=%d, in_use=%d, idle=%d", 
        stats.OpenConnections, stats.InUse, stats.Idle)
}
```

### 4. 检查连接状态

```go
// 检查是否已连接
if pgsql.IsConnected() {
    log.Info("Database is connected")
} else {
    log.Warn("Database is not connected")
}
```

### 5. 获取配置信息

```go
// 获取当前配置
config := pgsql.GetConfig()
if config != nil {
    log.Infof("Current config: driver=%s, max_conn=%d", 
        config.Driver, config.MaxConn)
}
```

### 6. Prometheus 监控

```go
// 获取 Prometheus 指标处理器
handler := pgsql.GetPrometheusHandler()

// 在 HTTP 服务器中注册指标端点
http.Handle("/metrics", handler)
```

## Prometheus 监控配置

### 启用监控

在配置文件中启用 Prometheus 监控：

```yaml
lynx:
  pgsql:
    driver: "postgres"
    source: "postgres://user:pass@localhost:5432/dbname"
    min_conn: 10
    max_conn: 50
    prometheus:
      enabled: true
      metrics_path: "/metrics"
      metrics_port: 9090
      namespace: "lynx"
      subsystem: "pgsql"
      labels:
        environment: "production"
        service: "myapp"
```

### 监控指标

插件提供以下 Prometheus 指标：

#### 连接池指标
- `lynx_pgsql_max_open_connections`: 最大连接数
- `lynx_pgsql_open_connections`: 当前连接数
- `lynx_pgsql_in_use_connections`: 使用中的连接数
- `lynx_pgsql_idle_connections`: 空闲连接数
- `lynx_pgsql_max_idle_connections`: 最大空闲连接数

#### 等待指标
- `lynx_pgsql_wait_count_total`: 等待连接的总次数
- `lynx_pgsql_wait_duration_seconds_total`: 等待连接的总时间

#### 连接关闭指标
- `lynx_pgsql_max_idle_closed_total`: 因空闲超时关闭的连接数
- `lynx_pgsql_max_lifetime_closed_total`: 因生命周期关闭的连接数

#### 健康检查指标
- `lynx_pgsql_health_check_total`: 健康检查总次数
- `lynx_pgsql_health_check_success_total`: 成功健康检查次数
- `lynx_pgsql_health_check_failure_total`: 失败健康检查次数

#### 配置指标
- `lynx_pgsql_config_min_connections`: 配置的最小连接数
- `lynx_pgsql_config_max_connections`: 配置的最大连接数

### 访问监控指标

启动应用后，可以通过以下方式访问监控指标：

```bash
# 访问指标端点
curl http://localhost:9090/metrics

# 查看特定指标
curl http://localhost:9090/metrics | grep lynx_pgsql
```

### Prometheus 配置

在 Prometheus 配置文件中添加抓取目标：

```yaml
scrape_configs:
  - job_name: 'lynx-pgsql'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Grafana 仪表板

可以创建 Grafana 仪表板来可视化监控指标：

```json
{
  "dashboard": {
    "title": "Lynx PgSQL 监控",
    "panels": [
      {
        "title": "连接池状态",
        "type": "stat",
        "targets": [
          {
            "expr": "lynx_pgsql_open_connections",
            "legendFormat": "当前连接数"
          }
        ]
      },
      {
        "title": "连接池利用率",
        "type": "gauge",
        "targets": [
          {
            "expr": "lynx_pgsql_in_use_connections / lynx_pgsql_max_open_connections * 100",
            "legendFormat": "利用率 %"
          }
        ]
      }
    ]
  }
}
```

## 监控和调试

### 连接池统计信息

插件提供以下统计信息：

- `MaxOpenConnections`: 最大打开连接数
- `OpenConnections`: 当前打开连接数
- `InUse`: 正在使用的连接数
- `Idle`: 空闲连接数
- `MaxIdleConnections`: 最大空闲连接数
- `WaitCount`: 等待连接的次数
- `WaitDuration`: 等待连接的总时间
- `MaxIdleClosed`: 因超过最大空闲时间而关闭的连接数
- `MaxLifetimeClosed`: 因超过最大生命周期而关闭的连接数

### 日志信息

插件会输出详细的日志信息：

- 配置加载和验证
- 连接建立过程
- 重试尝试
- 健康检查结果
- 连接池状态
- 错误和警告信息

## 错误处理

插件实现了完善的错误处理机制：

1. **配置验证错误**: 在初始化阶段验证配置有效性
2. **连接错误**: 连接失败时自动重试
3. **健康检查错误**: 提供详细的健康检查错误信息
4. **关闭错误**: 优雅处理连接关闭错误

## 最佳实践

### 1. 连接池配置

```yaml
# 开发环境
min_conn: 5
max_conn: 20

# 生产环境
min_conn: 20
max_conn: 200
```

### 2. 超时配置

```yaml
# 合理的超时配置
max_idle_time: "300s"    # 5分钟
max_life_time: "3600s"   # 1小时
```

### 3. SSL 配置

```yaml
# 开发环境
source: "postgres://user:pass@localhost:5432/db?sslmode=disable"

# 生产环境
source: "postgres://user:pass@db.example.com:5432/db?sslmode=require"
```

### 4. 监控集成

```go
// 定期检查连接池状态
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            stats := pgsql.GetStats()
            if stats != nil {
                // 发送监控指标
                metrics.RecordConnectionPoolStats(stats)
            }
        }
    }
}()
```

## 故障排除

### 常见问题

1. **连接失败**
   - 检查连接字符串格式
   - 验证数据库服务是否运行
   - 检查网络连接

2. **连接池耗尽**
   - 增加 `max_conn` 配置
   - 检查是否有连接泄漏
   - 优化查询性能

3. **健康检查失败**
   - 检查数据库服务状态
   - 验证网络连接
   - 查看详细错误日志

### 调试技巧

1. 启用详细日志
2. 监控连接池统计信息
3. 定期执行健康检查
4. 使用连接池监控工具

## 版本历史

- **v2.0.0**: 重构版本，添加了完整的错误处理、重试机制、监控功能等
- **v1.x.x**: 基础版本，提供基本的数据库连接功能 