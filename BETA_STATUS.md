# Beta Status

更新时间：2026-04-02

## 范围与判定口径

- 通过扫描工作区内各模块 `go.mod`，统计仍显式依赖 `github.com/go-lynx/lynx v1.6.0-beta` 的模块。
- 额外扫描插件源码中的 `pluginVersion` / `PluginVersion` 常量，识别仍对外自报 `v1.6.0-beta` 的插件。
- 本文只记录“当前仍在使用 beta 版本”的模块，不包含核心仓库 `github.com/go-lynx/lynx` 本身，因为它不是自己的依赖方。

## 汇总结论

- 共有 `27` 个 Go 模块仍在 `go.mod` 中依赖 `github.com/go-lynx/lynx v1.6.0-beta`。
- 其中 `23` 个插件模块仍在源码中对外暴露 `v1.6.0-beta` 版本常量。
- `go.mod` 仍依赖 beta、但源码未自报 beta 的模块有 `4` 个：
  - `lynx-layout`
  - `lynx-sql-sdk`
  - `lynx-redis-lock`
  - `lynx-etcd-lock`
- 独立 CLI 模块 `lynx/cmd/lynx` 也仍依赖 beta 核心；它不属于插件，因此没有插件版本常量。

## 风险分级说明

| 等级 | 含义 |
| --- | --- |
| 高 | 公开 API 面广，或直接承载插件生命周期、服务启动、框架扩展点；稳定版若调整接口/语义，业务接入面会立即受影响。 |
| 中 | API 面中等，主要封装外部中间件；升级风险集中在配置项、生命周期回调、监控指标或辅助能力变更。 |
| 低 | 封装较薄或场景较单一；预计主要受版本号、导出符号或少量配置兼容影响。 |

## 模块清单

