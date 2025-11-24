#!/bin/bash

# 运行所有数据库插件的单元测试

set -e

echo "=========================================="
echo "运行数据库插件单元测试"
echo "=========================================="
echo ""

# 修复MSSQL依赖
echo "1. 修复MSSQL依赖..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx/plugins/sql/mssql
go mod tidy
echo "✅ MSSQL依赖修复完成"
echo ""

# 测试Base包
echo "2. 测试Base包..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx/plugins/sql/base
go test -v . 2>&1 | head -50
echo ""

# 测试MySQL插件
echo "3. 测试MySQL插件..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx/plugins/sql/mysql
go test -v . 2>&1 | head -50
echo ""

# 测试PostgreSQL插件
echo "4. 测试PostgreSQL插件..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx/plugins/sql/pgsql
go test -v . 2>&1 | head -50
echo ""

# 测试MSSQL插件
echo "5. 测试MSSQL插件..."
cd /Users/tanzhuo/goProjects/go-lynx/lynx/plugins/sql/mssql
go test -v . 2>&1 | head -50
echo ""

echo "=========================================="
echo "所有测试完成"
echo "=========================================="

