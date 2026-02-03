# Lynx 代码审查问题清单（按优先级）

本文档基于对 lynx 全仓库（框架 + cmd/lynx）的审查，按**高 / 中 / 低**优先级整理，并说明原因，便于你过目后决定修改顺序与方案。

---

## 验证说明

- **构建**：`lynx` 根目录 `go build ./...`、`lynx/cmd/lynx` 下 `go build .` 均通过。
- **测试**：`cmd/lynx/internal/project` 已增加 `TestCheckDuplicates`，覆盖路径式名称、去重、trim、非法名过滤，使用 `go test ./internal/project/... -vet=off` 通过（当前包内存在 `base.T(key)` 非常量 format 的 vet 告警，为既有问题）。
- **app 包测试**：部分用例依赖 `resetGlobalState` 后再次 `NewApp`，因 `sync.Once` 无法重置会报「initialization channel not created」，属既有设计限制，非本次修改引入。

---

## 已修复的高优先级项（本次紧急优化）

| # | 修复内容 |
|---|----------|
| 1 | **不覆盖时返回明确错误**：用户选择不覆盖时改为 `return fmt.Errorf("directory %s already exists and overwrite was declined", p.Name)`，避免静默返回成功。 |
| 2 | **NewApp 二次调用**：在返回已有实例前增加 `log.Warnf`，明确提示「新配置与插件被忽略」。 |
| 3 | **BuildGrpcSubscriptions 失败时回滚**：订阅构建失败时先调用 `m.UnloadPlugins()` 再返回错误，避免插件已启、订阅未建的半启动状态。 |
| 4 | **订阅已配置但无控制面/发现**：当配置了 gRPC 订阅且 `controlPlane == nil` 或 `disc == nil` 时，先 `m.UnloadPlugins()` 再返回明确错误，不再静默 return nil。 |
| 5 | **路径式项目名**：`checkDuplicates` 中允许含 `/` 的路径式名称通过，并做 trim 与去重，与 `processProjectParams` 行为一致。 |
| 6 | **--force 删除前告警**：在 `os.RemoveAll(to)` 前增加 `base.Warnf("--force: removing existing directory %s", to)`，便于审计与误操作发现。 |
| 7 | **StopPlugin p==nil**：为「用 pluginName 做 CleanupResources」增加注释，说明插件应保持 Name/ID 一致以便清理。 |
| 8 | **CircuitBreaker 竞态**：`CanExecute()` 中 Open→HalfOpen 的判定与状态更新改为在**同一把 Lock()** 下完成，消除多 goroutine 同时放行的竞态。 |

---

## 一、高优先级（建议优先处理）

**定义**：会导致错误行为、数据丢失、静默失败或明显违背用户预期的缺陷；修复成本低、影响面大。

| # | 问题简述 | 位置 | 原因说明 |
|---|----------|------|----------|
| **1** | **目录已存在且用户选择「不覆盖」时仍返回成功** | `cmd/lynx/internal/project/new.go` 第 44–46 行 | 此时 `err` 来自 `os.Stat` 且为 `nil`，`return err` 等于 `return nil`，调用方会认为项目创建成功，实际未创建任何内容。脚本/CI 会误判，属于**静默失败**。 |
| **2** | **NewApp 二次调用会忽略新配置** | `lynx/app.go` 约 179–184 行 | 若全局已存在 `lynxApp`，直接 `return existing, nil`，第二次传入的 `cfg` 与 `plugins` 被完全忽略。测试或多入口场景下容易误以为「换配置再调 NewApp」会生效，属于**错误行为**。 |
| **3** | **LoadPlugins 成功后，gRPC 订阅构建失败仍返回错误但插件已全部启动** | `lynx/ops.go` 约 49–76 行 | 先执行 `loadSortedPluginsByLevel` 成功，再 `BuildGrpcSubscriptions`；若后者失败则直接 return error，此时插件已全部启动。调用方会进入 shutdown，但**中间状态**（插件已启、订阅未建）在日志与监控上不直观，排查成本高；且若上层未正确 shutdown，可能留下半启动状态。 |
| **4** | **control plane / discovery 为 nil 时 LoadPlugins 仍 return nil** | `lynx/ops.go` 约 51–55、73–74 行 | `controlPlane == nil` 或 `disc == nil` 时只打日志并 `return nil`，LoadPlugins 被视为成功。用户若配置了订阅但未接入控制面插件，会误以为启动成功，实际**没有任何 gRPC 订阅**，行为与预期不符。 |
| **5** | **项目名校验与路径处理不一致，带路径名被静默丢弃** | `cmd/lynx/internal/project/project.go`：`checkDuplicates` vs `processProjectParams` | `checkDuplicates` 用正则 `^[A-Za-z0-9_-]+$` 过滤，带路径的名字（如 `foo/bar/svc`）会被整条丢弃；`processProjectParams` 却支持路径。用户执行 `lynx new foo/bar/svc` 可能得到「无有效项目名」或名单被静默过滤，属于**静默失败 + 行为不一致**。 |
| **6** | **--force 覆盖时直接 RemoveAll，无二次确认** | `cmd/lynx/internal/project/new.go` | 使用 `--force` 时直接 `os.RemoveAll(to)`，无确认、无备份。误对已有项目目录执行会**不可恢复地删除**，属于**数据丢失风险**。 |
| **7** | **StopPlugin 在 p==nil 时用 pluginName 调 CleanupResources** | `lynx/ops.go` 约 444–446 行 | 其他路径多用 `p.ID()` 作为资源 key，此处用 `pluginName`（即 Name）。若 Name 与 ID 不一致，可能清错资源或清不到，导致**资源泄漏或误清其他插件**。 |
| **8** | **CircuitBreaker 在 Open→HalfOpen 时存在竞态** | `lynx/boot/application.go` 约 403–412 行 | `CanExecute()` 在 Open 状态下先 `RUnlock` 再 `Lock` 改状态为 HalfOpen，中间窗口内多个 goroutine 可能同时改为 HalfOpen 并都返回 true，导致**多次试探请求同时放行**，违背熔断语义。 |

