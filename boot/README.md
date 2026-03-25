# Boot Package

The `boot` package is an optional startup shell around Lynx core.

It is responsible for process-facing concerns such as:

- bootstrap configuration loading
- signal handling
- health checking
- circuit-breaker wiring
- banner/startup UX
- Kratos application startup glue

It should not be treated as the heart of plugin orchestration. Core plugin
management lives in the root `lynx` package and `plugins/UnifiedRuntime`.

## Current Boundary

`boot` is now intentionally less intrusive than before:

- importing `boot` no longer calls `flag.Parse()`
- the package still registers the `-conf` flag for compatibility
- configuration path resolution happens during explicit bootstrap loading
- `LYNX_CONFIG_PATH` remains the default environment-based override

This keeps host applications in control of command-line parsing order while
preserving the old shell-facing entrypoint shape.

## Main Files

- `application.go`
  - `Application` lifecycle shell
  - signal handling
  - startup / shutdown sequencing
- `configuration.go`
  - bootstrap config loading
  - config validation
  - config cleanup wiring
- `config_manager.go`
  - configuration path management

## Backward Compatibility

The following compatibility aliases remain:

- `type Boot = Application`
- `NewLynxApplication(...) == NewApplication(...)`

## Usage

```go
app := boot.NewApplication(wireFunc, plugins...)
if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

If the host process wants to parse flags itself, it can do so before `Run()`.
If it does not, `LoadBootstrapConfig()` still resolves configuration from the
registered `-conf` value or the default config path.
