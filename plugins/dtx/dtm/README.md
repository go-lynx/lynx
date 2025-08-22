# DTM Distributed Transaction Manager Plugin

## Introduction

The DTM plugin provides distributed transaction management capabilities for the Lynx framework, supporting multiple distributed transaction patterns:
- SAGA: Long transaction solution
- TCC: Try-Confirm-Cancel pattern
- 2-phase message: Reliable message eventual consistency
- XA: Two-phase commit protocol

## Features

- Supports both HTTP and gRPC protocols
- Automatically handles transaction timeouts and retries
- Provides branch barrier functionality to automatically handle idempotency, suspension, and empty compensation issues
- Supports custom request header passthrough
- Flexible timeout configuration

## Configuration Guide

```yaml
lynx:
  dtm:
    enabled: true                          # Whether to enable the plugin
    server_url: "http://localhost:36789/api/dtmsvr"  # DTM HTTP service address
    grpc_server: "localhost:36790"         # DTM gRPC service address (optional)
    timeout: 10                            # Request timeout (seconds)
    retry_interval: 10                     # Retry interval (seconds)
    transaction_timeout: 60                # Global transaction timeout (seconds)
    branch_timeout: 30                     # Branch transaction timeout (seconds)
    pass_through_headers:                  # Request headers that need to be passed through
      - "X-Request-ID"
      - "X-User-ID"
```


## Usage Examples

### SAGA Transaction

```go
import (
    "github.com/go-lynx/lynx/app"
    "github.com/go-lynx/lynx/plugins/dtm/dtm"
)

func UseSaga() {
    // Get DTM plugin instance
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    
    // Generate global transaction ID
    gid := dtmPlugin.GenerateGid()
    
    // Create SAGA transaction
    saga := dtmPlugin.NewSaga(gid)
    
    // Add transaction branches
    saga.Add(
        "http://localhost:8080/api/TransOut",     // Forward operation
        "http://localhost:8080/api/TransOutRevert", // Compensation operation
        map[string]interface{}{"amount": 100},
    )
    saga.Add(
        "http://localhost:8080/api/TransIn",
        "http://localhost:8080/api/TransInRevert",
        map[string]interface{}{"amount": 100},
    )
    
    // Submit transaction
    err := saga.Submit()
    if err != nil {
        log.Errorf("SAGA transaction failed: %v", err)
    }
}
```


### TCC Transaction

```go
func UseTCC() {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    gid := dtmPlugin.GenerateGid()
    
    // Create TCC transaction
    tcc := dtmPlugin.NewTcc(gid)
    
    // Register branch transaction
    err := tcc.CallBranch(
        map[string]interface{}{"amount": 100},
        "http://localhost:8080/api/TransOutTry",
        "http://localhost:8080/api/TransOutConfirm",
        "http://localhost:8080/api/TransOutCancel",
    )
    if err != nil {
        log.Errorf("TCC branch failed: %v", err)
        return
    }
    
    // Submit global transaction
    err = tcc.Submit()
    if err != nil {
        log.Errorf("TCC transaction failed: %v", err)
    }
}
```


### 2-Phase Message

```go
func UseMsg() {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    gid := dtmPlugin.GenerateGid()
    
    // Create message transaction
    msg := dtmPlugin.NewMsg(gid)
    
    // Add transaction steps
    msg.Add(
        "http://localhost:8080/api/TransOut",
        map[string]interface{}{"amount": 100},
    )
    msg.Add(
        "http://localhost:8080/api/TransIn",
        map[string]interface{}{"amount": 100},
    )
    
    // Prepare message
    err := msg.Prepare("http://localhost:8080/api/QueryPrepared")
    if err != nil {
        log.Errorf("Message prepare failed: %v", err)
        return
    }
    
    // Submit message
    err = msg.Submit()
    if err != nil {
        log.Errorf("Message transaction failed: %v", err)
    }
}
```


### XA Transaction

```go
func UseXA() {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    gid := dtmPlugin.GenerateGid()
    
    // Create XA transaction
    xa := dtmPlugin.NewXa(gid)
    
    // Register XA branch
    err := xa.CallBranch(
        "http://localhost:8080/api/TransOut",
        map[string]interface{}{"amount": 100},
    )
    if err != nil {
        log.Errorf("XA branch failed: %v", err)
        return
    }
    
    // Submit XA transaction
    err = xa.Submit()
    if err != nil {
        log.Errorf("XA transaction failed: %v", err)
    }
}
```


## Installing DTM Server

Before using the plugin, you need to install and run the DTM server:

```bash
# Run using Docker
docker run -itd --name dtm -p 36789:36789 -p 36790:36790 yedf/dtm:latest

# Or use binary file
wget https://github.com/dtm-labs/dtm/releases/download/v1.17.0/dtm_1.17.0_linux_amd64.tar.gz
tar -xzvf dtm_1.17.0_linux_amd64.tar.gz
./dtm -c conf.yml
```


## References

- [DTM Official Documentation](https://dtm.pub)
- [DTM GitHub](https://github.com/dtm-labs/dtm)
- [DTM Go SDK](https://github.com/dtm-labs/client)