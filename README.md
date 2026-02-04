# Go-OpenClaw

Go 语言完整重写 OpenClaw —— 网关驱动的 AI Agent 平台

## 项目目标

- 🚀 单个可编译运行的二进制文件
- 📡 WebSocket 架构的 Gateway 守护进程
- 🤖 支持多渠道（Telegram, WhatsApp, Slack, Discord 等）
- 🧩 Skills 插件系统
- ⏰ Cron 调度 + Webhooks
- 📊 Canvas 可视化工作区
- 📱 Nodes 设备控制

## 当前状态

🟢 **阶段 1 完成** - Gateway 核心

### 已完成功能

- ✅ 项目初始化
- ✅ 目录结构创建
- ✅ WebSocket 服务器（fasthttp/websocket）
- ✅ 基础协议实现（connect, health）
- ✅ 连接管理
- ✅ 编译测试通过

## 技术栈

- **Go**: 1.25.6+
- **WebSocket**: github.com/fasthttp/websocket
- **HTTP Server**: github.com/valyala/fasthttp

## 编译运行

### 编译

```bash
cd ~/.openclaw/workspace/go-openclaw
go build -o bin/gateway cmd/gateway/main.go
```

### 运行

```bash
./bin/gateway
```

预期输出：
```
🚀 Go-OpenClaw v0.0.1
🌐 Gateway listening on :18789
```

### 测试

```bash
# 健康检查
curl http://localhost:18789/health

# WebSocket 连接测试（使用 wscat）
wscat -c ws://localhost:18789/ws
# 然后发送：
{"type":"req","id":"1","method":"connect","params":{"token":"test","deviceId":"test-device","version":"0.0.1"}}
```

## 项目结构

```
go-openclaw/
├── cmd/
│   └── gateway/          # Gateway 主程序
├── pkg/
│   └── gateway/          # Gateway 核心逻辑
├── internal/
│   └── protocol/         # WebSocket 协议
├── channels/             # 消息渠道实现
├── web/                  # Web UI
├── docs/                 # 文档
└── go.mod
```

## 开发计划

- [x] 阶段 1: Gateway 核心（WebSocket 服务器）
- [ ] 阶段 2: CLI + Config
- [ ] 阶段 3: Agent 运行时
- [ ] 阶段 4: Session 管理
- [ ] 阶段 5: Telegram 渠道
- [ ] 阶段 6: Skills 系统
- [ ] 阶段 7: Cron 调度
- [ ] 阶段 8: 其他渠道
- [ ] 阶段 9: Canvas 系统
- [ ] 阶段 10: Nodes 控制

## 贡献

欢迎贡献！请先阅读项目文档和开发规范。

## 许可证

MIT License

## 参考

- OpenClaw (Node.js): https://github.com/openclaw/openclaw
- 文档: https://docs.openclaw.ai
