# Internal Package

The `internal` package contains private implementation details for the Lynx framework. These packages are not intended for external use and may change without notice.

> **Note**: Go's `internal` directory convention prevents these packages from being imported by external code.

## Package Structure

```
internal/
├── banner/         # Startup banner display
└── resource/       # Resource optimization utilities
```

## Sub-packages

### banner/

Handles the display of the Lynx startup banner.

| File | Description |
|------|-------------|
| `banner.go` | Banner display logic |
| `banner.txt` | ASCII art banner template |

**Features:**

- Customizable banner display
- Version and application info
- Can be disabled via configuration

### resource/

Resource management and optimization utilities.

| File | Description |
|------|-------------|
| `cache_optimizer.go` | Cache optimization strategies |

**Features:**

- Memory optimization
- Cache eviction strategies
- Resource pooling

## Usage Guidelines

### For Framework Developers

These packages can be freely used within the Lynx framework:

```go
import (
    "github.com/go-lynx/lynx/internal/banner"
    "github.com/go-lynx/lynx/internal/resource"
)
```

### For External Users

**Do not import these packages directly.** They are internal implementation details and may change without notice. Use the public APIs provided by the root `lynx` package and its sub-packages instead.

## Why Internal?

The `internal` directory is used for:

1. **Implementation details** - Code that supports public APIs but shouldn't be exposed
2. **Breaking change protection** - Allows refactoring without API compatibility concerns
3. **Cleaner public API** - Keeps the public surface area small and focused
4. **Dependency isolation** - External users can't depend on unstable code

## Adding New Internal Packages

When adding new internal packages:

1. **Consider if it should be internal** - Only internalize implementation details
2. **Document the purpose** - Add comments explaining the package's role
3. **Keep dependencies minimal** - Avoid circular dependencies
4. **Test thoroughly** - Internal code still needs testing

## Related Public Packages

| Internal Package | Related Public Package |
|------------------|------------------------|
| `internal/banner` | `boot/` (Application startup) |
| `internal/resource` | `cache/` (Cache abstractions) |

For configuration validation, use the public `lynx/pkg/config` package instead of internal packages.

## License

Apache License 2.0
