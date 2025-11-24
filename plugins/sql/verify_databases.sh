#!/bin/bash

# 验证数据库插件是否正常运行
# 这个脚本会测试MySQL和PostgreSQL的连接

echo "=== 验证数据库插件连接 ==="
echo ""

# 测试MySQL连接
echo "测试MySQL连接..."
mysql -h localhost -P 3306 -u lynx -plynx123456 -e "SELECT 1 as test;" lynx_test 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ MySQL连接成功"
else
    echo "❌ MySQL连接失败"
fi

echo ""

# 测试PostgreSQL连接
echo "测试PostgreSQL连接..."
PGPASSWORD=lynx123456 psql -h localhost -p 5432 -U lynx -d lynx_test -c "SELECT 1 as test;" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ PostgreSQL连接成功"
else
    echo "❌ PostgreSQL连接失败"
fi

echo ""
echo "=== 验证完成 ==="

