# Lynx Structure Classification And Execution Plan

See also: [Core Refocus Plan](./core_refocus_plan.md)

## Executive Conclusion

After re-reading the codebase, the central conclusion is:

- Lynx already has a viable plugin orchestration core.
- The main architectural risk is not missing functionality.
- The main architectural risk is that core framework code, optional application shell code, and compatibility surfaces still look equally "core".

Today the codebase effectively plays three roles at once:

1. Plugin orchestration framework
2. Application container and startup shell
3. Backward-compatibility platform

That mixture is what makes the design feel heavier than it needs to be.

The path forward should therefore start with classification, not more feature work.

## Structural Classification

### 1. Core

These areas are the real strategic core and should receive most design attention:

- `app.go`
  - instance assembly
  - app-owned runtime wiring
  - plugin manager access
- `manager.go`
  - plugin identity
  - plugin registration surfaces
  - capability inspection
- `prepare.go`
  - config-driven plugin preparation
- `topology.go`
  - dependency validation
  - startup/unload order
- `lifecycle.go`
  - initialize/start/failure-cleanup flow
- `ops.go`
  - load/unload operations
  - stop semantics
- `plugins/unified_runtime.go`
  - runtime shared state
  - resource ownership
  - plugin-scoped access views
- `plugins/plugin.go`
  - public plugin protocol and lifecycle contracts
- `plugins/base.go`
  - default plugin behavior

Core design goals:

- deterministic lifecycle
- explicit ownership
- restart-oriented mutation model
- stable multi-plugin runtime behavior

### 2. Optional Shell

These areas are useful, but should be treated as shell or integration layers rather than the framework heart:

- `boot/application.go`
  - startup orchestration
  - signal handling
  - health checker
  - circuit breaker
  - banner
  - process lifecycle glue
- `controlplane.go`
  - service discovery
  - rate limiting
  - routing
  - external config access
- `subscribe/`
  - gRPC subscription assembly

Shell design goals:

- convenient integration
- low surprise for application developers
- minimal intrusion into the core plugin model

Important boundary:

- shell code may depend on core
- core should not need shell assumptions in order to stay coherent

### 3. Compatibility Surface

These areas exist mainly to preserve historical behavior or migration safety:

- global singleton access in `app.go`
- `events/global.go`
- upgrade and rollback statuses/events in `plugins/` and `events/`
- runtime-config capability flags that no longer drive core behavior
- backward conversion logic in `events/converter.go`
- legacy global convenience entrypoints

Compatibility design goals:

- preserve old integrations
- remain clearly marked as compatibility-only
- stop influencing core design decisions

Important boundary:

- compatibility code should adapt to core
- core should not keep expanding to satisfy compatibility-first semantics

## Current Architectural Assessment

### What is healthy

- dependency sorting is strong and produces useful failure messages
- lifecycle orchestration is meaningful and mostly coherent
- runtime resource ownership has a defensible design
- recent changes already moved the framework closer to restart-oriented behavior

### What is still structurally expensive

1. `LynxApp` is doing too many kinds of work
   - app identity
   - singleton publication
   - event system ownership
   - control plane composition
   - gRPC subscription side effects
   - shutdown sequencing

2. plugin registration states are still too compressed
   - prepared inventory and actively managed runtime state are now separated, but both still live inside one manager type with significant compatibility surface around them
   - the remaining complexity is now mostly compatibility and API shape, not stale runtime-state leakage

3. compatibility layers still shape the public mental model
   - global event fallback
   - upgrade/rollback vocabulary
   - config reload compatibility reporting

4. documentation still overstates platform breadth
   - parts of the docs still read like a general microservice platform instead of a plugin orchestration framework

## Execution Policy

From this point forward, work should follow these rules:

1. Do not add new framework-level dynamic mutation features.
2. Prefer boundary clarification over feature expansion.
3. When in doubt, move behavior outward:
   - from core to shell
   - from shell to plugin-specific code
   - from framework internals to Kubernetes/process restart
4. Preserve compatibility, but label it clearly and keep it out of the happy path.

## Ordered Plan

### Step 1. Make The Docs Match The Real Structure

Goal:

- stop presenting Lynx as a broad "do everything in-process" platform
- explain the difference between core, shell, and compatibility layers

Tasks:

- update architecture docs to reflect the current layered reality
- correct outdated file references and terminology
- explicitly document that Kubernetes/process restart owns rollout concerns

Done when:

- docs describe the same architecture the code is actually moving toward

### Step 2. Tighten Core Registration Semantics

Goal:

- separate prepared plugins from actively managed lifecycle state more clearly

Tasks:

- review `prepare.go`, `manager.go`, `ops.go`, `lifecycle.go`
- decide whether prepared plugins need a distinct registry or explicit state marker
- ensure public queries cannot confuse prepared-only with active lifecycle-managed plugins

Done when:

- manager state is easy to reason about after prepare, partial start, rollback, and unload

Current progress:

- prepared and managed plugin state now use distinct registries
- runtime-facing queries no longer expose prepared-only plugins
- repeated load and subset-load paths have regression coverage
- concurrent load entrypoints are serialized so startup side effects do not run twice

### Step 3. Demote Runtime Config Compatibility Reporting

Goal:

- make restart-oriented behavior the only obvious path in core

Tasks:

- move core callers and reports toward `RestartRequirementReport`
- keep `ConfigReloadPlan` only as a compatibility wrapper
- reduce reliance on `ConfigHotReload`, `ConfigValidation`, `ConfigRollback`
- keep compatibility fields only where still necessary for old plugins

Done when:

- config reporting no longer implies in-process orchestration as a preferred workflow
- the restart-based report is the default vocabulary in core-facing APIs

### Step 4. Demote Upgrade And Rollback Vocabulary

Goal:

- stop presenting upgrade/rollback as part of the default core lifecycle language

Tasks:

- continue downgrading status/event semantics to compatibility-only
- remove upgrade-centric framing from docs and base abstractions where safe

Done when:

- the core lifecycle reads as initialize/start/stop/cleanup, not upgrade choreography

Current progress:

- public docs and plugin README now describe upgrade/rollback hooks as legacy compatibility surfaces
- base plugin behavior already rejects live upgrade operations by default

### Step 5. Separate Shell From Core More Explicitly

Goal:

- reduce accidental coupling from `boot/` and control-plane helpers back into core

Tasks:

- review `boot/application.go` for library-hostile behavior
- reduce import-time side effects
- keep shell conveniences optional and explicit

Done when:

- core remains coherent even if shell code is absent

Current progress:

- `boot` no longer parses command-line flags at import time
- bootstrap config resolution now happens during explicit shell startup

### Step 6. Reduce Global Compatibility Gravity

Goal:

- make instance-owned state the default mental model

Tasks:

- review `events/global.go`
- reduce fallback reliance
- keep singleton access clearly marked as compatibility mode

Done when:

- multi-instance reasoning and shutdown ordering become simpler

Current progress:

- clearing the default app now detaches global event/listener providers explicitly
- global fallbacks still exist, but they are less likely to retain stale app-owned references

## Immediate Next Move

Proceed with Step 1 first.

Reason:

- the docs are still encoding the old, broader platform story
- that makes every later refactor harder to evaluate
- once the documentation matches the architectural decision, code changes become easier to justify and sequence
