# Code Style Checks Guide

This document describes the GitHub Actions code checking workflows configured in the project.

## 📋 Workflow Overview

The project has configured the following GitHub Actions workflows to ensure code quality:

### 1. CI/CD Pipeline (`.github/workflows/ci.yml`)

The main continuous integration workflow includes the following checks:

- **Code Format Check (format-check)**
  - `gofmt` formatting check
  - `goimports` import formatting check
  - Common issues check (TODO/FIXME comments)

- **Code Linting (lint)**
  - Comprehensive code checking using `golangci-lint`
  - Based on `.golangci.yml` configuration file
  - Timeout: 5 minutes

- **Testing (test)**
  - Unit tests (with race detection)
  - Integration tests
  - Benchmark tests
  - Code coverage reports

- **Security Scan (security)**
  - Gosec security scanning
  - SARIF results upload

- **Build (build)**
  - Build main program
  - Build all plugins
  - Upload build artifacts

- **Release (release)**
  - Automatically create GitHub Release (when tags are pushed)

### 2. Code Style Check (`.github/workflows/code-style.yml`)

Dedicated code style validation workflow:

- ✅ Code formatting check (gofmt)
- ✅ Import formatting check (goimports)
- ✅ Trailing whitespace check
- ✅ File size validation
- ✅ Code complexity check
- ✅ Code smell detection
- ✅ Documentation validation
- ✅ golangci-lint style checks

### 3. Pull Request Check (`.github/workflows/pr-check.yml`)

Dedicated checks for Pull Requests:

- Only checks changed files (improves efficiency)
- Go module validation
- Code format validation
- Linter checks
- Breaking changes detection
- Test coverage analysis
- Automatic PR comments

### 4. Security Scan (`.github/workflows/security.yml`)

Regular security scanning:

- Dependency vulnerability scanning (Nancy, Govulncheck)
- Secret scanning (TruffleHog)
- Code security analysis (Gosec)
- Container security scanning (Trivy)
- License compliance check

## 🔧 Local Development

### Setting Up Git Hooks

Set up pre-commit hooks locally to automatically check code before committing:

```bash
# Run the setup script
bash .github/scripts/setup-git-hooks.sh
```

Or run checks manually:

```bash
# Run pre-commit checks
bash .github/scripts/pre-commit-check.sh
```

### Manual Checks

Before committing code, it's recommended to run the following checks manually:

```bash
# 1. Format code
gofmt -s -w .

# 2. Format imports
goimports -w .

# 3. Run linter
golangci-lint run --timeout=5m

# 4. Run tests
go test -v -race ./...

# 5. Verify Go module
go mod verify
go mod tidy
```

## 📝 Code Style Requirements

Based on the project's `.golangci.yml` configuration, code must meet the following requirements:

### Formatting
- Use `gofmt` standard format
- Use `goimports` to manage imports
- Line length limit: 140 characters

### Code Quality
- Cyclomatic complexity: no more than 15
- Function length: reasonable control
- Avoid duplicate code
- Proper error handling

### Naming Conventions
- Exported functions/types must have documentation comments
- Use clear naming
- Follow Go naming conventions

### Import Conventions
- Local package prefix: `github.com/go-lynx/lynx`
- Group standard library, third-party, and local packages

## 🚫 Excluded Files

The following files and directories are automatically excluded from checks:

- `third_party/` - Third-party code
- `vendor/` - Dependency packages
- `*.pb.go` - Protocol Buffers generated files
- `*.gen.go` - Auto-generated code

## ⚙️ Configuration

### `.golangci.yml`

The main code checking configuration file, including:
- Linter enable list
- Linter-specific settings
- Exclusion rules
- Issue filtering rules

### Go Version

The project uses **Go 1.25**, and all CI workflows use this version.

## 🔍 FAQ

### Q: How to skip pre-commit checks?

```bash
git commit --no-verify
```

**Note:** Skipping checks is not recommended as it may cause CI failures.

### Q: What to do when CI checks fail?

1. Check the specific error messages
2. Run the corresponding check commands locally
3. Fix the issues and resubmit

### Q: How to update `.golangci.yml` configuration?

Edit the `.golangci.yml` file directly. CI will automatically use the new configuration after commit.

## 📚 Related Resources

- [golangci-lint Documentation](https://golangci-lint.run/)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)

## 🤝 Contributing Guidelines

Before submitting a PR, please ensure:

1. ✅ All CI checks pass
2. ✅ Code is formatted
3. ✅ Tests are added/updated
4. ✅ Documentation is updated (if needed)
5. ✅ Code follows project style

Thank you for your contribution! 🎉
