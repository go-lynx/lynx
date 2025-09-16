# Seata Distributed Transaction Plugin for Lynx Framework

The Seata Plugin provides comprehensive distributed transaction management for the Lynx framework using Alibaba's Seata (Simple Extensible Autonomous Transaction Architecture). It supports multiple transaction patterns including AT, TCC, SAGA, and XA modes.

## Features

### Core Transaction Support
- **AT Mode**: Automatic compensation transaction mode (recommended)
- **TCC Mode**: Try-Confirm-Cancel transaction mode
- **SAGA Mode**: Long-running business process transaction mode
- **XA Mode**: X/Open XA distributed transaction protocol
- **Mixed Mode**: Support for multiple transaction modes in the same application

### Advanced Features
- **Global Transaction Management**: Centralized transaction coordination
- **Branch Transaction Support**: Local transaction management
- **Compensation Mechanisms**: Automatic rollback and compensation
- **Transaction Recovery**: Automatic transaction recovery after failures
- **Performance Optimization**: High-performance transaction processing
- **Monitoring Integration**: Comprehensive transaction monitoring

### Security & Reliability
- **ACID Compliance**: Full ACID transaction properties
- **Fault Tolerance**: Automatic failure detection and recovery
- **Data Consistency**: Strong consistency guarantees
- **Rollback Support**: Comprehensive rollback mechanisms
- **Timeout Management**: Configurable transaction timeouts

## Architecture

The plugin follows the Lynx framework's layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                    Seata Plugin Layer                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Client    │  │   Manager   │  │   Configuration    │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Transaction Layer                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │    AT       │  │    TCC      │  │        SAGA         │ │
│  │   Mode      │  │   Mode      │  │       Mode          │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Registry Layer                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Nacos     │  │   Eureka    │  │       Consul        │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
lynx:
  seata:
    enabled: true
    application_id: "lynx-seata-client"
    tx_service_group: "my_test_tx_group"
    service:
      vgroup_mapping:
        my_test_tx_group: "default"
      grouplist:
        default: "127.0.0.1:8091"
      enable_degrade: false
      disable_global_transaction: false
    
    config:
      type: "file"
      file:
        name: "file.conf"
      nacos:
        server_addr: "127.0.0.1:8848"
        namespace: ""
        group: "SEATA_GROUP"
        username: ""
        password: ""
    
    registry:
      type: "file"
      file:
        name: "registry.conf"
      nacos:
        application: "seata-server"
        server_addr: "127.0.0.1:8848"
        group: "SEATA_GROUP"
        namespace: ""
        username: ""
        password: ""
```

### Advanced Configuration

```yaml
lynx:
  seata:
    enabled: true
    application_id: "lynx-seata-client"
    tx_service_group: "my_test_tx_group"
    
    # Transaction service configuration
    service:
      vgroup_mapping:
        my_test_tx_group: "default"
      grouplist:
        default: "127.0.0.1:8091"
      enable_degrade: false
      disable_global_transaction: false
      enable_auto_data_source_proxy: true
    
    # Configuration center
    config:
      type: "nacos"
      nacos:
        server_addr: "127.0.0.1:8848"
        namespace: "seata"
        group: "SEATA_GROUP"
        username: "nacos"
        password: "nacos"
        data_id: "seata.properties"
    
    # Registry center
    registry:
      type: "nacos"
      nacos:
        application: "seata-server"
        server_addr: "127.0.0.1:8848"
        group: "SEATA_GROUP"
        namespace: "seata"
        username: "nacos"
        password: "nacos"
    
    # Client configuration
    client:
      rm:
        async_commit_buffer_limit: 10000
        report_retry_count: 5
        table_meta_check_enable: false
        report_success_enable: false
        saga_branch_register_enable: false
        saga_json_parser: "fastjson"
        saga_retry_persist_mode_update: false
        saga_retry_persist_period: 1000
        lock_retry_policy_branch_rollback_on_conflict: true
      tm:
        commit_retry_count: 5
        rollback_retry_count: 5
        default_global_transaction_timeout: 60000
        degrade_check: false
        degrade_check_allow_times: 10
        degrade_check_period: 2000
        interceptor_order: -2147482648
      undo:
        data_validation: true
        log_serialization: "jackson"
        log_table: "undo_log"
        only_care_update_columns: true
      log:
        exception_rate: 100
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "github.com/go-lynx/lynx/plugins/dtx/seata"
    "github.com/seata/seata-go/pkg/client"
)

