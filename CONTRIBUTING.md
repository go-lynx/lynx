# Contributing to Go-Lynx

Thank you for your interest in contributing to Go-Lynx! We welcome contributions of all kinds — bug reports, feature requests, documentation improvements, and code changes.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Features](#suggesting-features)
- [Documentation](#documentation)

---

## Code of Conduct

By participating in this project, you agree to uphold our community standards: be respectful, inclusive, and constructive in all interactions.

---

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/lynx.git
   cd lynx
   ```
3. **Add the upstream remote** so you can sync later:
   ```bash
   git remote add upstream https://github.com/go-lynx/lynx.git
   ```

---

## Development Setup

### Prerequisites

- **Go 1.25+** — see [go.dev/dl](https://go.dev/dl/)
- **protoc** — Protocol Buffers compiler (for regenerating `.pb.go` files)
- **golangci-lint** — for linting

Install required toolchain dependencies:

```bash
make init
```

This installs `protoc-gen-go`, `protoc-gen-go-grpc`, `kratos`, `wire`, and other tools into your `$GOPATH/bin`.

### Regenerate Protobuf Code

If you modify any `.proto` file, regenerate the Go code:

```bash
make config
```

---

## Making Changes

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/my-new-feature
   ```
   Use a descriptive prefix: `feat/`, `fix/`, `docs/`, `test/`, `refactor/`, `chore/`.

2. **Write your code** following the project's coding conventions (see [AGENTS.md](AGENTS.md)):
   - Go 1.25+, formatted with `gofmt`/`goimports`
   - Line length ≤ 140 characters
   - Public APIs must have doc comments
   - Prefer explicit dependency injection over global singletons

3. **Run the linter** before committing:
   ```bash
   golangci-lint run
   ```

4. **Commit** using the [Conventional Commits](https://www.conventionalcommits.org/) format:
   ```
   type(scope): short description

   Longer body if needed. Reference issues with "Fixes #123".
   ```
   Examples:
   ```
   feat(plugins): add hot-reload support for configuration changes
   fix(lifecycle): prevent goroutine leak on plugin init timeout
   docs(readme): fix duplicate contributing section
   test(cache): add unit tests for Manager.GetOrCreate
   ```

---

## Testing

Run the full test suite before submitting:

```bash
# Run all tests
go test ./...

# Run with race detector (required for concurrency changes)
go test -race ./...

# Run tests for a specific package
go test ./cache/...
go test ./plugins/...
```

### Writing Tests

- Place `_test.go` files alongside the source file they test.
- Use **table-driven tests** where possible.
- Name tests `TestFeature_Subject` (e.g., `TestCache_GetOrSet_ContextCancelled`).
- Benchmarks use `BenchmarkX`; examples use `ExampleType_Method`.
- Mock external systems via plugin contracts rather than real dependencies.
- Aim for coverage on all new logic, especially concurrency-sensitive paths.

---

## Submitting a Pull Request

1. **Sync with upstream** before opening a PR:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Push** your branch:
   ```bash
   git push origin feat/my-new-feature
   ```

3. **Open a Pull Request** on GitHub against the `main` branch.

4. **Fill in the PR template** — include:
   - **What** was changed and **why**
   - **How** it was tested (commands run, test cases added)
   - Any **breaking changes** or migration notes
   - Screenshots for UI-facing changes

5. **Ensure CI passes** — the PR must pass all lint and test checks before merging.

6. **Address review feedback** promptly; keep your branch up to date with `main`.

---

## Reporting Bugs

Please [open an issue](https://github.com/go-lynx/lynx/issues/new?template=bug-report.md) using the **Bug Report** template. Include:

- Go version and OS/arch
- Lynx version (or commit hash)
- Minimal reproducible example
- Expected vs. actual behaviour
- Relevant logs or stack traces

---

## Suggesting Features

Please [start a discussion](https://github.com/go-lynx/lynx/discussions) or [open an issue](https://github.com/go-lynx/lynx/issues/new?template=feature-request.md) using the **Feature Request** template. Describe:

- The problem you are trying to solve
- Your proposed solution
- Alternatives you have considered
- Any relevant prior art or related projects

---

## Documentation

Documentation improvements are very welcome:

- Fix typos or unclear explanations in `README.md`, `README_zh.md`, or package doc comments.
- Add or improve examples in `doc.go` or `_test.go` example functions.
- Keep `CHANGELOG.md` up to date when making significant changes.

---

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
