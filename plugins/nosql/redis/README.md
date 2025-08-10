# Redis 插件（UniversalClient）

本插件基于 go-redis v9 的 UniversalClient，统一支持单机、Cluster、Sentinel 拓扑，并内置 Prometheus 指标、启动健康检查、命令级别埋点、连接池统计与 TLS 支持。

注意：配置通过 protobuf（`conf/redis.proto`）下发，已移除废弃字段 `addr`，仅保留 `addrs`。所有注释均为中文。

## 功能概览
- 支持三种拓扑：single / cluster / sentinel（自动检测，或依据 `sentinel.master_name` 判定）
- TLS 支持：`tls.enabled` 与 `tls.insecure_skip_verify`，同时支持 `rediss://` 地址前缀自动启用 TLS
- 命令级别 Prometheus 指标：延迟直方图、错误计数
- 连接池指标：命中、未命中、等待超时、空闲/活动/陈旧连接数
- 启动与健康检查：启动 Ping、延迟日志；增强就绪检查（cluster 状态、role/connected_slaves）
- 兼容 API：`GetRedis()`（返回 redis.UniversalClient）与 `GetUniversalRedis()`

## 配置说明（protobuf）
见 `plugins/nosql/redis/conf/redis.proto`，核心字段如下（保持原编号，按领域分组）：
- 基础连接
  - `network` (1): 一般为 `tcp`
  - `addrs` (12): 地址列表，支持单机/集群/哨兵
  - `username` (13), `password` (3), `db` (4), `client_name` (20)
- 连接池/生命周期
  - `min_idle_conns` (5)
  - `max_idle_conns` (6) 注：go-redis 未使用该值，仅预留
  - `max_active_conns` (7) 映射为 go-redis `PoolSize`
  - `conn_max_idle_time` (8)
  - `idle_timeout` (15) 注：go-redis v9 已不建议，当前未映射
  - `max_conn_age` (16) 注：UniversalOptions 无该字段，当前未映射
  - `pool_timeout` (14)
- 超时
  - `dial_timeout` (9), `read_timeout` (10), `write_timeout` (11)
- 重试
  - `max_retries` (17), `min_retry_backoff` (18), `max_retry_backoff` (19)
- TLS
  - `tls.enabled`, `tls.insecure_skip_verify`
- Sentinel
  - `sentinel.master_name`, `sentinel.addrs`

已知限制：
- `max_idle_conns` 当前未被 go-redis 使用
- `idle_timeout`、`max_conn_age` 暂无对应 UniversalOptions 字段，未生效（后续如切换到 Options 构造或扩展将补齐）

## 使用示例

假设在运行时配置中（env/文件/配置中心）以 protobuf 对应结构下发 `redis` 段。

- 单机
```yaml
redis:
  network: tcp
  addrs: ["127.0.0.1:6379"]
  db: 0
  min_idle_conns: 10
  max_active_conns: 20
  dial_timeout: { seconds: 5 }
  read_timeout: { seconds: 5 }
  write_timeout: { seconds: 5 }
```

- Cluster
```yaml
redis:
  addrs: ["10.0.0.1:6379","10.0.0.2:6379","10.0.0.3:6379"]
  min_idle_conns: 20
  max_active_conns: 100
  pool_timeout: { seconds: 2 }
```

- Sentinel（推荐单独配置 sentinel.addrs；未提供时将复用 addrs）
```yaml
redis:
  addrs: ["10.0.0.10:26379","10.0.0.11:26379","10.0.0.12:26379"]
  sentinel:
    master_name: mymaster
    # addrs: ["10.0.0.10:26379","10.0.0.11:26379","10.0.0.12:26379"]
```

- TLS（两种方式其一即可）
```yaml
redis:
  addrs: ["rediss://10.0.0.1:6379"]
  tls:
    enabled: true
    insecure_skip_verify: true  # 仅测试环境
```

