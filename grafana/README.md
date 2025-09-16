# Lynx Framework Grafana Monitoring Dashboards

This directory contains Grafana monitoring dashboards organized by plugin structure. Each plugin has its own `grafana` subdirectory containing the relevant monitoring templates.

## Directory Structure

The monitoring dashboards are organized following the Lynx plugin structure:

```
plugins/
├── service/
│   ├── grpc/grafana/
│   │   └── lynx-grpc-dashboard.json
│   └── http/grafana/
│       └── lynx-http-dashboard.json
├── sql/
│   ├── mysql/grafana/
│   │   └── lynx-mysql-dashboard.json
│   ├── mssql/grafana/
│   │   └── lynx-mssql-dashboard.json
│   └── pgsql/grafana/
│       └── lynx-pgsql-dashboard.json
├── nosql/
│   ├── redis/grafana/
│   │   └── lynx-redis-dashboard.json
│   ├── redis/redislock/grafana/
│   │   └── lynx-redis-lock-dashboard.json
│   ├── mongodb/grafana/
│   │   └── lynx-mongodb-dashboard.json
│   └── elasticsearch/grafana/
│       └── lynx-elasticsearch-dashboard.json
├── mq/
│   ├── kafka/grafana/
│   │   └── lynx-kafka-dashboard.json
│   ├── rabbitmq/grafana/
│   │   └── lynx-rabbitmq-dashboard.json
│   ├── rocketmq/grafana/
│   │   └── lynx-rocketmq-dashboard.json
│   └── pulsar/grafana/
│       └── lynx-pulsar-dashboard.json
├── dtx/
│   ├── dtm/grafana/
│   │   └── lynx-dtm-dashboard.json
│   └── seata/grafana/
│       └── lynx-seata-dashboard.json
├── polaris/grafana/
│   └── lynx-polaris-dashboard.json
└── tracer/grafana/
    └── lynx-tracer-dashboard.json
```

## Available Dashboards

### Service Plugins
- **gRPC Plugin** (`plugins/service/grpc/grafana/lynx-grpc-dashboard.json`)
  - Server request rate and duration
  - Active connections
  - Error rates
  - Server status and uptime

- **HTTP Plugin** (`plugins/service/http/grafana/lynx-http-dashboard.json`)
  - Request rate and duration
  - Response/request size metrics
  - Error rates by status code

### Database Plugins

#### SQL Databases
- **MySQL Plugin** (`plugins/sql/mysql/grafana/lynx-mysql-dashboard.json`)
- **MSSQL Plugin** (`plugins/sql/mssql/grafana/lynx-mssql-dashboard.json`)
- **PostgreSQL Plugin** (`plugins/sql/pgsql/grafana/lynx-pgsql-dashboard.json`)

  Common metrics for all SQL plugins:
  - Connection pool status (total, active, idle connections)
  - Query duration percentiles (50th, 90th, 95th, 99th)
  - Query and transaction rates
  - Connection activity (attempts, success, failures)
  - Health check status

#### NoSQL Databases
- **Redis Plugin** (`plugins/nosql/redis/grafana/lynx-redis-dashboard.json`)
  - Connection pool status
  - Command duration and rate
  - Error rates by type
  - Health status and ping latency

- **Redis Distributed Lock** (`plugins/nosql/redis/redislock/grafana/lynx-redis-lock-dashboard.json`)
  - Lock acquire/unlock/renew activity
  - Success/failure rates
  - Active locks count
  - Skipped renewals

- **MongoDB Plugin** (`plugins/nosql/mongodb/grafana/lynx-mongodb-dashboard.json`)
  - Connection pool status
  - Operation duration and rate
  - Error rates by type

- **Elasticsearch Plugin** (`plugins/nosql/elasticsearch/grafana/lynx-elasticsearch-dashboard.json`)
  - Connection pool status
  - Operation duration and rate
  - Error rates by type

### Message Queue Plugins
- **Kafka Plugin** (`plugins/mq/kafka/grafana/lynx-kafka-dashboard.json`)
- **RabbitMQ Plugin** (`plugins/mq/rabbitmq/grafana/lynx-rabbitmq-dashboard.json`)
- **RocketMQ Plugin** (`plugins/mq/rocketmq/grafana/lynx-rocketmq-dashboard.json`)
- **Pulsar Plugin** (`plugins/mq/pulsar/grafana/lynx-pulsar-dashboard.json`)

  Common metrics for all MQ plugins:
  - Message throughput (produced/consumed)
  - Producer/consumer latency
  - Error rates (producer, consumer, connection)
  - Health status

### Distributed Transaction Plugins
- **DTM Plugin** (`plugins/dtx/dtm/grafana/lynx-dtm-dashboard.json`)
  - Transaction rates by mode (SAGA, TCC, 2PC)
  - Transaction status (committed, rollbacked, timeout)
  - Transaction duration
  - Error rates

- **Seata Plugin** (`plugins/dtx/seata/grafana/lynx-seata-dashboard.json`)
  - Transaction rates by mode (AT, TCC, SAGA, XA)
  - Transaction status (committed, rollbacked, timeout)
  - Transaction duration
  - Error rates

### Service Governance
- **Polaris Plugin** (`plugins/polaris/grafana/lynx-polaris-dashboard.json`)
  - Service discovery and registration
  - SDK operation duration
  - Rate limiting metrics
  - Health check status

### Observability
- **Tracer Plugin** (`plugins/tracer/grafana/lynx-tracer-dashboard.json`)
  - Span creation and completion rates
  - Trace creation and completion rates
  - Span duration percentiles
  - Error rates

## Usage

### Importing Dashboards

1. **Individual Plugin Import**: Navigate to the specific plugin's `grafana` directory and import the JSON file directly into Grafana.

2. **Bulk Import**: Use Grafana's provisioning feature to automatically load all dashboards:
   ```yaml
   # grafana/provisioning/dashboards/dashboards.yml
   apiVersion: 1
   
   providers:
   - name: 'lynx-dashboards'
     type: file
     disableDeletion: false
     updateIntervalSeconds: 10
     options:
       path: /path/to/plugins/*/grafana/*.json
   ```

### Configuration

Each dashboard expects a Prometheus data source named "Prometheus". Make sure to:

1. Configure your Prometheus data source in Grafana
2. Ensure the Lynx framework is exposing metrics on the expected endpoints
3. Verify that the metric names in the dashboards match your actual metric names

### Customization

The dashboards are designed to be easily customizable:

- **Time Ranges**: Default to 1 hour, easily adjustable
- **Refresh Intervals**: Can be configured per dashboard
- **Variables**: Can be added for dynamic filtering
- **Panels**: Can be added, removed, or modified as needed

## Metric Naming Convention

All metrics follow the `lynx_<plugin>_<metric_name>` pattern:

- `lynx_grpc_server_requests_total`
- `lynx_redis_connection_pool_total`
- `lynx_kafka_producer_messages_total`
- `lynx_dtm_transaction_committed_total`

## Troubleshooting

### Common Issues

1. **No Data**: Check that Prometheus is scraping metrics from your Lynx application
2. **Wrong Metric Names**: Verify the metric names match your actual implementation
3. **Missing Panels**: Ensure all required metrics are being collected

### Support

For issues with specific dashboards, check the corresponding plugin's documentation or create an issue in the Lynx repository.

## Contributing

When adding new plugins or modifying existing ones:

1. Create a `grafana` directory in the plugin's root
2. Add a dashboard JSON file following the naming convention
3. Update this README with the new dashboard information
4. Ensure all metrics are properly documented

## License

These dashboards are part of the Lynx framework and follow the same license terms.