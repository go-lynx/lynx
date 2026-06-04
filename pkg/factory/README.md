# Factory Package

The `factory` package creates and manages plugins in the Lynx framework. It separates
plugin *registration* from plugin *creation* behind small interfaces.

## File Structure

### `interfaces.go`
Core interfaces:
- `Registry` — register, unregister, and query plugins
- `Creator` — create plugin instances by name
- `Factory` — composes `Registry` and `Creator`

### `typed_factory.go`
- `TypedFactory` — concurrency-safe registry of plugin creators, with a config-prefix index
- `RegisterTypedPlugin[T]` / `GetTypedPlugin[T]` — generic helpers for type-safe access
- `GlobalTypedFactory()` — lazily initialized global factory

## Design

- **Interface segregation**: registration and creation are separate contracts; `Factory` joins them.
- **Type safety**: generics give callers concrete plugin types without manual assertions.
- **Concurrency safety**: `TypedFactory` guards its maps with an `sync.RWMutex`. First
  registration of a name wins; duplicates are ignored so plugin-init ordering races
  cannot clobber an existing creator.

## Usage

```go
// Get the global type-safe factory
typedFactory := factory.GlobalTypedFactory()

// Register a type-safe plugin
factory.RegisterTypedPlugin(typedFactory, "redis", "cache", func() *redis.Plugin {
    return redis.New()
})

// Get a type-safe plugin instance
redisPlugin, err := factory.GetTypedPlugin[*redis.Plugin](typedFactory, "redis")
```

`TypedFactory` also implements the non-generic `Factory` interface
(`RegisterPlugin`, `CreatePlugin`, `GetPluginRegistry`, `HasPlugin`, `UnregisterPlugin`)
for callers that work with `plugins.Plugin` directly.

## Interface Hierarchy

```
Factory (full capabilities)
├── Registry (registration)
│   ├── RegisterPlugin()
│   ├── UnregisterPlugin()
│   ├── GetPluginRegistry()
│   └── HasPlugin()
└── Creator (creation)
    └── CreatePlugin()
```
