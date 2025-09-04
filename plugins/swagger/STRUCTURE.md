# Swagger Plugin Directory Structure Description

## Directory Structure
```
plugins/swagger/
├── conf/                   # Configuration related files
│   ├── swagger.proto      # Protobuf configuration definition
│   └── swagger.pb.go      # Auto-generated Protobuf Go code
│
├── docs/                   # Documentation output directory
│   ├── README.md          # Documentation directory description
│   └── swagger.json       # Generated Swagger JSON documentation example
│
├── ui/                     # Swagger UI static resources
│   ├── index.html         # Swagger UI HTML template
│   └── embed.go           # Embedded resource handling
│
├── example/                # Complete examples
│   └── full_example.go    # Complete API example code
│
├── test/                   # Test files
│   ├── swagger_test.go    # Unit tests
│   └── integration_test.go # Integration tests
│
├── parser.go              # Annotation parser core implementation
├── plug.go                # Plugin registration and initialization
├── swagger.go             # Plugin main logic implementation
├── ui.go                  # Swagger UI server implementation
├── README.md              # Plugin usage documentation
├── SUMMARY.md             # Implementation summary documentation
├── STRUCTURE.md           # Directory structure description (this file)
└── run_test.sh            # Test script
```

## Directory Function Descriptions

### 1. `conf/` - Configuration Directory
- **swagger.proto**: Defines the plugin configuration structure using Protocol Buffers format
- **swagger.pb.go**: Go code automatically generated from proto files via `protoc` command

### 2. `docs/` - Documentation Directory
- Used to store generated Swagger API documentation
- Default output `swagger.json` file
- Configurable output path and format

### 3. `ui/` - UI Resources Directory
- **index.html**: Swagger UI HTML template using Go template syntax
- **embed.go**: Uses Go 1.16+ `//go:embed` feature to embed static resources
- Provides Swagger UI web interface service

### 4. `example/` - Examples Directory
- Contains complete usage examples
- Demonstrates various annotation usage methods
- Provides runnable demonstration code

### 5. `test/` - Test Directory
- Unit tests and integration tests
- Verifies core functionality like annotation parsing, documentation generation, etc.

## Core File Descriptions

### parser.go
- Implements AST parsing functionality
- Extracts and parses Swagger annotations
- Builds OpenAPI specification data structures

### swagger.go
- Plugin main logic implementation
- Implements Plugin interface
- Manages documentation generation and update processes
- Handles file monitoring

### ui.go
- Swagger UI HTTP server
- Handles web requests
- Provides API documentation interface

### plug.go
- Plugin registration entry point
- Uses `init()` function to automatically register with the framework

## Usage Flow

1. **Import Plugin**
   ```go
   import _ "github.com/go-lynx/lynx/plugins/swagger"
   ```

2. **Configuration File** (config.yaml)
   ```yaml
   lynx:
     swagger:
       enabled: true
       gen:
         scan_dirs: ["./controllers"]
         output_path: "./docs/swagger.json"
       ui:
         enabled: true
         port: 8080
         path: "/swagger"
   ```

3. **Add Annotations**
   Add Swagger annotations in code

4. **Access Documentation**
   After starting the application, visit `http://localhost:8080/swagger`

## Dependencies

- `github.com/go-openapi/spec`: OpenAPI specification data structures
- `github.com/go-lynx/lynx`: Lynx framework core
- CDN Resources: Swagger UI frontend resources (loaded via CDN)