func main() {
    // Get the Seata client instance
    seataClient := seata.GetSeataClient()
    
    // Start a global transaction
    ctx := context.Background()
    tx, err := seataClient.Begin(ctx, "business-service")
    if err != nil {
        panic(err)
    }
    defer tx.Rollback()
    
    // Execute business logic
    err = executeBusinessLogic(ctx, tx)
    if err != nil {
        return err
    }
    
    // Commit the transaction
    err = tx.Commit()
    if err != nil {
        panic(err)
    }
}
```

### AT Mode Usage

```go
// AT mode - Automatic compensation
func executeBusinessLogic(ctx context.Context, tx *seata.Transaction) error {
    // Business operations that will be automatically compensated
    err := updateInventory(ctx, tx, "product-1", 10)
    if err != nil {
        return err
    }
    
    err = updateOrder(ctx, tx, "order-123", "confirmed")
    if err != nil {
        return err
    }
    
    return nil
}
```

### TCC Mode Usage

```go
// TCC mode - Try-Confirm-Cancel
func executeTCCBusiness(ctx context.Context, tx *seata.Transaction) error {
    // Try phase
    err := tryReserveInventory(ctx, "product-1", 10)
    if err != nil {
        return err
    }
    
    err = tryCreateOrder(ctx, "order-123")
    if err != nil {
        // Cancel phase
        cancelReserveInventory(ctx, "product-1", 10)
        return err
    }
    
    // Confirm phase
    err = confirmReserveInventory(ctx, "product-1", 10)
    if err != nil {
        return err
    }
    
    err = confirmCreateOrder(ctx, "order-123")
    if err != nil {
        return err
    }
    
    return nil
}
```

### SAGA Mode Usage

```go
// SAGA mode - Long-running business process
func executeSAGABusiness(ctx context.Context) error {
    saga := seataClient.NewSaga("order-process")
    
    // Add saga steps
    saga.AddStep("reserve-inventory", 
        func(ctx context.Context) error {
            return reserveInventory(ctx, "product-1", 10)
        },
        func(ctx context.Context) error {
            return releaseInventory(ctx, "product-1", 10)
        })
    
    saga.AddStep("create-order",
        func(ctx context.Context) error {
            return createOrder(ctx, "order-123")
        },
        func(ctx context.Context) error {
            return cancelOrder(ctx, "order-123")
        })
    
    // Execute saga
    return saga.Execute(ctx)
}
```

### XA Mode Usage

```go
// XA mode - X/Open XA protocol
func executeXABusiness(ctx context.Context) error {
    xa := seataClient.NewXA("xa-transaction")
    
    // Add XA resources
    err := xa.AddResource("mysql", "jdbc:mysql://localhost:3306/db1")
    if err != nil {
        return err
    }
    
    err = xa.AddResource("mysql", "jdbc:mysql://localhost:3306/db2")
    if err != nil {
        return err
    }
    
    // Execute XA transaction
    return xa.Execute(ctx, func(ctx context.Context) error {
        // Business logic using XA resources
        return executeBusinessLogic(ctx)
    })
}
```

## API Reference

### SeataClient

The main client interface providing access to all Seata functionality.

#### Methods

- `GetSeataConfig() *conf.Seata` - Returns the current configuration
- `Begin(ctx context.Context, xid string) (*Transaction, error)` - Begins a global transaction
- `NewSaga(name string) *Saga` - Creates a new SAGA transaction
- `NewXA(name string) *XA` - Creates a new XA transaction
- `IsEnabled() bool` - Checks if Seata is enabled
- `GetTransactionManager() *TransactionManager` - Gets the transaction manager

### Transaction

Represents a global transaction instance.

#### Methods

- `Commit() error` - Commits the transaction
- `Rollback() error` - Rollbacks the transaction
- `GetXID() string` - Gets the transaction XID
- `IsActive() bool` - Checks if transaction is active
- `AddBranch(branch *Branch) error` - Adds a branch transaction

### Saga

Represents a SAGA transaction instance.

#### Methods

- `AddStep(name string, action, compensation func(context.Context) error)` - Adds a saga step
- `Execute(ctx context.Context) error` - Executes the saga
- `Compensate(ctx context.Context) error` - Compensates the saga

### XA

Represents an XA transaction instance.

#### Methods

- `AddResource(name, url string) error` - Adds an XA resource
- `Execute(ctx context.Context, business func(context.Context) error) error` - Executes XA transaction

## Transaction Patterns

### 1. AT Mode (Automatic Compensation)

AT mode is the most commonly used mode in Seata. It automatically generates reverse SQL for compensation.

```go
// AT mode automatically handles compensation
func atModeExample(ctx context.Context) error {
    tx, err := seataClient.Begin(ctx, "at-transaction")
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // These operations will be automatically compensated if transaction fails
    err = updateInventory(ctx, "product-1", -10)
    if err != nil {
        return err
    }
    
    err = createOrder(ctx, "order-123")
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### 2. TCC Mode (Try-Confirm-Cancel)

TCC mode requires manual implementation of Try, Confirm, and Cancel methods.

```go
// TCC mode requires manual compensation logic
func tccModeExample(ctx context.Context) error {
    tx, err := seataClient.Begin(ctx, "tcc-transaction")
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Try phase
    err = tryReserveInventory(ctx, "product-1", 10)
    if err != nil {
        return err
    }
    
    // Confirm phase (called on success)
    err = confirmReserveInventory(ctx, "product-1", 10)
    if err != nil {
        // Cancel phase (called on failure)
        cancelReserveInventory(ctx, "product-1", 10)
        return err
    }
    
    return tx.Commit()
}
```

### 3. SAGA Mode (Long-running Process)

SAGA mode is suitable for long-running business processes.

```go
// SAGA mode for long-running processes
func sagaModeExample(ctx context.Context) error {
    saga := seataClient.NewSaga("order-process")
    
    // Define saga steps with compensation
    saga.AddStep("reserve-inventory",
        func(ctx context.Context) error {
            return reserveInventory(ctx, "product-1", 10)
        },
        func(ctx context.Context) error {
            return releaseInventory(ctx, "product-1", 10)
        })
    
    saga.AddStep("create-order",
        func(ctx context.Context) error {
            return createOrder(ctx, "order-123")
        },
        func(ctx context.Context) error {
            return cancelOrder(ctx, "order-123")
        })
    
    saga.AddStep("send-notification",
        func(ctx context.Context) error {
            return sendOrderNotification(ctx, "order-123")
        },
        func(ctx context.Context) error {
            return cancelOrderNotification(ctx, "order-123")
        })
    
    return saga.Execute(ctx)
}
```

## Monitoring and Metrics

### Health Checks

```go
// Check Seata client health
err := seataClient.CheckHealth()
if err != nil {
    log.Printf("Seata health check failed: %v", err)
}

// Get transaction statistics
stats := seataClient.GetTransactionStats()
log.Printf("Active transactions: %d, Committed: %d, Rolled back: %d",
    stats.ActiveTransactions, stats.CommittedTransactions, stats.RolledBackTransactions)
```

### Prometheus Metrics

The plugin exposes comprehensive Prometheus metrics:

#### Transaction Metrics
- `lynx_seata_transactions_total` - Total transactions
- `lynx_seata_transactions_active` - Active transactions
- `lynx_seata_transactions_committed_total` - Committed transactions
- `lynx_seata_transactions_rolled_back_total` - Rolled back transactions
- `lynx_seata_transaction_duration_seconds` - Transaction duration

#### Branch Metrics
- `lynx_seata_branches_total` - Total branch transactions
- `lynx_seata_branches_committed_total` - Committed branches
- `lynx_seata_branches_rolled_back_total` - Rolled back branches

#### Error Metrics
- `lynx_seata_errors_total` - Total errors
- `lynx_seata_timeout_errors_total` - Timeout errors
- `lynx_seata_network_errors_total` - Network errors

## Deployment

### Seata Server Setup

1. **Download Seata Server**
```bash
wget https://github.com/seata/seata/releases/download/v1.8.0/seata-server-1.8.0.zip
unzip seata-server-1.8.0.zip
cd seata
```

2. **Configure Seata Server**
```bash
# Edit conf/application.yml
vim conf/application.yml
```

3. **Start Seata Server**
```bash
./bin/seata-server.sh -p 8091 -h 127.0.0.1 -m file
```

### Docker Deployment

```yaml
version: '3.8'
services:
  seata-server:
    image: seataio/seata-server:1.8.0
    ports:
      - "8091:8091"
    environment:
      - SEATA_PORT=8091
      - STORE_MODE=file
    volumes:
      - ./seata:/opt/seata-server/conf
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: seata-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: seata-server
  template:
    metadata:
      labels:
        app: seata-server
    spec:
      containers:
      - name: seata-server
        image: seataio/seata-server:1.8.0
        ports:
        - containerPort: 8091
        env:
        - name: SEATA_PORT
          value: "8091"
        - name: STORE_MODE
          value: "file"
```

## Troubleshooting

### Common Issues

1. **Transaction Not Starting**
   - Check Seata server connectivity
   - Verify configuration settings
   - Check network connectivity

2. **Compensation Failures**
   - Verify compensation logic
   - Check database connectivity
   - Review transaction logs

3. **Performance Issues**
   - Monitor transaction duration
   - Check resource utilization
   - Review configuration settings

4. **Configuration Errors**
   - Validate configuration files
   - Check registry and config center settings
   - Verify service group mappings

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
lynx:
  seata:
    client:
      log:
        exception_rate: 100
    logging:
      level: "DEBUG"
```

## Best Practices

### Transaction Design
- Keep transactions short and focused
- Avoid long-running transactions
- Design for compensation
- Use appropriate transaction modes

### Performance
- Monitor transaction performance
- Optimize compensation logic
- Use connection pooling
- Implement proper timeout handling

### Monitoring
- Set up comprehensive monitoring
- Monitor transaction success rates
- Track compensation performance
- Alert on transaction failures

### Security
- Secure transaction data
- Implement proper authentication
- Use encrypted connections
- Regular security audits

## Contributing

Contributions are welcome! Please see the main Lynx framework contribution guidelines.

## License

This plugin is part of the Lynx framework and follows the same license terms.

## Support

For support and questions:
- GitHub Issues: [Lynx Framework Issues](https://github.com/go-lynx/lynx/issues)
- Documentation: [Lynx Documentation](https://lynx.go-lynx.com)
- Community: [Lynx Community](https://community.go-lynx.com)
- Seata Documentation: [Seata Official Docs](https://seata.io/)