| 模块目录 | 模块路径 | Beta 证据 | 风险 | 升级到稳定版的 API 兼容性评估 |
| --- | --- | --- | --- | --- |
| `lynx/cmd/lynx` | `github.com/go-lynx/lynx/cmd/lynx` | `go.mod` 依赖 beta 核心 | 高 | 该模块负责项目脚手架与模板生成；若稳定版调整插件注册、目录模板、配置前缀或初始化方式，生成项目会直接漂移。 |
| `lynx-apollo` | `github.com/go-lynx/lynx-apollo` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 主要风险在配置中心监听、默认值与校验逻辑；若核心插件生命周期或配置装载接口变更，需要适配 watcher/cleanup/resilience 流程。 |
| `lynx-dtm` | `github.com/go-lynx/lynx-dtm` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | API 主要围绕分布式事务客户端；若稳定版改变上下文透传、插件注册或错误模型，事务接入点需要小幅调整。 |
| `lynx-elasticsearch` | `github.com/go-lynx/lynx-elasticsearch` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 模块对外暴露配置、健康检查与指标能力；若稳定版调整插件基类或指标接线方式，兼容成本可控但需要逐项验配置。 |
| `lynx-eon-id` | `github.com/go-lynx/lynx-eon-id` | `go.mod` 依赖 beta；`PluginVersion = "v1.6.0-beta"` | 中 | 该模块是 ID 生成与 Redis Worker 协调封装；若稳定版改变插件基类或资源注册方式，会影响初始化与集成测试，但业务调用面相对稳定。 |
| `lynx-etcd` | `github.com/go-lynx/lynx-etcd` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 风险集中在配置监听、注册发现与 TLS/生命周期管理；稳定版若改动 runtime/config 接口，需要适配多个子路径。 |
| `lynx-etcd-lock` | `github.com/go-lynx/lynx/plugins/etcd-lock` | `go.mod` 依赖 beta 核心 | 低 | 锁插件封装较薄，升级主要关注插件接口签名、上下文控制和错误类型变化。 |
| `lynx-grpc` | `github.com/go-lynx/lynx-grpc` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"`；本地 `replace ../lynx` | 高 | 该模块承载服务端、客户端、TLS、拦截器、连接池与监控；稳定版若调整核心生命周期、插件事件或中间件装配接口，影响面很大。 |
| `lynx-http` | `github.com/go-lynx/lynx-http` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"`；本地 `replace ../lynx` | 高 | HTTP 服务插件对路由、编码器、中间件、熔断、追踪有大量扩展点；稳定版若改动 server/runtime 契约，兼容性风险高。 |
| `lynx-kafka` | `github.com/go-lynx/lynx-kafka` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 生产者、消费者、SASL、重试、监控等能力较多；稳定版主要风险在插件初始化、指标注册和配置结构变更。 |
| `lynx-layout` | `github.com/go-lynx/lynx-layout` | `go.mod` 依赖 beta 核心 | 高 | 该模板项目同时依赖多个已发版插件，但核心仍是 beta；稳定版一旦改变应用启动、插件加载或生成代码约定，模板将出现整链路兼容问题。 |
| `lynx-mongodb` | `github.com/go-lynx/lynx-mongodb` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 模块对外暴露配置、TLS、读写关注、压缩与指标能力；稳定版若调整插件资源发布或配置扫描，需回归但预计不需要大改业务 API。 |
| `lynx-mssql` | `github.com/go-lynx/lynx-mssql` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | MSSQL 建立在 `lynx-sql-sdk` 之上；若稳定版调整 SQL 基座、连接池配置或插件生命周期，会同时影响配置兼容与运行时行为。 |
| `lynx-mysql` | `github.com/go-lynx/lynx-mysql` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | MySQL 插件广泛参与模板项目与数据层；依赖核心插件体系和 `lynx-sql-sdk`，稳定版改动会直接传导到 `ent`/数据接线。 |
| `lynx-nacos` | `github.com/go-lynx/lynx-nacos` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 配置中心与注册中心能力较多，生命周期与兼容层文件较多；升级重点在配置监听、事件桥接与注册发现 API。 |
| `lynx-pgsql` | `github.com/go-lynx/lynx-pgsql` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | PgSQL 同时接入 tracing、prometheus 和 SQL 基座；稳定版若调整插件 runtime、metrics recorder 或 base SQL 契约，回归面较大。 |
| `lynx-polaris` | `github.com/go-lynx/lynx-polaris` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | 该模块覆盖服务治理、注册发现、限流、负载均衡、watcher 与兼容层；稳定版改动对外部暴露面和运行语义影响都较大。 |
| `lynx-pulsar` | `github.com/go-lynx/lynx-pulsar` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | Pulsar 插件封装客户端、管理器和接口层；主要兼容风险在插件初始化、配置结构和连接资源管理。 |
| `lynx-rabbitmq` | `github.com/go-lynx/lynx-rabbitmq` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 封装连接池、客户端、指标与错误模型；若稳定版改变插件注册/资源暴露方式，需要小到中等规模适配。 |
| `lynx-redis` | `github.com/go-lynx/lynx-redis` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | Redis 是模板和多个插件的基础依赖，且模块包含配置、指标、生命周期、增强特性和校验链路；稳定版升级需要重点验证资源发布与配置兼容。 |
| `lynx-redis-lock` | `github.com/go-lynx/lynx-redis-lock` | `go.mod` 依赖 beta 核心 | 中 | 模块虽然聚焦分布式锁，但依赖 Redis 插件对外 API；若稳定版改变 Redis 资源获取方式或插件管理接口，会有连带兼容成本。 |
| `lynx-rocketmq` | `github.com/go-lynx/lynx-rocketmq` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 插件面向消息客户端封装，风险主要在初始化、资源池、错误处理与监控挂接；业务侧调用面相对集中。 |
| `lynx-seata` | `github.com/go-lynx/lynx-seata` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | Seata 属于事务基础设施，稳定版若改变 context、hook 或错误抽象，适配点不多但回归必须谨慎。 |
| `lynx-sentinel` | `github.com/go-lynx/lynx-sentinel` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 涉及规则、仪表盘、middleware 和兼容层；稳定版如调整中间件装配或插件元数据接口，需要同步修改。 |
| `lynx-sql-sdk` | `github.com/go-lynx/lynx-sql-sdk` | `go.mod` 依赖 beta 核心 | 高 | 这是 MySQL/PgSQL/MSSQL 的公共基座；稳定版只要调整一个基类接口，就会把兼容问题放大到多个 SQL 插件。 |
| `lynx-swagger` | `github.com/go-lynx/lynx-swagger` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 中 | 该模块主要提供文档生成与 UI 服务；兼容风险相对集中在插件装配、配置结构和静态资源暴露。 |
| `lynx-tracer` | `github.com/go-lynx/lynx-tracer` | `go.mod` 依赖 beta；`pluginVersion = "v1.6.0-beta"` | 高 | 追踪插件与 HTTP、gRPC、SQL 链路耦合紧密；稳定版若改动上下文、生命周期或插件间资源获取接口，会影响整条可观测性链。 |

## 建议的收尾顺序

1. 先处理 `高` 风险模块：`lynx-grpc`、`lynx-http`、`lynx-tracer`、`lynx-sql-sdk`、`lynx-mysql`、`lynx-pgsql`、`lynx-mssql`、`lynx-redis`、`lynx-polaris`、`lynx-layout`、`lynx/cmd/lynx`。
2. 再处理 `中` 风险基础设施模块：配置中心、服务治理、消息队列、事务相关插件。
3. 最后处理 `低` 风险锁类模块，并统一收尾插件版本常量与 README/发布说明。

## 备注

- 本文是静态仓库审计结果，判定时间点为 `2026-04-02`。
- 风险评估基于当前模块角色、对核心插件生命周期的耦合程度、导出 API 面大小，以及是否承担其他插件/模板的基础设施职责。