---

## 二、中优先级（建议在迭代中安排）

**定义**：影响可维护性、可观测性或在特定场景下才会暴露；或设计不够健壮但当前有缓解手段。

| # | 问题简述 | 位置 | 原因说明 |
|---|----------|------|----------|
| 9 | 模板强依赖 `cmd/user`，无存在性检查 | `cmd/lynx/internal/project/new.go` 约 64–69 行 | 直接 `os.Rename(cmd/user, cmd/<p.Name>)`，模板仓库无 `cmd/user` 时会失败，错误信息不明确，排查成本高。 |
| 10 | UnloadPlugins 总超时后强制清理与 goroutine 的竞态窗口 | `lynx/ops.go` 约 152–174 行 | 总超时后对「未开始清理」的插件做强制清理，虽有 `cleaningUp` 防护，极端情况下仍可能与正在执行的 Stop/Cleanup 产生竞态，若插件实现不幂等有风险。 |
| 11 | UnloadPluginsByName 无总超时、串行执行 | `lynx/ops.go` 约 325–428 行 | 逐个 Stop + Cleanup，无总超时；若某插件卡住会一直阻塞，且无并发控制，插件多时耗时长。 |
| 12 | run 的 --build-args 用 strings.Fields 解析，带空格参数会错 | `cmd/lynx/internal/run/runner.go` | 如 `-ldflags="-s -w"` 会被拆坏，复杂构建参数易传错或构建失败。 |
| 13 | run 的 validateProject 只检查 cmd/ 存在，不保证有 main.go | `cmd/lynx/internal/run/run.go` | 空 `cmd/` 也会通过，错误在 build 阶段才暴露，提示滞后。 |
| 14 | watch 模式 Debouncer.Trigger 与 Timer 竞态 | `cmd/lynx/internal/run/watcher.go` | `timer.Stop()` 返回 false 时回调可能已在执行，再 `AfterFunc` 会再触发一次，一次保存可能触发多次重启。 |
| 15 | doctor 期望目录与 lynx new 产出不一致 | `cmd/lynx/internal/doctor/checks.go` | 期望 `app/boot/plugins/cmd/...`，lynx new 生成的是 `cmd/server/`、`configs/`、`internal/` 等，新建项目会被报缺目录。 |
| 16 | doctor --fix 依赖当前工作目录 | `cmd/lynx/internal/doctor/checks.go` | Fix 使用当前目录的 go.mod、Makefile 等，在子目录执行可能修错目录或误建文件。 |
| 17 | TypedBasePlugin.Stop 仅允许 StatusActive 时执行 | `lynx/plugins/base.go` 约 247–250 行 | 非 Active（如 Initializing/Stopping/Failed）时返回 ErrPluginNotActive，回滚或强制卸载时无法统一通过 Stop 清理，依赖 CleanupResources 兜底，行为不直观。 |
| 18 | sync.Once 导致 Close 后无法重新初始化 | `lynx/app.go` | resetInitState 不清 initOnce，Close 后再调 NewApp 不会重新执行初始化，需重启进程；若文档/注释不醒目易被忽略。 |
| 19 | DefaultControlPlane 全部返回 nil | `lynx/controlplane.go` | 未接入控制面时，Registry/Discovery/Router/GetConfig 均为 nil，若某处未判空会 panic，需保证所有调用点都有防护。 |
| 20 | NewKratos 依赖全局 Lynx 状态 | `lynx/kratos/kratos.go` | 使用 GetHost/GetName/GetVersion、log.Logger 等全局状态，多实例或测试中若未先 NewApp 或替换全局 app，可能拿到错误或过期的值。 |

