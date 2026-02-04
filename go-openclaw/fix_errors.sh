#!/bin/bash

# Go-OpenClaw 编译错误快速修复脚本

cd ~/.openclaw/workspace/go-openclaw

echo "🔧 快速修复编译错误..."

# 备份原文件
cp internal/protocol/protocol.go internal/protocol/protocol.go.bak
cp pkg/gateway/gateway.go pkg/gateway/gateway.go.bak

# 修复 1: 删除未使用的 seq 变量（heartbeat.go:112, 196）
sed -i '' '/^\s*h\.seq\s*int$/d' pkg/gateway/heartbeat.go

# 修复 2: 修复 lastSeen 大小写问题（heartbeat.go:148, 160）
sed -i '' 's/client\.LastSeen(/client.lastSeen(/g' pkg/gateway/heartbeat.go

# 修复 3: 添加自定义错误常量（gateway.go:83-95）
sed -i '' '/^var (/a\
// Custom errors
var (
	ErrServerClosed   = NewProtocolError("server closed")
	ErrClientClosed   = NewProtocolError("client closed")
)

' pkg/gateway/gateway.go

# 修复 4: 修复 delete() 参数问题（gateway.go:298）
sed -i '' 's/delete(g\.clients, client)/delete(g.clients, client.ID)/g' pkg/gateway/gateway.go

echo "✅ 基础修复完成！"
echo ""
echo "🧪 尝试编译..."
go build ./... 2>&1 | head -20

if [ $? -eq 0 ]; then
    echo "✅ 编译成功！"
    ls -lh bin/
else
    echo "❌ 编译仍有错误，继续手动修复..."
fi