## 代码中使用
- 推荐使用包级方法获取客户端（无需持有 *PlugRedis 实例）：
```go
import (
    "context"
    "fmt"
    rplug "github.com/go-lynx/lynx/plugins/nosql/redis"
)

func useRedis() error {
    cli := rplug.GetUniversalRedis() // redis.UniversalClient：单机/集群/哨兵通用
    if cli == nil {
        return fmt.Errorf("redis plugin not initialized")
    }
    ctx := context.Background()
    return cli.Set(ctx, "k", "v", 0).Err()
}

// 若仅在单机模式下需要底层 *redis.Client：
func useSingleClient() error {
    c := rplug.GetRedis() // *redis.Client（Cluster/Sentinel 下为 nil）
    if c == nil {
        return nil // 或根据需要返回错误
    }
    return c.Ping(context.Background()).Err()
}
```

## 文件结构与职责
- `plug.go`：
  - 完成插件注册（init 注册到全局工厂）
  - 提供包级便捷方法 `GetUniversalRedis()`、`GetRedis()` 用于获取客户端
- `plugin_meta.go`：插件元数据常量（名称、配置前缀）与工厂函数 `NewRedisClient`
- `types.go`：定义插件实例 `PlugRedis` 的结构体与内部字段（配置、UniversalClient、采集协程控制等）
- `options.go`：将 protobuf 配置构建为 go-redis `redis.UniversalOptions` 的逻辑
- `hooks.go`：实现 go-redis v9 Hook（命令级别埋点：延迟直方图、错误计数）
- `health.go`：
  - 拓扑检测（single/cluster/sentinel）与地址解析
  - 启动/就绪检查（解析 INFO cluster/replication），并同步指标
  - 读取版本、运行信息、后台信息采集协程
- `lifecycle.go`：插件生命周期（初始化资源、启动任务、清理、配置注入、健康检查）
- `metrics.go`：Prometheus 指标定义与注册（连接池、命令、运行信息等）
- `pool_stats.go`：定时拉取并上报连接池统计（hits/misses/timeouts/idle/total/stale）
- `conf/redis.proto`：配置定义（protobuf），生成到 `plugins/nosql/redis/conf` 目录

## 健康检查与指标
- 启动时进行 Ping 并记录延迟；失败会计数并返回错误
- 就绪检查：
  - Cluster：解析 `INFO cluster` 判定 `cluster_state:ok`
  - 单机/哨兵：解析 `INFO replication` 判定 `role`、`connected_slaves`
- 指标：
  - 连接池：hits/misses/timeouts/idle/total/stale
  - 命令：延迟直方图、错误计数（按命令名标记）
  - 运行信息：redis_version、role、connected_slaves、cluster_state

## 常见问题
- 未找到 `protoc-gen-go`
  - 方案一：临时追加 PATH 后执行生成
    ```bash
    PATH="$(go env GOPATH)/bin:$PATH" make config
    ```
  - 方案二：仅对本插件执行显式插件路径
    ```bash
    cd lynx
    protoc -I plugins/nosql/redis/conf -I third_party -I boot -I app \
      --plugin=protoc-gen-go=$(go env GOPATH)/bin/protoc-gen-go \
      --go_out=paths=source_relative:plugins/nosql/redis/conf \
      plugins/nosql/redis/conf/redis.proto
    ```
- `addr` 字段相关编译错误
  - 说明：本插件已移除 `addr`（string），统一使用 `addrs`（repeated string）。
  - 现象：编译/生成代码时报找不到 `addr` 字段或结构体无该字段。
  - 处理：将单地址改为数组写法；应用代码中读取位置无需更改。
  - 迁移示例：
    - 旧：
      ```yaml
      redis:
        addr: "127.0.0.1:6379"
      ```
    - 新：
      ```yaml
      redis:
        addrs: ["127.0.0.1:6379"]
      ```
  - 已移除 `addr`，请改用 `addrs`（可配置单个地址）
- `MaxConnAge` / `IdleTimeout` 不生效
  - 当前 go-redis UniversalOptions 无对应字段，暂未映射（README 已标注）

## 版本与兼容性
- go-redis v9
- Prometheus client_golang v1.18+（已在 go.mod）
- 通过 `redis.UniversalClient` 同时支持单机/集群/哨兵

## 开发者提示
- 若需要进一步区分 Cluster 与 Failover 行为，可在 `detectMode()` 与 `enhancedReadinessCheck()` 中扩展
- 如果后续切换为更细粒度的客户端（Options/ClusterOptions/FailoverOptions），可补齐 `max_conn_age`、`idle_timeout` 的映射
