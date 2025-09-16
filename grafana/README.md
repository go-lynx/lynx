# Lynx Framework Grafana Dashboards

This directory contains Grafana dashboard templates for monitoring the Lynx framework and its plugins.

## Dashboard Overview

### 1. Lynx Framework Overview Dashboard
- **File**: `lynx-overview-dashboard.json`
- **Purpose**: High-level overview of all Lynx framework components
- **Metrics**: Service request rates, message queue throughput, error rates, connection pools, response times

### 2. Plugin-Specific Dashboards

#### Redis Plugin Dashboard
- **File**: `lynx-redis-dashboard.json`
- **Metrics**:
  - Connection pool statistics (hits, misses, timeouts)
  - Connection pool status (total, idle, stale connections)
  - Command latency percentiles
  - Command error rates
  - Cluster state and master status
  - Connected slaves count
  - Ping latency

#### Kafka Plugin Dashboard
- **File**: `lynx-kafka-dashboard.json`
- **Metrics**:
  - Producer/Consumer throughput (messages/sec, bytes/sec)
  - Error rates (producer/consumer errors)
  - Latency metrics
  - Offset management (commits, commit errors)
  - Connection health (errors, reconnections)

#### gRPC Plugin Dashboard
- **File**: `lynx-grpc-dashboard.json`
- **Metrics**:
  - Request rate by method and status
  - Request duration percentiles
  - Active connections
  - Server error rates
  - Server status and start time
  - Request status distribution

#### HTTP Plugin Dashboard
- **File**: `lynx-http-dashboard.json`
- **Metrics**:
  - Request rate by method, path, and status
  - Request/Response duration percentiles
  - Request/Response size percentiles
  - Error rates by type
  - Health check rates
  - Request status distribution

#### SQL Plugin Dashboard
- **File**: `lynx-sql-dashboard.json`
- **Metrics**:
  - Connection pool status (max, open, in-use, idle connections)
  - Connection pool activity (waits, closes, retries)
  - Query/Transaction duration percentiles
  - Error rates by type
  - Health check status
  - Connection activity (attempts, success, failures)
  - Slow query rates

#### Polaris Plugin Dashboard
- **File**: `lynx-polaris-dashboard.json`
- **Metrics**:
  - SDK operations and duration
  - Service discovery operations
  - Service instances count
  - Service registration and heartbeat
  - Configuration operations
  - Routing operations
  - Rate limiting (requests, rejected)
  - Health checks

#### Redis Lock Plugin Dashboard
- **File**: `lynx-redis-lock-dashboard.json`
- **Metrics**:
  - Lock acquire/unlock/renew operations
  - Active locks count
  - Script latency percentiles
  - Skipped renewals
  - Operations by type

## Installation

### 1. Using Grafana Provisioning

1. Copy the dashboard files to your Grafana dashboards directory:
   ```bash
   cp grafana/dashboards/*.json /var/lib/grafana/dashboards/
   cp grafana/provisioning/dashboards/dashboards.yml /etc/grafana/provisioning/dashboards/
   ```

2. Restart Grafana:
   ```bash
   systemctl restart grafana-server
   ```

### 2. Manual Import

1. Open Grafana in your browser
2. Go to "Dashboards" → "Import"
3. Copy and paste the JSON content from each dashboard file
4. Click "Load" and configure the data source

## Configuration

### Data Source Setup

Ensure your Prometheus data source is configured in Grafana:

1. Go to "Configuration" → "Data Sources"
2. Add Prometheus data source
3. Set URL to your Prometheus server (e.g., `http://localhost:9090`)
4. Test the connection

### Dashboard Variables

Some dashboards may include variables for filtering. Configure them as needed:

- **Instance**: Filter by specific service instances
- **Namespace**: Filter by namespace (for Polaris)
- **Service**: Filter by service name

## Metric Naming Convention

All Lynx framework metrics follow this naming convention:

- **Namespace**: `lynx` (for Lynx-specific metrics)
- **Subsystem**: Plugin name (e.g., `redis_client`, `grpc`, `http`)
- **Metric Name**: Descriptive name (e.g., `requests_total`, `duration_seconds`)

### Examples:
- `lynx_redis_client_pool_hits_total`
- `grpc_requests_total`
- `lynx_http_request_duration_seconds`
- `polaris_service_instances_total`

## Alerting

### Recommended Alerts

1. **High Error Rate**
   - Alert when error rate exceeds 5% for any component
   - Expression: `rate(component_errors_total[5m]) / rate(component_requests_total[5m]) > 0.05`

2. **High Response Time**
   - Alert when 95th percentile response time exceeds threshold
   - Expression: `histogram_quantile(0.95, component_duration_seconds_bucket) > 1`

3. **Connection Pool Exhaustion**
   - Alert when connection pool usage exceeds 80%
   - Expression: `component_in_use_connections / component_max_connections > 0.8`

4. **Service Down**
   - Alert when service health check fails
   - Expression: `component_health_status == 0`

### Alert Configuration

1. Go to "Alerting" → "Alert Rules"
2. Create new alert rule
3. Use the expressions above as conditions
4. Set appropriate evaluation interval and notification channels

## Customization

### Adding New Metrics

1. Update the plugin's metrics collection code
2. Add new panels to the relevant dashboard JSON file
3. Use Grafana's panel editor to create visualizations
4. Export the updated dashboard JSON

### Modifying Existing Panels

1. Open the dashboard in Grafana
2. Click "Edit" on any panel
3. Modify queries, visualizations, or settings
4. Save the dashboard
5. Export the updated JSON for version control

## Troubleshooting

### Common Issues

1. **No Data in Panels**
   - Check if Prometheus is scraping metrics from your application
   - Verify metric names match exactly (case-sensitive)
   - Check time range settings

2. **Missing Metrics**
   - Ensure the plugin is enabled and configured
   - Check if metrics collection is enabled in plugin configuration
   - Verify metric names in the plugin source code

3. **Dashboard Not Loading**
   - Check JSON syntax for errors
   - Verify data source configuration
   - Check Grafana logs for errors

### Debugging Steps

1. Check Prometheus targets: `http://prometheus:9090/targets`
2. Query metrics directly: `http://prometheus:9090/graph`
3. Check Grafana logs: `journalctl -u grafana-server -f`
4. Verify dashboard JSON syntax using online JSON validators

## Contributing

When adding new dashboards or modifying existing ones:

1. Follow the established naming conventions
2. Include comprehensive documentation
3. Test with real data before submitting
4. Update this README with new dashboard information
5. Ensure JSON files are properly formatted

## Support

For issues related to:
- **Dashboard functionality**: Check Grafana documentation
- **Metric collection**: Check Lynx framework plugin documentation
- **Prometheus integration**: Check Prometheus documentation

## License

These dashboard templates are provided under the same license as the Lynx framework.