#!/bin/bash

# 用法: ./tag-version.sh polaris v2.0.0
# 最终生成: git tag polaris/v2.0.0

set -e

if [ $# -ne 2 ]; then
  echo "❌ 用法: $0 <模块名> <版本号>"
  echo "例如: $0 polaris v2.0.0"
  exit 1
fi

MODULE=$1       # 模块名，例如 polaris
VERSION=$2      # 版本号，例如 v2.0.0
TAG_NAME="$MODULE/$VERSION"

MODULE_PATH="../plugins/$MODULE/${VERSION%%.*}"  # 截取 v2 前缀 -> plugins/polaris/v2

# 检查模块路径是否存在
if [ ! -f "$MODULE_PATH/go.mod" ]; then
  echo "❌ 模块路径 $MODULE_PATH/go.mod 不存在，请确认模块结构正确。"
  exit 1
fi

# 确认 go.mod 模块名匹配
EXPECTED_MODULE="github.com/go-lynx/plugins/$MODULE/${VERSION%%.*}"
ACTUAL_MODULE=$(grep "^module " "$MODULE_PATH/go.mod" | awk '{print $2}')

if [ "$EXPECTED_MODULE" != "$ACTUAL_MODULE" ]; then
  echo "❌ go.mod 中模块名不匹配："
  echo "    预期: $EXPECTED_MODULE"
  echo "    实际: $ACTUAL_MODULE"
  exit 1
fi

# 创建 tag
git tag "$TAG_NAME"
echo "✅ 创建 tag: $TAG_NAME"

# 推送
git push origin "$TAG_NAME"
echo "🚀 已推送到远程: $TAG_NAME"
