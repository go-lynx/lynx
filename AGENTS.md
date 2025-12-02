# Repository Guidelines

## Project Structure & Module Organization

### Root Package (github.com/go-lynx/lynx)
Core framework files with clear naming:
- `doc.go`: Package documentation
- `app.go`: LynxApp core structure, initialization, and main API
- `manager.go`: Plugin manager interfaces and DefaultPluginManager implementation
- `lifecycle.go`: Plugin lifecycle operations (init/start/stop with safety)
- `ops.go`: Plugin loading/unloading operations and resource management
- `topology.go`: Plugin dependency resolution and topological sorting
- `runtime.go`: TypedRuntimePlugin for resource sharing and event handling
- `controlplane.go`: ControlPlane interfaces for service management
- `certificate.go`: CertificateProvider interface for TLS
- `prepare.go`: Plugin preparation and bootstrapping from configuration
- `recovery.go`: Error recovery and circuit breaker mechanisms

### Sub-packages
- `boot/`: Bootstrap helpers and config loading used by `lynx` apps.
- `cmd/`: CLI entrypoints (e.g., `lynx` tool); main packages live here.
- `conf/`: Configuration proto definitions for framework bootstrap.
- `events/`: Event system for inter-plugin communication.
- `log/`: Logging system with zerolog integration.
- `tls/`: TLS certificate management and validation.
- `cache/`: Caching abstractions and implementations.
- `subscribe/`: Service subscription and dependency management.
- `observability/`: Metrics and monitoring utilities.
- `pkg/`: Reusable utility packages (auth, cast, collection, etc.).
- `internal/`: Private implementation details (kratos adapter, banner, etc.).
- `plugins/`: Plugin SDK with base implementations.
- `test/`: Integration and behavioral tests for core framework.
- `third_party/`: Third-party proto definitions (Google well-known types).
- `docs/`, `docker-compose*.yml`: Ops/monitoring assets for local stacks.

## Build, Test, and Development Commands
- `go test ./...`: Run unit and integration tests across modules.
- `go test -race ./...`: Race detector for concurrency-sensitive changes.
- `golangci-lint run`: Lint with repo-configured rules from `.golangci.yml`.
- `make init`: Install required toolchain (`protoc` plugins, `kratos`, `wire`).
- `make config`: Regenerate Go code from all `*.proto` files under `boot/`, `conf/`, `log/conf/`, `tls/conf/` folders.
- `lynx run --watch`: Build and run the current service with hot reload (requires `go install ./cmd/lynx`).

## Coding Style & Naming Conventions
- Go 1.25+, fmt-ed with `gofmt`/`goimports` (local prefix `github.com/go-lynx/lynx`).
- Follow `.golangci.yml`: 140-char line cap, avoid naked returns, check shadowing, keep functions small (gocyclo ?15 where practical).
- Package names: short, lower_snake; files match feature (e.g., `router.go`, `metrics.go`).
- Public APIs need doc comments; keep interfaces lean and purpose-specific.

## Testing Guidelines
- Prefer table-driven tests with `_test.go` colocated to source; larger flows belong in `test/`.
- Name tests `TestFeature_Subject`; benchmarks use `BenchmarkX`; examples `ExampleType_Method`.
- Aim for coverage on new logic and concurrency edges; mock external systems via plugin contracts where possible.

## Commit & Pull Request Guidelines
- Commits: concise imperative subject (?72 chars), scope in parentheses when helpful, no "WIP"; reference issue IDs where relevant.
- Keep diffs focused; include regenerated files only when needed (e.g., after `make config`).
- PRs: describe intent, major design points, testing performed (`go test ./...`, `golangci-lint run`), and mention config/migration impacts; add screenshots for UX-facing changes.

## Security & Configuration Tips
- Store secrets outside the repo; use env vars or external secret stores referenced in config YAML.
- TLS, tracing, and metrics settings live in service configs; avoid hardcoding endpoints or credentials.
- Before pushing, run lint + tests to keep CI green and module tags consistent.
