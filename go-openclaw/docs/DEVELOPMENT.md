# Go-OpenClaw 开发文档

## 快速开始

### 前置要求

- Go 1.23+ (推荐 1.25.6+)

### 编译

```bash
cd ~/.openclaw/workspace/go-openclaw

# 方法 1: 使用 go build
go build -o bin/gateway cmd/gateway/main.go

# 方法 2: 使用 Make
make build
```

### 运行

```bash
# 方法 1: 直接运行
./bin/gateway

# 方法 2: 使用 Make
make run
```

### 预期输出

```
🚀 Go-OpenClaw v0.0.1
2026/02/03 12:53:30 🌐 Gateway listening on :18790
```

## 测试

### 健康检查

```bash
curl http://localhost:18790/health
# 输出: {"status":"ok"}
```

### WebSocket 连接测试

需要安装 wscat:
```bash
npm install -g wscat
```

连接测试:
```bash
wscat -c ws://localhost:18790/ws
```

发送连接消息:
```json
{
  "type": "req",
  "id": "1",
  "method": "connect",
  "params": {
    "token": "test-token",
    "deviceId": "test-device",
    "version": "0.0.1"
  }
}
```

预期响应:
```json
{
  "type": "res",
  "id": "1",
  "ok": true,
  "payload": {
    "version": "0.0.1",
    "deviceId": "test-device",
    "sessionId": "session-XXX",
    "workspace": "default",
    "state": {
      "version": "0.0.1",
      "sessionId": "session-XXX",
      "workspace": "default"
    }
  }
}
```

## 项目结构

```
go-openclaw/
├── cmd/                    # 主程序入口
│   └── gateway/
│       └── main.go        # Gateway 主程序
├── pkg/                    # 可复用的公共包
│   ├── gateway/           # Gateway 核心逻辑
│   ├── session/           # Session 管理
│   ├── agent/             # Agent 运行时
│   ├── channels/          # 消息渠道接口
│   ├── tools/             # 内置工具
│   ├── skills/            # Skills 系统
│   ├── config/            # 配置管理
│   ├── scheduler/         # Cron 调度
│   ├── nodes/             # Nodes 控制
│   ├── canvas/            # Canvas 系统
│   └── auth/              # 认证与配对
├── internal/               # 内部实现（不对外暴露）
│   ├── protocol/          # WebSocket 协议
│   ├── models/            # 数据模型
│   ├── storage/           # 存储层
│   └── ws/                # WebSocket 处理
├── channels/               # 各渠道实现
│   ├── telegram/
│   ├── whatsapp/
│   ├── slack/
│   ├── discord/
│   └── imessage/
├── web/                    # Web UI
│   ├── gateway/
│   └── canvas/
├── api/                    # OpenAPI spec
├── docs/                   # 文档
├── scripts/                # 构建脚本
├── test/                   # 测试
├── bin/                    # 编译输出
├── go.mod                  # Go 模块定义
├── go.sum                  # 依赖锁定文件
├── Makefile               # 构建脚本
├── README.md              # 项目说明
└── .gitignore             # Git 忽略文件
```

## 开发指南

### 添加新的依赖

```bash
go get github.com/example/package
go mod tidy
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./pkg/gateway

# 带详细输出的测试
go test -v ./pkg/gateway
```

### 代码格式化

```bash
go fmt ./...
```

### 代码检查

```bash
go vet ./...
```

## WebSocket 协议

### 消息类型

1. **req**: 请求消息
   ```json
   {
     "type": "req",
     "id": "1",
     "method": "connect",
     "params": {}
   }
   ```

2. **res**: 响应消息
   ```json
   {
     "type": "res",
     "id": "1",
     "ok": true,
     "payload": {}
   }
   ```

3. **event**: 事件消息
   ```json
   {
     "type": "event",
     "event": "message",
     "payload": {},
     "seq": 1
   }
   ```

### 支持的方法

- `connect`: 握手连接
- `health`: 健康检查

### 支持的事件

暂无（待实现）

## 故障排查

### 端口被占用

如果 18790 端口被占用，可以在 `cmd/gateway/main.go` 中修改端口号：

```go
gw := gateway.New(":18790")  // 修改为其他端口
```

### 编译错误

确保 Go 版本正确：
```bash
go version  # 需要 1.23+
```

清理并重新编译：
```bash
make clean
make build
```

## 下一步

- [ ] 实现 CLI 命令行工具
- [ ] 添加配置文件支持
- [ ] 实现 Agent 运行时
- [ ] 实现 Session 管理
- [ ] 实现 Telegram 渠道
- [ ] 实现 Skills 系统
- [ ] 实现 Cron 调度

## 参考资料

- [Go 官方文档](https://golang.org/doc/)
- [fasthttp 文档](https://github.com/valyala/fasthttp)
- [fasthttp/websocket 文档](https://github.com/fasthttp/websocket)
- [OpenClaw 原始项目](https://github.com/openclaw/openclaw)
