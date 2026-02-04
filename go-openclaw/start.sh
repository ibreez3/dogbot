#!/bin/bash

# Go-OpenClaw 快速启动脚本

echo "==================================="
echo "  Go-OpenClaw 快速启动"
echo "==================================="
echo ""

# 检查是否已编译
if [ ! -f "bin/gateway" ]; then
    echo "📦 首次运行，正在编译..."
    make build
    echo ""
fi

# 启动 Gateway
echo "🚀 启动 Gateway..."
echo ""

./bin/gateway
