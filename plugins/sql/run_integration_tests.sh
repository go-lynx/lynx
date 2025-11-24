#!/bin/bash

# 运行数据库插件集成测试

set -e

echo "=========================================="
echo "运行数据库插件集成测试"
echo "=========================================="
echo ""

# 检查Docker容器状态
echo "1. 检查Docker容器状态..."
docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "mysql|postgres" || echo "未找到数据库容器"
echo ""

# 测试MySQL连接
echo "2. 测试MySQL连接..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx
go test -tags=integration -v ./plugins/sql/mysql -run TestMySQLIntegration 2>&1
echo ""

# 测试PostgreSQL连接
echo "3. 测试PostgreSQL连接..."
go test -tags=integration -v ./plugins/sql/pgsql -run TestPostgreSQLIntegration 2>&1
echo ""

# 运行所有MySQL集成测试
echo "4. 运行所有MySQL集成测试..."
go test -tags=integration -v ./plugins/sql/mysql/... 2>&1
echo ""

# 运行所有PostgreSQL集成测试
echo "5. 运行所有PostgreSQL集成测试..."
go test -tags=integration -v ./plugins/sql/pgsql/... 2>&1
echo ""

echo "=========================================="
echo "所有集成测试完成"
echo "=========================================="

