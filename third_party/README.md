# Third Party Protobuf Definitions

This directory contains third-party Protocol Buffer definitions required for compiling the Lynx framework's proto files.

## Contents

### google/protobuf/

Contains Google's well-known Protocol Buffer types used by Lynx proto definitions:

| File | Description | Used By |
|------|-------------|---------|
| `duration.proto` | Duration type for time spans | `tls/conf/tls.proto` |

## Why This Directory Exists

The Lynx framework's proto files import Google's well-known types (e.g., `google/protobuf/duration.proto`). These imports require the proto definitions to be available during `protoc` compilation.

This directory provides those definitions so that `make config` can successfully compile all proto files without requiring users to have protobuf installed system-wide or to locate the includes manually.

## Usage

The `Makefile` includes this directory in the protoc command:

```makefile
protoc --proto_path="$$DIR" -I ./third_party -I ./boot -I . --go_out=paths=source_relative:"$$DIR" "$$PROTO_FILE"
```

## Source

The proto files in this directory are sourced from:
- [Google Protocol Buffers](https://github.com/protocolbuffers/protobuf)

## License

The files in `google/protobuf/` are licensed under the BSD 3-Clause License by Google Inc.

