# Redis 插件配置验证

本文档描述了 Redis 插件的配置验证功能，包括验证规则、使用方法和最佳实践。

## 概述

Redis 插件现在包含了完整的配置验证逻辑，确保在插件启动前配置的正确性和合理性。验证功能包括：

- **基础连接验证**：地址格式、网络类型等
- **连接池配置验证**：连接数量关系、超时时间等
- **超时配置验证**：各种超时时间的合理性和关系
- **重试配置验证**：重试次数和退避时间的合理性
- **TLS配置验证**：TLS启用与地址格式的匹配
- **Sentinel配置验证**：哨兵模式的必要参数
- **数据库配置验证**：数据库编号范围
- **客户端名称验证**：名称格式和长度

## 验证规则详解

### 1. 基础连接验证

#### 地址格式验证
- 支持 `redis://`、`rediss://` 前缀
- 验证主机:端口格式
- 端口范围：1-65535
- 不允许空地址

```yaml
# ✅ 有效地址
addrs: ["localhost:6379", "redis://127.0.0.1:6380", "rediss://secure.redis:6379"]

# ❌ 无效地址
addrs: ["invalid-address", ":6379", "localhost:", "localhost:99999"]
```

#### 网络类型验证
- 支持的网络类型：`tcp`、`tcp4`、`tcp6`、`unix`、`unixpacket`
- 默认为 `tcp`

### 2. 连接池配置验证

#### 连接数量关系
- `MinIdleConns` ≥ 0
- `MaxActiveConns` > 0
- `MinIdleConns` ≤ `MaxActiveConns`

```yaml
# ✅ 有效配置
min_idle_conns: 10
max_active_conns: 20

# ❌ 无效配置
min_idle_conns: 30
max_active_conns: 20  # 最小值不能大于最大值
```

#### 连接生命周期
- `ConnMaxIdleTime`: 0-24小时
- `MaxConnAge`: 0-7天
- `PoolTimeout`: 0-30秒

### 3. 超时配置验证

#### 超时时间范围
- `DialTimeout`: 0-60秒
- `ReadTimeout`: 0-5分钟
- `WriteTimeout`: 0-5分钟

#### 超时时间关系
- `DialTimeout` ≤ `ReadTimeout`（建议）

```yaml
# ✅ 有效配置
dial_timeout: { seconds: 5 }
read_timeout: { seconds: 10 }

# ❌ 无效配置
dial_timeout: { seconds: 10 }
read_timeout: { seconds: 5 }  # 建连超时不应大于读超时
```

### 4. 重试配置验证

#### 重试次数
- `MaxRetries`: 0-10

#### 退避时间
- `MinRetryBackoff`: 0-1秒
- `MaxRetryBackoff`: 0-30秒
- `MinRetryBackoff` ≤ `MaxRetryBackoff`

### 5. TLS配置验证

- 如果启用 TLS，建议使用 `rediss://` 前缀
- 支持 `tls.enabled` 和 `tls.insecure_skip_verify` 配置

### 6. Sentinel配置验证

- 启用 Sentinel 模式时必须提供 `master_name`
- Sentinel 地址格式验证

### 7. 数据库配置验证

- 数据库编号范围：0-15（Redis 默认限制）

### 8. 客户端名称验证

- 长度限制：≤ 64 字符
- 字符限制：只允许字母、数字、下划线、连字符

## 使用方法

### 1. 自动验证（推荐）

配置验证会在插件初始化时自动执行：

```go
// 在 InitializeResources 中自动调用
if err := ValidateAndSetDefaults(r.conf); err != nil {
    return fmt.Errorf("redis configuration validation failed: %w", err)
}
```

### 2. 手动验证

如果需要手动验证配置：

```go
import "github.com/go-lynx/lynx/plugins/nosql/redis"

// 验证配置
result := redis.ValidateRedisConfig(config)
if !result.IsValid {
    log.Errorf("Configuration validation failed: %s", result.Error())
    return
}

// 验证并设置默认值
if err := redis.ValidateAndSetDefaults(config); err != nil {
    log.Errorf("Configuration validation failed: %v", err)
    return
}
```

### 3. 获取验证错误详情

```go
result := redis.ValidateRedisConfig(config)
if !result.IsValid {
    for _, err := range result.Errors {
        log.Errorf("Field: %s, Error: %s", err.Field, err.Message)
    }
}
```

## 默认值设置

如果配置验证通过，系统会自动设置合理的默认值：

```go
// 网络类型
Network: "tcp"

// 连接池
MinIdleConns: 10
MaxIdleConns: 20
MaxActiveConns: 20

// 超时时间
DialTimeout: 10s
ReadTimeout: 10s
WriteTimeout: 10s
PoolTimeout: 3s

// 连接生命周期
ConnMaxIdleTime: 10s
MaxConnAge: 30m

// 重试配置
MaxRetries: 3
MinRetryBackoff: 8ms
MaxRetryBackoff: 512ms
```

## 错误处理

### 验证错误类型

```go
type ValidationError struct {
    Field   string  // 出错的字段名
    Message string  // 错误描述
}
```

### 验证结果

```go
type ValidationResult struct {
    IsValid bool              // 是否验证通过
    Errors  []ValidationError // 错误列表
}
```

## 最佳实践

### 1. 配置模板

```yaml
# 生产环境配置模板
redis:
  network: tcp
  addrs: ["redis-master:6379", "redis-slave:6379"]
  min_idle_conns: 20
  max_active_conns: 100
  dial_timeout: { seconds: 5 }
  read_timeout: { seconds: 10 }
  write_timeout: { seconds: 10 }
  pool_timeout: { seconds: 2 }
  max_retries: 3
  client_name: "myapp-prod"
```

### 2. 开发环境配置

```yaml
# 开发环境配置
redis:
  addrs: ["localhost:6379"]
  min_idle_conns: 5
  max_active_conns: 20
  dial_timeout: { seconds: 2 }
  read_timeout: { seconds: 5 }
  write_timeout: { seconds: 5 }
  client_name: "myapp-dev"
```

### 3. 哨兵模式配置

```yaml
redis:
  addrs: ["sentinel1:26379", "sentinel2:26379"]
  sentinel:
    master_name: "mymaster"
  min_idle_conns: 10
  max_active_conns: 50
  pool_timeout: { seconds: 1 }
```

### 4. TLS配置

```yaml
redis:
  addrs: ["rediss://secure.redis:6379"]
  tls:
    enabled: true
    insecure_skip_verify: false  # 生产环境应为 false
  min_idle_conns: 10
  max_active_conns: 20
```

## 故障排除

### 常见验证错误

1. **地址格式错误**
   ```
   validation error in field 'addrs[0]': invalid address format: address localhost: invalid port
   ```

2. **连接池配置错误**
   ```
   validation error in field 'min_idle_conns': cannot be greater than max_active_conns
   ```

3. **超时配置错误**
   ```
   validation error in field 'dial_timeout': should not be greater than read_timeout
   ```

4. **数据库编号错误**
   ```
   validation error in field 'db': database number cannot exceed 15 (Redis default limit)
   ```

### 调试建议

1. 使用 `ValidateRedisConfig()` 进行预验证
2. 检查配置文件的 YAML 语法
3. 验证地址格式和端口号
4. 确认连接池参数关系
5. 检查超时时间设置

## 测试

运行配置验证测试：

```bash
cd lynx/plugins/nosql/redis
go test -v -run TestValidateRedisConfig
```

测试覆盖了各种配置场景，包括：
- 有效配置验证
- 无效配置检测
- 边界条件测试
- 错误消息格式验证
