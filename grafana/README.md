# 🎨 Lynx Grafana Dashboards

## 概述

这是一套专为 Lynx 微服务框架设计的现代化 Grafana 仪表板，采用炫酷的视觉效果和直观的数据展示方式。

## 🚀 特色功能

### ✨ 现代化设计
- **深色主题**: 采用深色背景，减少视觉疲劳
- **渐变色彩**: 使用渐变色彩方案，提升视觉效果
- **Emoji 图标**: 使用直观的 Emoji 图标，快速识别面板类型
- **响应式布局**: 适配不同屏幕尺寸

### 📊 丰富的图表类型
- **统计卡片**: 带渐变仪表盘的 KPI 指标
- **时间序列图**: 平滑曲线，支持多种显示模式
- **热力图**: 性能分布可视化
- **柱状图**: 错误率和分类统计

### 🎯 智能阈值
- **动态阈值**: 根据业务场景设置合理的告警阈值
- **颜色编码**: 绿色(正常) → 黄色(警告) → 红色(异常)
- **单位优化**: 自动选择合适的单位显示

## 📁 仪表板列表

### 1. 🌟 System Overview Dashboard
**文件**: `overview.json`
**功能**: 系统整体概览
- 🚀 总服务数统计
- ✅ 健康服务监控
- ❌ 失败服务告警
- 📊 系统可用性
- 📈 HTTP 请求率
- ⚡ 系统延迟监控
- ❌ 错误率趋势
- 🔥 性能热力图

### 2. 🚀 HTTP Service Dashboard
**文件**: `http/dashboard.json`
**功能**: HTTP 服务监控
- 📊 请求速率统计
- ⚡ 响应时间监控
- ❌ 错误率分析
- 🔄 活跃连接数
- 📈 请求方法分布
- 🔥 错误路径分析
- ⚡ 延迟百分位数
- 📦 响应大小分布
- 🔄 飞行中请求
- 📥 请求大小分布
- 🔥 请求热力图

### 3. 🔥 Redis Service Dashboard
**文件**: `redis/dashboard.json`
**功能**: Redis 缓存监控
- 🚀 客户端启动统计
- ❌ 启动失败监控
- ⚡ PING 延迟
- 🔗 连接池状态
- 📊 连接池性能
- 🎯 池命中率分析
- 🔥 命令错误分析
- 🔥 Redis 性能热力图

### 4. 📊 Kafka Service Dashboard
**文件**: `kafka/dashboard.json`
**功能**: Kafka 消息队列监控
- 📤 消息生产速率
- 📥 消息消费速率
- ❌ 生产者错误
- ⚡ 生产者延迟
- 📈 主题生产分布
- 📉 主题消费分布
- ⚡ 延迟百分位数
- 🔥 错误类型分析
- 🔥 Kafka 性能热力图

## 🎨 设计特色

### 色彩方案
- **主色调**: 深色背景 (#1e1e1e)
- **成功色**: 绿色渐变
- **警告色**: 黄色渐变
- **错误色**: 红色渐变
- **信息色**: 蓝色渐变

### 布局设计
- **网格系统**: 24 列响应式网格
- **卡片布局**: 统一的卡片设计
- **间距规范**: 一致的边距和内边距
- **层次结构**: 清晰的信息层次

### 交互体验
- **悬停效果**: 丰富的悬停提示
- **缩放功能**: 支持时间范围缩放
- **刷新间隔**: 多种自动刷新选项
- **模板变量**: 灵活的数据源选择

## 🔧 配置说明

### 数据源配置
```json
{
  "name": "DS_PROM",
  "type": "datasource",
  "query": "prometheus"
}
```

### 模板变量
- `rate_interval`: 速率计算间隔 (1m, 5m, 15m)
- `job`: 服务作业名称
- `instance`: 服务实例
- `topic`: Kafka 主题 (仅 Kafka 仪表板)

### 时间配置
- **默认时间范围**: 最近 1 小时
- **刷新间隔**: 5s, 10s, 30s, 1m, 5m, 15m, 30m, 1h, 2h, 1d

## 📈 指标说明

### HTTP 指标
- `lynx_http_requests_total`: 总请求数
- `lynx_http_errors_total`: 错误请求数
- `lynx_http_request_duration_seconds`: 请求延迟
- `lynx_http_response_size_bytes`: 响应大小
- `lynx_http_request_size_bytes`: 请求大小
- `lynx_http_inflight_requests`: 飞行中请求

### Redis 指标
- `lynx_redis_client_startup_total`: 客户端启动数
- `lynx_redis_client_startup_failed_total`: 启动失败数
- `lynx_redis_client_ping_latency_seconds`: PING 延迟
- `lynx_redis_client_pool_*`: 连接池相关指标
- `lynx_redis_client_cmd_errors_total`: 命令错误

### Kafka 指标
- `lynx_kafka_messages_produced_total`: 生产消息数
- `lynx_kafka_messages_consumed_total`: 消费消息数
- `lynx_kafka_producer_errors_total`: 生产者错误
- `lynx_kafka_producer_latency_seconds`: 生产者延迟

## 🚀 快速开始

1. **导入仪表板**:
   - 在 Grafana 中点击 "Import"
   - 选择对应的 JSON 文件
   - 配置数据源

2. **配置数据源**:
   - 确保 Prometheus 数据源已配置
   - 验证指标数据可用

3. **自定义配置**:
   - 调整时间范围
   - 设置刷新间隔
   - 配置告警规则

## 🎯 最佳实践

### 监控策略
- **实时监控**: 使用 5s 刷新间隔进行实时监控
- **趋势分析**: 使用 1h 刷新间隔进行趋势分析
- **容量规划**: 使用 24h 时间范围进行容量规划

### 告警配置
- **响应时间**: P95 > 500ms 告警
- **错误率**: > 1% 告警
- **可用性**: < 99% 告警

### 性能优化
- **查询优化**: 使用适当的 rate_interval
- **缓存策略**: 合理设置查询缓存
- **资源监控**: 监控 Grafana 自身资源使用

## 🔮 未来规划

- [ ] 添加更多服务类型的仪表板
- [ ] 支持自定义主题
- [ ] 增加更多图表类型
- [ ] 优化移动端体验
- [ ] 添加更多告警模板

---

*这些仪表板设计参考了现代监控系统的最佳实践，旨在提供直观、美观且实用的监控体验。*