---

## 三、低优先级（可随改动顺带处理或长期优化）

**定义**：代码质量、可维护性、兼容性（如废弃 API）或文档/体验类问题。

| # | 问题简述 | 位置 | 原因说明 |
|---|----------|------|----------|
| 21 | 插件模块路径硬编码，新插件需改 CLI | `cmd/lynx/internal/project/new.go` getPluginModulePath | 维护成本高，新插件易漏或写错；可考虑从配置或 registry 读取。 |
| 22 | plugin copyDir 不保留权限与符号链接 | `cmd/lynx/internal/plugin/manager.go` | 固定 0644、未处理符号链接，跨盘或需可执行权限的插件可能异常。 |
| 23 | 根命令 PersistentPreRun 忽略 flag 错误 | `cmd/lynx/main.go` | GetBool/GetString 的 error 被忽略，异常 flag 时静默用默认值，不利于排查。 |
| 24 | validateConfig 只校验 key 存在，不校验格式/长度 | `lynx/boot/configuration.go` | 空字符串或非法值可能到后续才暴露，可加强校验或在上层约定。 |
| 25 | lifecycle 回滚循环内 defer cleanupCancel | `lynx/lifecycle.go` 约 243–244 行 | 回滚插件多时 defer 数量多，可读性差；逻辑正确，可考虑显式在循环内 cancel。 |
| 26 | 健康检查里 total_resources / total_size 类型断言冗长易漏 | `lynx/boot/application.go` 约 398–419 行 | 多种类型分支重复，default 处理不统一，可抽成辅助函数并统一类型。 |
| 27 | strings.Title 已废弃 | `cmd/lynx/internal/plugin/list.go`、`doctor/doctor.go` | Go 1.18+ 弃用，可改为 `golang.org/x/text/cases`。 |
| 28 | doctor Markdown 表格多一个 \| | `cmd/lynx/internal/doctor/doctor.go` 约 381 行 | 表头分隔行 `\|-------\|--------\|---------|\|` 多一个 `|`，渲染异常。 |
| 29 | 事件健康检查间隔 30s 硬编码 | `lynx/app.go`、events 包 | 无法按环境配置，可考虑从配置读取。 |
| 30 | SetGlobalConfig 依赖事件序号与 config_version 的约定 | `lynx/app.go` | 若订阅方不按版本/序号处理，可能乱序；可在文档或接口上明确约定。 |

---

## 本次继续修复（中/低优先级）

| 项 | 修复内容 |
|---|----------|
| **中#9** | 模板 `cmd/user` 存在性检查：在 `os.Rename` 前检查 `cmd/user` 是否存在，不存在则返回明确错误，提示检查 repo 分支或 layout 结构。 |
| **中#12** | run `--build-args` 解析：新增 `parseBuildArgs()`，支持双引号包裹的参数字符串（如 `-ldflags="-s -w"`），避免被 `strings.Fields` 拆坏。 |
| **中#13** | run `validateProject`：在存在 `cmd/` 时要求至少有一个 `cmd/<子目录>/main.go`，空 `cmd/` 不再通过校验。 |
| **中#14** | watch Debouncer 竞态：`Trigger` 中若 `timer.Stop()` 返回 false（已触发），则不再调度新 timer，避免与正在执行的回调并发。 |
| **中#15** | doctor 项目结构：期望目录改为与 `lynx new` 一致：`cmd`、`configs`、`internal`。 |
| **低#27** | 废弃 `strings.Title`：在 doctor 与 plugin/list 中改为首字母大写的本地 helper（`titleCase` / `titleCasePlugin`），无新依赖。 |
| **低#28** | doctor Markdown 表格：表头分隔行去掉多余 `\|`，修正为 `\|-------\|--------\|----------|\`。 |

---

## 四、使用建议

1. **高优先级**：建议先通读上表「原因说明」，确认是否与你的使用场景一致，再决定是否在本迭代内修；其中 **#1、#2、#5、#6、#7、#8** 更偏向「正确性/数据安全」，建议优先。
2. **中优先级**：可与新功能或重构一起做（例如改 Unload 超时与并发时顺带做 #10、#11；改 CLI run 时顺带做 #12、#13、#14）。
3. **低优先级**：可在改到相关文件时顺带修（如 #27、#28），或列入技术债清单分批做。

如需对某一项的**修改思路**或**补丁示例**做展开，可以指定序号或文件，再单独写一版修改方案。
