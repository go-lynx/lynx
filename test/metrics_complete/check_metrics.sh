#!/bin/bash

echo "========================================="
echo "🔍 Lynx Framework Metrics Verification"
echo "========================================="

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 服务端点
METRICS_URL="http://localhost:8080/metrics"
PROMETHEUS_URL="http://localhost:9091"
GRAFANA_URL="http://localhost:3000"

echo -e "\n${YELLOW}📊 服务端点状态:${NC}"
echo "----------------------------------------"

# 检查应用指标端点
if curl -s -o /dev/null -w "%{http_code}" $METRICS_URL | grep -q "200"; then
    echo -e "✅ 应用指标端点: ${GREEN}正常${NC} ($METRICS_URL)"
else
    echo -e "❌ 应用指标端点: ${RED}异常${NC}"
fi

# 检查Prometheus
if curl -s -o /dev/null -w "%{http_code}" $PROMETHEUS_URL | grep -q "200"; then
    echo -e "✅ Prometheus: ${GREEN}正常${NC} ($PROMETHEUS_URL)"
else
    echo -e "❌ Prometheus: ${RED}异常${NC}"
fi

# 检查Grafana
if curl -s -o /dev/null -w "%{http_code}" $GRAFANA_URL | grep -q "200"; then
    echo -e "✅ Grafana: ${GREEN}正常${NC} ($GRAFANA_URL)"
    echo -e "   用户名: admin / 密码: lynx123456"
else
    echo -e "❌ Grafana: ${RED}异常${NC}"
fi

echo -e "\n${YELLOW}📈 收集到的指标统计:${NC}"
echo "----------------------------------------"

# 从应用端点获取指标统计
echo "从应用收集的指标类型:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_" | cut -d'{' -f1 | sort | uniq | head -10

echo -e "\n${YELLOW}🔢 各插件操作次数:${NC}"
echo "----------------------------------------"

# 获取各类操作计数
echo "Redis 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_redis_operations_total" | tail -1

echo -e "\nKafka 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_kafka_messages_total" | tail -2

echo -e "\nRabbitMQ 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_rabbitmq_messages_total" | tail -2

echo -e "\nMySQL 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_mysql_queries_total" | tail -2

echo -e "\nPostgreSQL 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_postgres_queries_total" | tail -2

echo -e "\nMongoDB 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_mongodb_operations_total" | tail -2

echo -e "\nElasticsearch 操作统计:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_elasticsearch_operations_total" | tail -2

echo -e "\n${YELLOW}📊 Grafana 访问地址:${NC}"
echo "----------------------------------------"
echo "浏览器访问: http://localhost:3000"
echo "用户名: admin"
echo "密码: lynx123456"
echo ""
echo "仪表板名称: Lynx Complete Monitoring Dashboard"

echo -e "\n${GREEN}✨ 监控系统验证完成!${NC}"
echo "========================================="