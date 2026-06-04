# Internal Package

Private implementation details for the Lynx framework. Go's `internal` convention
keeps these packages from being imported by external code; treat them as unstable
and subject to change without notice.

## Sub-packages

### `app/`

Application bootstrap and runtime core: the `LynxApp` instance, plugin manager,
lifecycle (init/start/stop with timeouts, parallel-by-level startup, rollback),
dependency topology, control-plane composition, gRPC subscriptions, and error
recovery. A `compat/` layer and `!v2` files hold deprecated singleton helpers
slated for removal in v2.0.

### `banner/`

Displays the startup banner. Prefers a project-local `configs/banner.txt`, falls
back to the embedded `banner.txt`, and can be disabled via configuration.

### `resource/`

Lightweight cache abstraction used by the public `cache` package wrapper.

## For External Users

Do not import these packages directly. Use the public APIs of the root `lynx`
package and its sub-packages instead.

## License

Apache License 2.0
