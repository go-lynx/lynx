# Lynx Code Review Issues (by Priority)

This document is based on a full review of the lynx repository (framework + cmd/lynx). Issues are grouped by **high / medium / low** priority with brief rationale to help decide ordering and approach.

---

## Verification Notes

- **Build**: `go build ./...` at lynx root and `go build .` under `lynx/cmd/lynx` both succeed.
- **Tests**: `TestCheckDuplicates` was added under `cmd/lynx/internal/project`, covering path-style names, dedup, trim, and invalid-name filtering; run with `go test ./internal/project/... -vet=off` (the package has existing vet warnings for `base.T(key)` non-constant format).
- **App package tests**: Some cases rely on `resetGlobalState` then calling `NewApp` again; due to `sync.Once` not being resettable they report "initialization channel not created"—this is a known design limitation, not introduced by recent changes.

---

## High-Priority Items Already Fixed (Emergency Optimization)

| # | Fix Summary |
|---|-------------|
| 1 | **Explicit error when user declines overwrite**: When user chooses not to overwrite, now `return fmt.Errorf("directory %s already exists and overwrite was declined", p.Name)` to avoid silent success. |
| 2 | **NewApp second call**: Added `log.Warnf` before returning the existing instance to make it clear that "new config and plugins are ignored". |
| 3 | **Rollback on BuildGrpcSubscriptions failure**: On subscription build failure, call `m.UnloadPlugins()` before returning the error to avoid half-started state (plugins up, subscriptions not built). |
| 4 | **Subscriptions configured but no control plane/discovery**: When gRPC subscriptions are configured and `controlPlane == nil` or `disc == nil`, call `m.UnloadPlugins()` and return an explicit error instead of silently returning nil. |
| 5 | **Path-style project names**: In `checkDuplicates`, allow path-style names containing `/`, with trim and dedup, aligned with `processProjectParams`. |
| 6 | **--force delete warning**: Before `os.RemoveAll(to)`, add `base.Warnf("--force: removing existing directory %s", to)` for audit and accidental-operation visibility. |
| 7 | **StopPlugin p==nil**: Added comment for "using pluginName in CleanupResources" that plugins should keep Name/ID consistent for cleanup. |
| 8 | **CircuitBreaker race**: In `CanExecute()`, Open→HalfOpen check and state update are done under the **same Lock()** to avoid multiple goroutines passing through at once. |

---

## I. High Priority (recommended to address first)

**Definition**: Bugs that cause wrong behavior, data loss, silent failure, or clearly violate user expectations; low fix cost, high impact.

| # | Summary | Location | Rationale |
|---|---------|----------|-----------|
| **1** | **Directory exists and user chose "do not overwrite" but call still returns success** | `cmd/lynx/internal/project/new.go` lines 44–46 | `err` from `os.Stat` was nil, so `return err` was `return nil`; callers assumed success while nothing was created. Scripts/CI can misjudge—**silent failure**. |
| **2** | **Second NewApp call ignores new config** | `lynx/app.go` ~179–184 | If global `lynxApp` exists, code returns `existing, nil`; the second `cfg` and `plugins` are ignored. In tests or multi-entry scenarios it is easy to assume "new config + NewApp" takes effect—**wrong behavior**. |
| **3** | **After LoadPlugins succeeds, gRPC subscription build failure still returns error but all plugins are already started** | `lynx/ops.go` ~49–76 | `loadSortedPluginsByLevel` runs first and succeeds, then `BuildGrpcSubscriptions`; if the latter fails, return error while plugins are already up. Caller may shutdown, but **intermediate state** (plugins up, subscriptions not built) is unclear in logs/monitoring and can leave a half-started state if shutdown is not done correctly. |
| **4** | **LoadPlugins returns nil when control plane / discovery is nil** | `lynx/ops.go` ~51–55, 73–74 | When `controlPlane == nil` or `disc == nil`, code only logs and `return nil`, so LoadPlugins is considered successful. Users who configured subscriptions but did not wire a control-plane plugin may think startup succeeded while **no gRPC subscriptions** exist—behavior does not match expectation. |
| **5** | **Project name validation vs path handling inconsistent; path-style names silently dropped** | `cmd/lynx/internal/project/project.go`: `checkDuplicates` vs `processProjectParams` | `checkDuplicates` uses regex `^[A-Za-z0-9_-]+$`, so path-style names (e.g. `foo/bar/svc`) are dropped; `processProjectParams` supports paths. `lynx new foo/bar/svc` can yield "no valid project name" or silently filtered list—**silent failure + inconsistency**. |
| **6** | **--force overwrite does RemoveAll with no second confirmation** | `cmd/lynx/internal/project/new.go` | With `--force`, code calls `os.RemoveAll(to)` directly—no confirmation, no backup. Mistaken run on an existing project dir **irreversibly deletes** it—**data loss risk**. |
| **7** | **StopPlugin uses pluginName for CleanupResources when p==nil** | `lynx/ops.go` ~444–446 | Other paths use `p.ID()` as resource key; here `pluginName` (Name) is used. If Name and ID differ, wrong resources may be cleaned or cleanup may miss—**leak or wrong plugin cleanup**. |
| **8** | **CircuitBreaker race on Open→HalfOpen** | `lynx/boot/application.go` ~403–412 | In `CanExecute()` under Open, code does RUnlock then Lock to set HalfOpen; in that window multiple goroutines can all set HalfOpen and return true—**multiple probe requests let through**, violating circuit semantics. |

