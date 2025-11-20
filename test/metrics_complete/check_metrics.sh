#!/bin/bash

echo "========================================="
echo "🔍 Lynx Framework Metrics Verification"
echo "========================================="

# Color definitions
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Service endpoints
METRICS_URL="http://localhost:8080/metrics"
PROMETHEUS_URL="http://localhost:9091"
GRAFANA_URL="http://localhost:3000"

echo -e "\n${YELLOW}📊 Service Endpoint Status:${NC}"
echo "----------------------------------------"

# Check application metrics endpoint
if curl -s -o /dev/null -w "%{http_code}" $METRICS_URL | grep -q "200"; then
    echo -e "✅ Application metrics endpoint: ${GREEN}OK${NC} ($METRICS_URL)"
else
    echo -e "❌ Application metrics endpoint: ${RED}FAILED${NC}"
fi

# Check Prometheus
if curl -s -o /dev/null -w "%{http_code}" $PROMETHEUS_URL | grep -q "200"; then
    echo -e "✅ Prometheus: ${GREEN}OK${NC} ($PROMETHEUS_URL)"
else
    echo -e "❌ Prometheus: ${RED}FAILED${NC}"
fi

# Check Grafana
if curl -s -o /dev/null -w "%{http_code}" $GRAFANA_URL | grep -q "200"; then
    echo -e "✅ Grafana: ${GREEN}OK${NC} ($GRAFANA_URL)"
    echo -e "   Username: admin / Password: lynx123456"
else
    echo -e "❌ Grafana: ${RED}FAILED${NC}"
fi

echo -e "\n${YELLOW}📈 Collected Metrics Statistics:${NC}"
echo "----------------------------------------"

# Get metrics statistics from application endpoint
echo "Metrics types collected from application:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_" | cut -d'{' -f1 | sort | uniq | head -10

echo -e "\n${YELLOW}🔢 Plugin Operation Counts:${NC}"
echo "----------------------------------------"

# Get operation counts by type
echo "Redis operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_redis_operations_total" | tail -1

echo -e "\nKafka operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_kafka_messages_total" | tail -2

echo -e "\nRabbitMQ operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_rabbitmq_messages_total" | tail -2

echo -e "\nMySQL operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_mysql_queries_total" | tail -2

echo -e "\nPostgreSQL operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_postgres_queries_total" | tail -2

echo -e "\nMongoDB operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_mongodb_operations_total" | tail -2

echo -e "\nElasticsearch operation statistics:"
curl -s $METRICS_URL 2>/dev/null | grep "^lynx_elasticsearch_operations_total" | tail -2

echo -e "\n${YELLOW}📊 Grafana Access Address:${NC}"
echo "----------------------------------------"
echo "Browser access: http://localhost:3000"
echo "Username: admin"
echo "Password: lynx123456"
echo ""
echo "Dashboard name: Lynx Complete Monitoring Dashboard"

echo -e "\n${GREEN}✨ Monitoring system verification completed!${NC}"
echo "========================================="