# Elasticsearch Plugin for Lynx Framework

The Elasticsearch plugin provides complete Elasticsearch integration support for the Lynx framework, including search, indexing, aggregation, and other functionalities.

## Features

- ✅ **Complete Elasticsearch client support**
- ✅ **Multiple authentication methods**: username/password, API Key, service tokens
- ✅ **TLS/SSL secure connections**
- ✅ **Connection pool management**
- ✅ **Automatic retry mechanisms**
- ✅ **Health checks**
- ✅ **Metrics monitoring**
- ✅ **Hot configuration updates**

## Quick Start

### 1. Configuration

Add Elasticsearch configuration in `config.yml`:

```yaml
lynx:
  elasticsearch:
    addresses:
      - "http://localhost:9200"
      - "http://localhost:9201"
    username: "elastic"
    password: "changeme"
    max_retries: 3
    connect_timeout: "30s"
    enable_metrics: true
    enable_health_check: true
    health_check_interval: "30s"
    compress_request_body: true
    index_prefix: "myapp"
```

### 2. Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/go-lynx/lynx/app/boot"
    "github.com/go-lynx/lynx/plugins/nosql/elasticsearch"
    "github.com/elastic/go-elasticsearch/v8/esapi"
)

func main() {
    // Start Lynx application
    boot.LynxApplication(wireApp).Run()
    
    // Get Elasticsearch client
    client := elasticsearch.GetElasticsearch()
    if client == nil {
        log.Fatal("failed to get elasticsearch client")
    }
    
    // Create index
    createIndex(client)
    
    // Index document
    indexDocument(client)
    
    // Search documents
    searchDocuments(client)
}

func createIndex(client *elasticsearch.Client) {
    ctx := context.Background()
    
    // Create index mapping
    mapping := `{
        "mappings": {
            "properties": {
                "title": {"type": "text"},
                "content": {"type": "text"},
                "created_at": {"type": "date"}
            }
        }
    }`
    
    req := esapi.IndicesCreateRequest{
        Index: "myapp_documents",
        Body:  strings.NewReader(mapping),
    }
    
    res, err := req.Do(ctx, client)
    if err != nil {
        log.Printf("failed to create index: %v", err)
        return
    }
    defer res.Body.Close()
    
    if res.IsError() {
        log.Printf("failed to create index: %s", res.String())
        return
    }
    
    log.Println("index created successfully")
}

func indexDocument(client *elasticsearch.Client) {
    ctx := context.Background()
    
    // Document data
    doc := map[string]interface{}{
        "title":     "Sample Document",
        "content":   "This is the content of a sample document",
        "created_at": time.Now(),
    }
    
    req := esapi.IndexRequest{
        Index:      "myapp_documents",
        DocumentID: "1",
        Body:       strings.NewReader(mustEncodeJSON(doc)),
        Refresh:    "true",
    }
    
    res, err := req.Do(ctx, client)
    if err != nil {
        log.Printf("failed to index document: %v", err)
        return
    }
    defer res.Body.Close()
    
    if res.IsError() {
        log.Printf("failed to index document: %s", res.String())
        return
    }
    
    log.Println("document indexed successfully")
}

func searchDocuments(client *elasticsearch.Client) {
    ctx := context.Background()
    
    // Search query
    query := `{
        "query": {
            "match": {
                "content": "sample"
            }
        }
    }`
    
    req := esapi.SearchRequest{
        Index: []string{"myapp_documents"},
        Body:  strings.NewReader(query),
    }
    
    res, err := req.Do(ctx, client)
    if err != nil {
        log.Printf("failed to search documents: %v", err)
        return
    }
    defer res.Body.Close()
    
    if res.IsError() {
        log.Printf("failed to search documents: %s", res.String())
        return
    }
    
    // Parse search results
    var result map[string]interface{}
    if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
        log.Printf("failed to decode response: %v", err)
        return
    }
    
    hits := result["hits"].(map[string]interface{})
    total := hits["total"].(map[string]interface{})
    log.Printf("found %v documents", total["value"])
}

func mustEncodeJSON(v interface{}) string {
    data, err := json.Marshal(v)
    if err != nil {
        panic(err)
    }
    return string(data)
}
```

## Configuration Options

| Configuration Item | Type | Default Value | Description |
|-------------------|------|---------------|-------------|
| `addresses` | `[]string` | `["http://localhost:9200"]` | List of Elasticsearch server addresses |
| `username` | `string` | `""` | Username |
| `password` | `string` | `""` | Password |
| `api_key` | `string` | `""` | API Key |
| `service_token` | `string` | `""` | Service token |
| `certificate_fingerprint` | `string` | `""` | Certificate fingerprint |
| `compress_request_body` | `bool` | `false` | Whether to compress request body |
| `connect_timeout` | `string` | `"30s"` | Connection timeout |
| `max_retries` | `int` | `3` | Maximum retry count |
| `enable_metrics` | `bool` | `false` | Whether to enable metrics collection |
| `enable_health_check` | `bool` | `false` | Whether to enable health checks |
| `health_check_interval` | `string` | `"30s"` | Health check interval |
| `index_prefix` | `string` | `""` | Index prefix |
| `log_level` | `string` | `"info"` | Log level |

## API Reference

### Get Client

```go
// Get Elasticsearch client
client := elasticsearch.GetElasticsearch()

// Get plugin instance
plugin := elasticsearch.GetElasticsearchPlugin()

// Get connection statistics
stats := plugin.GetConnectionStats()
```

### Plugin Options

```go
// Configure plugin using option pattern
plugin := elasticsearch.NewElasticsearchClient(
    elasticsearch.WithAddresses([]string{"http://localhost:9200"}),
    elasticsearch.WithCredentials("elastic", "changeme"),
    elasticsearch.WithMaxRetries(5),
    elasticsearch.WithMetrics(true),
    elasticsearch.WithHealthCheck(true, 30*time.Second),
)
```

## Monitoring and Metrics

The plugin provides the following monitoring capabilities:

- **Connection status monitoring**
- **Cluster health status**
- **Index statistics**
- **Query performance metrics**
- **Error rate statistics**

## Health Checks

The plugin supports automatic health checks and can monitor:

- Cluster connection status
- Node health status
- Index availability
- Query response time

## Error Handling

The plugin provides comprehensive error handling mechanisms:

- Automatic retry on connection failures
- Network timeout handling
- Authentication failure handling
- Cluster failover

## Best Practices

1. **Connection Pool Configuration**: Adjust connection pool size based on load
2. **Timeout Settings**: Reasonably set connection and query timeout times
3. **Retry Strategy**: Configure appropriate retry count and intervals
4. **Monitoring Alerts**: Enable metrics collection and health checks
5. **Security Configuration**: Use TLS and appropriate authentication methods

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check server address and port
   - Verify network connectivity
   - Confirm firewall settings

2. **Authentication Failures**
   - Check username and password
   - Verify API Key or service token
   - Confirm authentication database configuration

3. **Performance Issues**
   - Adjust connection pool size
   - Optimize query statements
   - Check index configuration

### Log Debugging

Enable debug logging:

```yaml
lynx:
  elasticsearch:
    log_level: "debug"
```

## License

This project is licensed under the Apache License 2.0.
