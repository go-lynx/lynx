# Lynx Core Refocus Plan

See also: [Structure Classification And Execution Plan](./structure_classification_plan.md)

## Purpose

This document records the agreed refocus direction for Lynx core so future work can follow a stable plan instead of rediscovering the boundary every time.

This document focuses on refactoring direction inside the core. For the broader
classification of what belongs to core, optional shell, or compatibility
surface, see the structure classification plan.

The target is clear:

- Lynx core is a plugin orchestration framework.
- Lynx core is not a runtime hot-reload platform.
- Lynx core is not responsible for live plugin replacement, in-process rolling upgrade, or other concerns already handled well by external rollout systems such as Kubernetes.

## Core Boundary

Lynx core should concentrate on these responsibilities:

- Plugin registration and identity management
- Dependency graph validation and startup order
- Lifecycle orchestration: initialize, start, stop, cleanup
- Runtime resource ownership and isolation
- Event delivery needed for plugin coordination
- Failure containment, shutdown ordering, and cleanup guarantees
- Observability around plugin state, resource state, and lifecycle failures

Lynx core should not treat these as first-class framework guarantees:

- Runtime plugin hot reload
- In-process plugin upgrade or replacement
- Runtime config rollout across already loaded plugins
- Zero-downtime upgrade orchestration inside one process

Those concerns should be handled by:

- Process restart
- Kubernetes rolling update
- External config/control systems owned by plugins themselves

## Design Principles

### 1. Prefer Restart Over In-Process Mutation

When a configuration change affects loaded plugins, Lynx core should prefer:

- clear detection
- explicit refusal or restart-required reporting
- predictable shutdown and restart behavior

instead of:

- partial in-process reconfiguration
- plugin-by-plugin rollback choreography
- hidden best-effort mutation of already running components

### 2. Make State Transitions Boring

Plugin state should be easy to reason about and easy to verify.

Primary lifecycle states should remain:

- `inactive`
- `initializing`
- `active`
- `stopping`
- `terminated`
- `failed`

States such as `upgrading` and `rollback` may remain for compatibility in the short term, but they are not strategic core states and should be removed from the default core path over time.

### 3. One Runtime View, Not Snapshots

Plugins should observe a stable shared runtime view. The runtime should not silently fork into stale per-plugin snapshots for config, logger, or event wiring. Shared state should stay coherent.

### 4. Registration Must Match Reality

Prepared, loaded, started, and stopped plugins must not be mixed together in one ambiguous registration state. Manager indexes should always reflect real lifecycle state.

### 5. Stability Over Feature Breadth

New core behavior should be accepted only if it improves:

- lifecycle determinism
- cleanup correctness
- concurrency safety
- resource ownership clarity
- observability

Feature growth that expands runtime mutability should be rejected by default.

## Current Decisions Already Applied

The following direction has already started landing in code:

- Core plugin manager cleanup now removes stale plugin indexes more aggressively after failed startup and unload paths.
- `SetGlobalConfig()` no longer acts like a runtime plugin reconfiguration orchestrator. If plugins are already present, the change is rejected and restart is required.
- `RestartRequirementReport` was added as the core-facing restart-based replacement for `ConfigReloadPlan`, while the older plan object remains as a compatibility view.
- `BasePlugin` no longer treats runtime config reload, config rollback, or live plugin upgrade as default supported behavior.
- `RuntimeReport()` was added to expose current plugin and plan state for inspection.
- Unload paths were tightened so not-yet-started plugins are not force-stopped as if they were active.

These changes are part of the refocus, not incidental cleanups.

## Remaining Work Plan

### Phase 1: Stabilize Plugin Manager State

Goal: manager state always matches actual lifecycle state.

Tasks:

- Audit all paths that mutate `pluginList`, `pluginInstances`, and `pluginIDs`
- Separate "prepared" from "started" semantics more clearly
- Ensure rollback and unload cannot leave ghost registrations
- Ensure runtime reports and manager queries expose deterministic state

