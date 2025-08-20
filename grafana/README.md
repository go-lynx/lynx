# 🎨 Lynx Grafana Dashboards

## Overview

This is a set of modern Grafana dashboards specifically designed for the Lynx microservices framework, featuring stunning visual effects and intuitive data presentation.

## 🚀 Key Features

### ✨ Modern Design
- **Dark Theme**: Dark background to reduce eye strain
- **Gradient Colors**: Gradient color schemes to enhance visual appeal
- **Emoji Icons**: Intuitive emoji icons for quick panel type recognition
- **Responsive Layout**: Adapts to different screen sizes

### 📊 Rich Chart Types
- **Stat Cards**: KPI metrics with gradient dashboards
- **Time Series**: Smooth curves with multiple display modes
- **Heatmaps**: Performance distribution visualization
- **Bar Charts**: Error rates and categorical statistics

### 🎯 Smart Thresholds
- **Dynamic Thresholds**: Reasonable alert thresholds based on business scenarios
- **Color Coding**: Green (normal) → Yellow (warning) → Red (abnormal)
- **Unit Optimization**: Automatic selection of appropriate unit display

## 📁 Dashboard List

### 1. 🌟 System Overview Dashboard
**File**: [overview.json](file:///Users/claire/GolandProjects/lynx/lynx/grafana/overview.json)
**Function**: System-wide overview
- 🚀 Total service count statistics
- ✅ Healthy service monitoring
- ❌ Failed service alerts
- 📊 System availability
- 📈 HTTP request rate
- ⚡ System latency monitoring
- ❌ Error rate trends
- 🔥 Performance heatmap

### 2. 🚀 HTTP Service Dashboard
**File**: [http/dashboard.json](file:///Users/claire/GolandProjects/lynx/lynx/grafana/http/dashboard.json)
**Function**: HTTP service monitoring
- 📊 Request rate statistics
- ⚡ Response time monitoring
- ❌ Error rate analysis
- 🔄 Active connections
- 📈 Request method distribution
- 🔥 Error path analysis
- ⚡ Latency percentiles
- 📦 Response size distribution
- 🔄 In-flight requests
- 📥 Request size distribution
- 🔥 Request heatmap

### 3. 🔥 Redis Service Dashboard
**File**: [redis/dashboard.json](file:///Users/claire/GolandProjects/lynx/lynx/grafana/redis/dashboard.json)
**Function**: Redis cache monitoring
- 🚀 Client startup statistics
- ❌ Startup failure monitoring
- ⚡ PING latency
- 🔗 Connection pool status
- 📊 Connection pool performance
- 🎯 Pool hit rate analysis
- 🔥 Command error analysis
- 🔥 Redis performance heatmap

### 4. 📊 Kafka Service Dashboard
**File**: [kafka/dashboard.json](file:///Users/claire/GolandProjects/lynx/lynx/grafana/kafka/dashboard.json)
**Function**: Kafka message queue monitoring
- 📤 Message production rate
- 📥 Message consumption rate
- ❌ Producer errors
- ⚡ Producer latency
- 📈 Topic production distribution
- 📉 Topic consumption distribution
- ⚡ Latency percentiles
- 🔥 Error type analysis
- 🔥 Kafka performance heatmap

## 🎨 Design Features

### Color Scheme
- **Primary Tone**: Dark background (#1e1e1e)
- **Success Color**: Green gradient
- **Warning Color**: Yellow gradient
- **Error Color**: Red gradient
- **Info Color**: Blue gradient

### Layout Design
- **Grid System**: 24-column responsive grid
- **Card Layout**: Unified card design
- **Spacing Standards**: Consistent margins and padding
- **Hierarchy**: Clear information hierarchy

### Interactive Experience
- **Hover Effects**: Rich hover tooltips
- **Zoom Functionality**: Time range zoom support
- **Refresh Intervals**: Multiple auto-refresh options
- **Template Variables**: Flexible data source selection

## 🔧 Configuration Guide

### Data Source Configuration
```json
{
  "name": "DS_PROM",
  "type": "datasource",
  "query": "prometheus"
}
```


### Template Variables
- `rate_interval`: Rate calculation interval (1m, 5m, 15m)
- `job`: Service job name
- [instance](file:///Users/claire/GolandProjects/lynx/lynx/plugins/base.go#L37-L37): Service instance
- `topic`: Kafka topic (Kafka dashboard only)

### Time Configuration
- **Default Time Range**: Last 1 hour
- **Refresh Intervals**: 5s, 10s, 30s, 1m, 5m, 15m, 30m, 1h, 2h, 1d

## 📈 Metrics Guide

### HTTP Metrics
- `lynx_http_requests_total`: Total requests
- `lynx_http_errors_total`: Error requests
- `lynx_http_request_duration_seconds`: Request latency
- `lynx_http_response_size_bytes`: Response size
- `lynx_http_request_size_bytes`: Request size
- `lynx_http_inflight_requests`: In-flight requests

### Redis Metrics
- `lynx_redis_client_startup_total`: Client startups
- `lynx_redis_client_startup_failed_total`: Startup failures
- `lynx_redis_client_ping_latency_seconds`: PING latency
- `lynx_redis_client_pool_*`: Connection pool related metrics
- `lynx_redis_client_cmd_errors_total`: Command errors

### Kafka Metrics
- `lynx_kafka_messages_produced_total`: Produced messages
- `lynx_kafka_messages_consumed_total`: Consumed messages
- `lynx_kafka_producer_errors_total`: Producer errors
- `lynx_kafka_producer_latency_seconds`: Producer latency

## 🚀 Quick Start

1. **Import Dashboard**:
   - Click "Import" in Grafana
   - Select the corresponding JSON file
   - Configure data source

2. **Configure Data Source**:
   - Ensure Prometheus data source is configured
   - Verify metric data availability

3. **Customize Configuration**:
   - Adjust time range
   - Set refresh interval
   - Configure alert rules

## 🎯 Best Practices

### Monitoring Strategy
- **Real-time Monitoring**: Use 5s refresh interval for real-time monitoring
- **Trend Analysis**: Use 1h refresh interval for trend analysis
- **Capacity Planning**: Use 24h time range for capacity planning

### Alert Configuration
- **Response Time**: P95 > 500ms alert
- **Error Rate**: > 1% alert
- **Availability**: < 99% alert

### Performance Optimization
- **Query Optimization**: Use appropriate rate_interval
- **Cache Strategy**: Reasonably set query cache
- **Resource Monitoring**: Monitor Grafana's own resource usage

## 🔮 Future Plans

- [ ] Add dashboards for more service types
- [ ] Support custom themes
- [ ] Add more chart types
- [ ] Optimize mobile experience
- [ ] Add more alert templates

---

*These dashboards are designed based on best practices of modern monitoring systems, aiming to provide an intuitive, beautiful, and practical monitoring experience.*