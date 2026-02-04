#!/bin/bash

# Go-OpenClaw 验证脚本

set -e

echo "==================================="
echo "  Go-OpenClaw 验证测试"
echo "==================================="
echo ""

# 1. 检查 Go 版本
echo "1️⃣  检查 Go 版本..."
go version
echo "   ✅ Go 版本检查通过"
echo ""

# 2. 检查项目结构
echo "2️⃣  检查项目结构..."
if [ -d "cmd/gateway" ] && [ -d "pkg/gateway" ] && [ -d "internal/protocol" ]; then
    echo "   ✅ 项目结构检查通过"
else
    echo "   ❌ 项目结构不完整"
    exit 1
fi
echo ""

# 3. 编译项目
echo "3️⃣  编译项目..."
go build -o bin/gateway cmd/gateway/main.go
echo "   ✅ 编译成功"
echo ""

# 4. 检查二进制文件
echo "4️⃣  检查二进制文件..."
if [ -f "bin/gateway" ]; then
    echo "   ✅ 二进制文件已创建"
    ls -lh bin/gateway
else
    echo "   ❌ 二进制文件不存在"
    exit 1
fi
echo ""

# 5. 停止现有进程
echo "5️⃣  清理现有进程..."
pkill -f "bin/gateway" 2>/dev/null || true
sleep 1
echo "   ✅ 清理完成"
echo ""

# 6. 启动 Gateway
echo "6️⃣  启动 Gateway..."
./bin/gateway &
GATEWAY_PID=$!
echo "   Gateway PID: $GATEWAY_PID"
sleep 2
echo "   ✅ Gateway 启动成功"
echo ""

# 7. 测试健康检查
echo "7️⃣  测试健康检查..."
HEALTH_RESPONSE=$(curl -s http://localhost:18790/health)
if [ "$HEALTH_RESPONSE" = '{"status":"ok"}' ]; then
    echo "   ✅ 健康检查通过"
else
    echo "   ❌ 健康检查失败: $HEALTH_RESPONSE"
    kill $GATEWAY_PID
    exit 1
fi
echo ""

# 8. 清理
echo "8️⃣  清理进程..."
kill $GATEWAY_PID
wait $GATEWAY_PID 2>/dev/null || true
echo "   ✅ 清理完成"
echo ""

echo "==================================="
echo "  ✅ 所有测试通过！"
echo "==================================="
echo ""
echo "项目验证成功！"
echo ""
echo "快速开始:"
echo "  ./bin/gateway        # 启动 Gateway"
echo "  curl http://localhost:18790/health  # 健康检查"
echo ""