Success criteria:

- failed startup can be retried without stale manager state
- unload leaves no stale plugin identity records
- tests cover partial startup, rollback, subset unload, and repeated reload attempts

Status:

- completed

Final structural outcome:

- manager now separates prepared inventory from lifecycle-managed runtime state
- runtime-facing queries no longer treat prepared-only plugins as loaded/runtime-visible objects
- retries and subset loads operate on deterministic staging semantics instead of ambiguous shared registration

Current implementation progress:

- manager now keeps separate prepared inventory and lifecycle-managed registries
- runtime-facing queries such as `Plugins(...)`, `ListPluginNames(...)`, `RuntimeReport()`, and config checks only look at managed plugins
- plugins are migrated out of prepared staging once they successfully enter managed lifecycle
- subset load cleans up non-target staged plugins instead of leaving long-lived prepared residue
- repeated load attempts now no-op for already managed plugins instead of duplicating runtime state

Recent verification:

- added regression coverage for prepared-only invisibility until managed registration
- added regression coverage for repeated `LoadPlugins()` calls after a successful load
- added regression coverage for `LoadPluginsByName()` cleaning unused staged plugins
- added regression coverage for retrying `LoadPlugins()` after a startup failure and rollback

### Phase 2: Fix UnifiedRuntime Shared-State Semantics

Goal: plugins use one coherent runtime view.

Tasks:

- remove stale snapshot behavior from `WithPluginContext()`
- ensure config, logger, and event adapter updates are visible consistently
- keep context ownership and permission checks without duplicating runtime state
- verify resource ownership enforcement still holds after the refactor

Success criteria:

- runtime mutations are visible consistently across plugin-scoped views
- no plugin can forge ownership
- per-plugin context remains an access-control view, not a forked runtime copy

Status:

- completed

### Phase 3: Remove Dynamic Config Orchestration From Core Path

Goal: core no longer implies live config rollout inside a running process.

Tasks:

- simplify or remove config-reload-specific protocol concepts from core docs and code
- continue moving callers from `ConfigReloadPlan` to `RestartRequirementReport`
- update tests and docs to stop assuming hot-reload as a preferred path

Success criteria:

- core docs consistently describe restart-based application of config changes
- no base implementation claims in-process plugin config rollout as a default capability

Status:

- in progress

Open compatibility surfaces still to simplify:

- `PluginProtocol` still carries `ConfigHotReload`, `ConfigValidation`, and `ConfigRollback`
- `ConfigReloadPlan` still exists as a compatibility report even though core now has `RestartRequirementReport`
- `LynxApp` still contains compatibility inspection paths for configurable and rollback-capable plugins

### Phase 4: Remove Upgrade Semantics From Default Core Model

Goal: live upgrade is not presented as a core lifecycle feature.

Tasks:

- reduce or remove default upgrade state transitions from base abstractions
- reduce upgrade/rollback events from the default lifecycle vocabulary
- move any remaining upgrade helpers behind optional extension interfaces or compatibility layers

Success criteria:

- core lifecycle documentation no longer centers upgrade/rollback
- base plugin behavior does not advertise live upgrade support

Status:

- in progress

Open compatibility surfaces still to simplify:

- `PluginStatus` still includes `upgrading` and `rollback`
- `UpgradeCapability` and upgrade interfaces still exist in the public plugin API
- plugin event constants still exist as compatibility vocabulary even though docs and base behavior now demote them

### Phase 5: Tighten Global vs Instance State

Goal: avoid hidden coupling between singleton compatibility and real app instances.

Tasks:

- audit global event bus and listener fallbacks
- reduce reliance on global mutable providers
- make shutdown order explicit and instance-scoped where possible

Success criteria:

- tests are less order-sensitive
- multiple app instances are easier to reason about
- close/shutdown has fewer hidden side effects

