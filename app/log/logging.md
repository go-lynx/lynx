# Lynx 日志系统使用文档

本日志系统基于 zerolog + Kratos 统一封装，具备以下能力：

- 统一日志级别过滤（Kratos 与 zerolog 一致）
- 支持可配置时区（timezone 字符串），默认本地时区
- 支持 caller 源码位置信息，并可配置 `caller_skip`
- 支持堆栈采集（可配置阈值、最大帧数、过滤前缀）
- 支持 info/debug 采样与每秒限流
- 支持配置热更新（当前实现为 2s 轮询降级方案）
- 采样使用包级本地 RNG，不修改全局 `math/rand` 的种子

日志核心代码位于：
- `app/log/logger.go`
- `app/log/lynx_log.go`
- 配置 Proto：`app/log/conf/log.proto`

## 配置结构（YAML）

配置路径键为 `lynx.log`（即在 YAML 中 `lynx:` 下的 `log:`）。

示例：见下文「完整示例」章节。

### 顶层字段
- `level`: 日志级别（debug/info/warn/error）。默认 `info`。
- `console_output`: 是否输出到控制台（bool）。
- `file_output`: 是否输出到文件（bool）。
- `file_path`: 日志文件路径（仅当 `file_output=true` 时有效）。
- `max_size`: 单个日志文件最大大小（MB）。
- `max_age`: 日志保留天数。
- `max_backups`: 保留的备份文件数量。
- `compress`: 是否压缩滚动日志。
- `timezone`: 日志时间戳时区（如 `Asia/Shanghai`、`UTC`）。未配置默认使用本地时区。
- `caller_skip`: caller 源码位置的栈深度偏移量，默认 5。

### Stack（堆栈）
`lynx.log.stack`
- `enable`: 是否启用堆栈输出。
- `skip`: 采集堆栈时跳过的帧数（用于剔除日志内部封装栈）。
- `max_frames`: 最大采集帧数。
- `level`: 触发堆栈输出的最低日志级别（debug/info/warn/error/fatal）。
- `filter_prefixes`: 过滤的帧前缀列表（包名或文件路径前缀）。

### Sampling（采样与限流）
`lynx.log.sampling`
- `enable`: 是否启用采样/限流。
- `info_ratio`: info 日志采样比例 [0,1]，0 表示全部丢弃，1 表示全量保留。
- `debug_ratio`: debug 日志采样比例 [0,1]。
- `max_info_per_sec`: info 每秒最大条数（0 表示不限制）。
- `max_debug_per_sec`: debug 每秒最大条数（0 表示不限制）。

说明：采样与限流目前仅对 `info/debug` 生效；`warn/error` 不受影响。

## 动态热更新

- 优先使用配置源的 Watch 机制；若不支持 Watch，则降级为每 2 秒轮询 `lynx.log`。
- 当前支持热更新的字段：`level`、`timezone`、`caller_skip`。

## 使用方式

- 初始化在应用启动时由 `boot/strap.go` 调用 `log.InitLogger(...)`，无需业务侧显式调用。
- 在业务代码中使用快捷方法：
  - `log.Debug/Info/Warn/Error/Fatal`
  - 带上下文：`log.InfoCtx(ctx, ...)` 等
  - 结构化：`log.Infow("key", val, ...)`

## 完整示例（configs/log-example.yaml）

```yaml
lynx:
  log:
    level: info                # debug/info/warn/error
    console_output: true
    file_output: true
    file_path: logs/lynx.log
    max_size: 128              # MB
    max_age: 7                 # days
    max_backups: 5
    compress: true

    timezone: Asia/Shanghai    # 例如 Asia/Shanghai、UTC；不配置则为本地时区
    caller_skip: 5

    stack:
      enable: true
      skip: 6
      max_frames: 32
      level: error             # 达到该级别及以上输出堆栈
      filter_prefixes:         # 过滤内部调用栈前缀
        - github.com/go-kratos/kratos
        - github.com/rs/zerolog
        - github.com/go-lynx/lynx/app/log

    sampling:
      enable: true
      info_ratio: 0.5          # 50% 保留 info
      debug_ratio: 0.2         # 20% 保留 debug
      max_info_per_sec: 50     # 每秒最多 50 条 info（0=不限）
      max_debug_per_sec: 20    # 每秒最多 20 条 debug（0=不限）
```

## 常见问题

- 为什么 info/debug 有时不打印？
  - 若启用了 `sampling`，可能被比例采样或每秒限流所丢弃；请检查 `info_ratio/debug_ratio` 与 `max_*_per_sec`。

- 修改配置多久生效？
  - 若配置源支持 Watch：`level`、`timezone`、`caller_skip` 变更后即时生效；其他字段当前不支持热更新。
  - 若配置源不支持 Watch（降级轮询）：上述三个字段在 2 秒内生效；其他字段需重启或等待后续支持。

- 采样的随机性如何保证？
  - 采样模块使用包级本地 `*rand.Rand`，在进程启动时用加密安全随机源播种（失败则退回时间种子），不会影响全局 `math/rand`。

- 如何定位 caller 深度不准确？
  - 调整 `caller_skip`，或根据你的封装栈层级增减。

- 堆栈过长或包含无关帧？
  - 调整 `max_frames` 与 `filter_prefixes`；或提高 `stack.level` 门限。