---

## II. Medium Priority (schedule in iteration)

**Definition**: Affects maintainability, observability, or only shows in specific scenarios; or design is not robust but has current mitigations.

| # | Summary | Location | Rationale |
|---|---------|----------|-----------|
| 9 | Template assumes `cmd/user` exists, no check | `cmd/lynx/internal/project/new.go` ~64–69 | Direct `os.Rename(cmd/user, cmd/<p.Name>)`; if template repo has no `cmd/user`, it fails with unclear error. |
| 10 | UnloadPlugins race window after total timeout vs forced cleanup | `lynx/ops.go` ~152–174 | After total timeout, plugins "not yet cleaned" are force-cleaned; despite `cleaningUp` guard, edge cases can still race with in-flight Stop/Cleanup; non-idempotent plugins are at risk. |
| 11 | UnloadPluginsByName has no total timeout, runs serially | `lynx/ops.go` ~325–428 | Stop + Cleanup one by one, no total timeout; one stuck plugin blocks forever, no concurrency control—slow with many plugins. |
| 12 | run --build-args parsed with strings.Fields, breaks args with spaces | `cmd/lynx/internal/run/runner.go` | e.g. `-ldflags="-s -w"` gets split incorrectly; complex build args can fail or behave wrong. |
| 13 | run validateProject only checks cmd/ exists, not main.go | `cmd/lynx/internal/run/run.go` | Empty `cmd/` still passes; error appears only at build time. |
| 14 | watch mode Debouncer.Trigger vs Timer race | `cmd/lynx/internal/run/watcher.go` | When `timer.Stop()` returns false the callback may already be running; scheduling `AfterFunc` again can trigger twice—one save can cause multiple restarts. |
| 15 | doctor expected dirs vs lynx new output mismatch | `cmd/lynx/internal/doctor/checks.go` | Expects `app/boot/plugins/cmd/...`; lynx new produces `cmd/server/`, `configs/`, `internal/`, etc.; new projects get false "missing dir" reports. |
| 16 | doctor --fix depends on current working directory | `cmd/lynx/internal/doctor/checks.go` | Fix uses current dir's go.mod, Makefile, etc.; running from a subdir can fix wrong dir or create wrong files. |
| 17 | TypedBasePlugin.Stop only runs when StatusActive | `lynx/plugins/base.go` ~247–250 | When not Active (e.g. Initializing/Stopping/Failed) returns ErrPluginNotActive; rollback or force unload cannot consistently use Stop, relies on CleanupResources—behavior is non-obvious. |
| 18 | sync.Once prevents re-init after Close | `lynx/app.go` | resetInitState does not clear initOnce; after Close, NewApp does not re-run init—process restart required; easy to miss if docs/comments are not prominent. |
| 19 | DefaultControlPlane returns all nil | `lynx/controlplane.go` | Without control plane, Registry/Discovery/Router/GetConfig are nil; any missing nil check can panic—all call sites need guards. |
| 20 | NewKratos depends on global Lynx state | `lynx/kratos/kratos.go` | Uses GetHost/GetName/GetVersion, log.Logger, etc.; with multiple instances or tests, if NewApp was not called or global app not replaced, wrong or stale values can be used. |