Status:

- in progress

Recent progress:

- added explicit compatibility-provider clearing for global event-bus and listener-manager access
- `ClearDefaultApp()` now detaches app-owned global event providers instead of leaving nil-returning closures behind
- added regression coverage proving global event/listener lookups stop resolving to the cleared default app
- `boot` no longer parses command-line flags at import time; it only registers `-conf` and resolves the path during explicit bootstrap loading
- added regression coverage proving bootstrap config can still load from the registered `-conf` value without eager `flag.Parse()`
- operation entrypoints now guard against nil-manager regressions even after lifecycle-operation serialization was added
- `boot.Application` can now create a standalone app and optionally publish the default singleton explicitly instead of always going through `NewApp()`
- explicit helpers such as `GetTypedPluginFromApp(...)` now exist so new code can avoid the global default app path

Current engineering assessment:

- approximate completion against the current core-refocus target: `72%`
- strongest areas today: core runtime direction, plugin-manager state separation, lifecycle-path refactoring feasibility
- largest remaining gaps: global-vs-instance state cleanup, compatibility-surface reduction, boot thinning, event-system slimming, and finishing unload-path helper unification

### Phase 6: Increase Failure-Oriented Verification

Goal: prove the framework is stable under bad conditions.

Tasks:

- add targeted tests for lifecycle failure injection
- add tests for concurrent load/unload edge cases
- add tests for resource cleanup under timeout paths
- investigate and fix existing flaky or timing-sensitive recovery tests

Success criteria:

- lifecycle and runtime tests focus on failure modes, not only happy paths
- noisy unload behavior and shutdown races are covered by regression tests

Status:

- partially started

Recent verification progress:

- plugin manager state cleanup paths are covered by targeted tests
- `UnifiedRuntime` shared-state refactor passes plugin package tests
- recovery manager concurrent error-stat access was stabilized by restoring backward-compatible stats output
- root recovery tests for concurrency and goroutine cleanup now pass
- plugin manager lifecycle operations now serialize load/unload/stop entrypoints to avoid concurrent duplicate startup side effects
- added regression coverage proving concurrent `LoadPlugins()` calls only start a plugin once
- full `go test ./... -count=1` passes on the current tree

Known unrelated test debt still present outside the current plugin-core refocus:

- `log` package has time- and filesystem-sensitive test failures around rotation and total-size accounting
- `pkg/strx` currently has a failing string truncation assertion

## Immediate Next Steps

The recommended next implementation order is:

1. Finish Step 1 documentation cleanup so the codebase consistently describes core, shell, and compatibility layers.
2. Split prepared-plugin staging from active manager registration.
3. Continue simplifying config-reload concepts in core types and docs.
4. Clean up remaining upgrade-centric state and event semantics from the default core path.
5. Tighten global-vs-instance behavior and reduce hidden singleton coupling.
6. Return to unrelated flaky test debt after plugin-core boundaries are stable.

## Validation Snapshot

Verified with the local Go toolchain in this workspace:

- `go test . -run 'TestSetGlobalConfig|TestRuntimeReport'`
- `go test . -run 'TestErrorRecoveryManager_(ConcurrentRecordError|ConcurrentRecovery|GoroutineLeak|StopMonitorGoroutineExit|SemaphoreTimeoutGoroutineCleanup)'`
- `go test ./plugins/...`

Current non-core failures observed during broader test runs:

- `github.com/go-lynx/lynx/log`
- `github.com/go-lynx/lynx/pkg/strx`

## How To Evaluate Future Changes

Before adding any new core capability, ask:

1. Does this improve plugin orchestration or framework stability?
2. Could Kubernetes, process restart, or plugin-local logic solve this better?
3. Does this increase runtime mutability or hidden state transitions?
4. Will this make failures easier or harder to reason about?

If the answer trends toward more hidden runtime mutation, the change should stay out of Lynx core.
