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


### TCC Transaction (Recommended: Helper or Global Transaction API)

Using Helper wrapper (Recommended):

```go
func UseTCCWithHelper(ctx context.Context) {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    helper := dtm.NewTransactionHelper(dtmPlugin)
    gid := helper.MustGenGid()

    branches := []dtm.TCCBranch{
        { // Example branch 1
            Try:     "http://localhost:8081/api/inventory/try",
            Confirm: "http://localhost:8081/api/inventory/confirm",
            Cancel:  "http://localhost:8081/api/inventory/cancel",
            Data:    map[string]any{"product_id": "sku-1", "quantity": 2},
        },
    }
    opts := &dtm.TransactionOptions{TimeoutToFail: 60, BranchTimeout: 10}
    if err := helper.ExecuteTCC(ctx, gid, branches, opts); err != nil {
        log.Errorf("ExecuteTCC failed: %v", err)
    }
}
```

Using Global Transaction API (Native usage):

```go
func UseTCCWithNative(ctx context.Context) {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    gid := dtmPlugin.GenerateGid()
    _ = dtmcli.TccGlobalTransaction(dtmPlugin.GetServerURL(), gid, func(tcc *dtmcli.Tcc) (*resty.Response, error) {
        _, err := tcc.CallBranch(
            map[string]any{"product_id": "sku-1", "quantity": 2},
            "http://localhost:8081/api/inventory/try",
            "http://localhost:8081/api/inventory/confirm",
            "http://localhost:8081/api/inventory/cancel",
        )
        return nil, err
    })
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


### XA Transaction (Recommended: Helper or Global Transaction API)

Using Helper wrapper (Recommended):

```go
func UseXAWithHelper(ctx context.Context) {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    helper := dtm.NewTransactionHelper(dtmPlugin)
    gid := helper.MustGenGid()

    branches := []dtm.XABranch{
        { // XABranch.Data uses string (e.g., JSON)
            Action: "http://localhost:8080/api/TransOut",
            Data:   `{"amount": 100}`,
        },
    }
    opts := &dtm.TransactionOptions{TimeoutToFail: 60, BranchTimeout: 10}
    if err := helper.ExecuteXA(ctx, gid, branches, opts); err != nil {
        log.Errorf("ExecuteXA failed: %v", err)
    }
}
```

Using Global Transaction API (Native usage):

```go
func UseXAWithNative(ctx context.Context) {
    dtmPlugin := app.GetPlugin("dtm.server").(*dtm.DTMClient)
    gid := dtmPlugin.GenerateGid()
    _ = dtmcli.XaGlobalTransaction(dtmPlugin.GetServerURL(), gid, func(xa *dtmcli.Xa) (*resty.Response, error) {
        _, err := xa.CallBranch("http://localhost:8080/api/TransOut", `{"amount": 100}`)
        return nil, err
    })
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

## Notes

- `NewTcc()` and `NewXa()` do not provide direct instance return implementations in the current version. Please use the Helper wrapper or DTM's native Global Transaction API as shown above.