---

## III. Low Priority (fix when touching code or long-term)

**Definition**: Code quality, maintainability, compatibility (e.g. deprecated API), or docs/UX.

| # | Summary | Location | Rationale |
|---|---------|----------|-----------|
| 21 | Plugin module paths hardcoded; new plugins require CLI change | `cmd/lynx/internal/project/new.go` getPluginModulePath | High maintenance; easy to miss or typo; consider config or registry. |
| 22 | plugin copyDir does not preserve permissions or symlinks | `cmd/lynx/internal/plugin/manager.go` | Fixed 0644, no symlink handling; cross-filesystem or executable plugins may misbehave. |
| 23 | Root command PersistentPreRun ignores flag errors | `cmd/lynx/main.go` | GetBool/GetString errors ignored; invalid flags silently use defaults. |
| 24 | validateConfig only checks key existence, not format/length | `lynx/boot/configuration.go` | Empty or invalid values may surface later; could tighten validation or document contract. |
| 25 | lifecycle rollback loop uses defer cleanupCancel | `lynx/lifecycle.go` ~243–244 | Many defers when rolling back many plugins; correct but less readable; consider explicit cancel in loop. |
| 26 | total_resources / total_size type assertions in health check are long and easy to miss | `lynx/boot/application.go` ~398–419 | Repeated type branches, default handling inconsistent; could extract helper and unify types. |
| 27 | strings.Title deprecated | `cmd/lynx/internal/plugin/list.go`, `doctor/doctor.go` | Go 1.18+ deprecated; could use `golang.org/x/text/cases`. |
| 28 | doctor Markdown table has extra \| | `cmd/lynx/internal/doctor/doctor.go` ~381 | Header separator has one extra `|`, breaks rendering. |
| 29 | Event health check interval 30s hardcoded | `lynx/app.go`, events package | Not configurable per environment. |
| 30 | SetGlobalConfig relies on event order and config_version contract | `lynx/app.go` | If subscribers do not respect version/order, ordering can be wrong; document or enforce in API. |

---

## Follow-up Fixes (medium/low)

| Item | Fix Summary |
|------|--------------|
| **M#9** | Template `cmd/user` existence check: before `os.Rename`, verify `cmd/user` exists; if not, return explicit error suggesting to check repo branch or layout. |
| **M#12** | run `--build-args` parsing: add `parseBuildArgs()` supporting quoted argument strings (e.g. `-ldflags="-s -w"`) so they are not broken by `strings.Fields`. |
| **M#13** | run `validateProject`: when `cmd/` exists, require at least one `cmd/<subdir>/main.go`; empty `cmd/` no longer passes. |
| **M#14** | watch Debouncer race: in `Trigger`, if `timer.Stop()` returns false (already fired), do not schedule a new timer to avoid overlapping with in-flight callback. |
| **M#15** | doctor project layout: align expected dirs with lynx new: `cmd`, `configs`, `internal`. |
| **L#27** | Replace deprecated `strings.Title`: use local title-case helper in doctor and plugin/list (`titleCase` / `titleCasePlugin`), no new deps. |
| **L#28** | doctor Markdown table: remove extra `\|` in header separator. |

---

## IV. Usage Recommendations

1. **High priority**: Read the "Rationale" column above and confirm fit with your use case, then decide what to fix in this iteration; **#1, #2, #5, #6, #7, #8** are especially important for correctness/data safety.
2. **Medium priority**: Can be done with new features or refactors (e.g. Unload timeout/concurrency for #10, #11; CLI run changes for #12, #13, #14).
3. **Low priority**: Fix when touching related code (e.g. #27, #28) or add to tech-debt list.

For detailed **change ideas** or **patch sketches** on any item, specify the issue number or file and a separate change plan can be written.
