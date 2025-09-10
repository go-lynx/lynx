# Lynx Logging System Documentation

This logging system is a unified encapsulation based on zerolog + Kratos, with the following capabilities:

- Unified log level filtering (consistent between Kratos and zerolog)
- Support for configurable timezone (timezone string), defaulting to local timezone
- Support for caller source location information, with configurable `caller_skip`
- Support for stack trace collection (configurable threshold, maximum frames, filter prefixes)
- Support for info/debug sampling and per-second rate limiting
- Support for dynamic configuration updates (currently implemented as a 2s polling fallback solution)
- Sampling uses package-level local RNG without modifying the global `math/rand` seed

Core logging code is located at:
- [app/log/logger.go](file:///Users/claire/GolandProjects/lynx/lynx/app/log/logger.go)
- Configuration Proto: [app/log/conf/log.proto](file:///Users/claire/GolandProjects/lynx/lynx/app/log/conf/log.proto)

## Configuration Structure (YAML)

The configuration path key is `lynx.log` (i.e., under `lynx:` in YAML, there is `log:`).

Example: See the "Complete Example" section below.

### Top-level Fields
- `level`: Log level (debug/info/warn/error). Default is `info`.
- `console_output`: Whether to output to console (bool).
- `file_path`: Log file path. When non-empty, file output is enabled.
- `max_size_mb`: Maximum size of a single log file before rotation (MB).
- `max_age_days`: Number of days to retain rotated logs.
- `max_backups`: Number of backup files to retain.
- `compress`: Whether to compress rotated logs.
- `timezone`: Timezone for log timestamps (e.g., `Asia/Shanghai`, `UTC`). Defaults to local timezone if not configured.
- `caller_skip`: Stack depth offset for caller source location, default is 5.

### Stack
`lynx.log.stack`
- `enable`: Whether to enable stack output.
- `skip`: Number of frames to skip when collecting stack (used to eliminate internal logging stack frames).
- `max_frames`: Maximum number of frames to collect.
- `level`: Minimum log level that triggers stack output (debug/info/warn/error/fatal).
- `filter_prefixes`: List of frame prefix filters (package names or file path prefixes).

### Sampling (Sampling and Rate Limiting)
`lynx.log.sampling`
- `enable`: Whether to enable sampling/rate limiting.
- `info_ratio`: Info log sampling ratio [0,1], 0 means discard all, 1 means retain all.
- `debug_ratio`: Debug log sampling ratio [0,1].
- `max_info_per_sec`: Maximum info logs per second (0 means no limit).
- `max_debug_per_sec`: Maximum debug logs per second (0 means no limit).

Note: Sampling and rate limiting currently only apply to `info/debug`; `warn/error` are unaffected.

## Dynamic Configuration Updates

- Prefer to use the Watch mechanism of the configuration source; if Watch is not supported, falls back to polling `lynx.log` every 2 seconds.
- Supported hot-update fields: `level`, `timezone`, `caller_skip`, `stack.*`, `sampling.*`.

## Usage

- Initialization is called by [log.InitLogger(...)](file:///Users/claire/GolandProjects/lynx/lynx/app/log/logger.go#L48-L310) at application startup, no explicit call is needed from business code.
- Quick methods in business code:
  - `log.Debug/Info/Warn/Error/Fatal`
  - With context: [log.InfoCtx(ctx, ...)](file:///Users/claire/GolandProjects/lynx/lynx/app/log/helper.go#L121-L123) etc.
  - Structured: [log.Infow("key", val, ...)](file:///Users/claire/GolandProjects/lynx/lynx/app/log/helper.go#L133-L135)

### Programmatic level update
`log.SetLevel()` now updates zerolog and Kratos filter consistently and rebuilds the logger so that helper methods take effect immediately.

## Complete Example (configs/log-example.yaml)

