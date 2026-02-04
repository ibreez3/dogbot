#!/bin/bash

# 修复编译错误 - 第二阶段

cd ~/.openclaw/workspace/go-openclaw

echo "🔧 开始修复剩余编译错误..."

# 问题 1: 检查 protocol.go 中的 NewEvent 函数
echo "📝 检查 protocol.go 是否包含 NewEvent 函数..."
if grep -q "func NewEvent" internal/protocol/protocol.go; then
    echo "✅ NewEvent 函数存在"
else
    echo "❌ NewEvent 函数不存在，需要添加"
fi

echo ""
echo "🔧 修复完成！现在尝试编译..."
echo ""

# 尝试编译
go build ./...

if [ $? -eq 0 ]; then
    echo "✅ 编译成功！"
    ls -lh bin/
else
    echo "❌ 编译失败，查看错误信息"
fi
