#!/bin/bash
# =======================================================
# OpenClaw 快速部署脚本 v2.0 - 适用于 2C2G 内存机器
# 改进：多下载源、超时处理、安装验证、清理功能
# =======================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置变量
INSTALL_DIR="$HOME/.openclaw"
BIN_DIR="$INSTALL_DIR/bin"
LOG_FILE="/tmp/openclaw-deploy.log"

# 下载源（GitHub + 国内镜像）
DOWNLOAD_SOURCES=(
  "https://github.com/openclaw/openclaw/releases/latest/download"
  "https://ghproxy.com/https://github.com/openclaw/openclaw/releases/latest/download"
)

# 清理函数
cleanup() {
  echo -e "${YELLOW}🧹 Cleaning up..."
  rm -rf "$INSTALL_DIR" 2>/dev/null || true
  echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# 错误处理
error_exit() {
  echo -e "${RED}❌ ERROR: $1${NC}"
  echo "📋 See log: $LOG_FILE"
  exit 1
}

# 进度显示
show_progress() {
  echo -e "${BLUE}⏳ $1${NC}"
}

# 成功消息
success_msg() {
  echo -e "${GREEN}✅ $1${NC}"
}

# 显示菜单
show_menu() {
  echo -e "${BLUE}═════════════════════════════════${NC}"
  echo -e "${BLUE}  OpenClaw Deployment Menu${NC}"
  echo -e "${BLUE}═══════════════════════════════${NC}"
  echo ""
  echo "1) Install/Update OpenClaw"
  echo "2) Uninstall OpenClaw"
  echo "3) Status Check"
  echo "4) View Logs"
  echo "5) Exit"
  echo ""
}

# 安装函数
install_openclaw() {
  echo -e "${BLUE}🚀 OpenClaw Deployment v2.0${NC}"
  echo -e "${BLUE}===================================${NC}"
  echo ""

  # 检测系统架构
  show_progress "Detecting system architecture..."
  ARCH=$(uname -m)
  OS=$(uname -s)

  case "$OS" in
    Darwin)
      case "$ARCH" in
        x86_64)
          BINARY_FILE="openclaw-darwin-amd64"
          ;;
        arm64)
          BINARY_FILE="openclaw-darwin-arm64"
          ;;
        *)
          error_exit "Unsupported architecture: $ARCH on macOS"
          ;;
      esac
      ;;
    Linux)
      case "$ARCH" in
        x86_64)
          BINARY_FILE="openclaw-linux-amd64"
          ;;
        aarch64)
          BINARY_FILE="openclaw-linux-arm64"
          ;;
        armv7l)
          BINARY_FILE="openclaw-linux-armv7"
          ;;
        *)
          error_exit "Unsupported architecture: $ARCH on Linux"
          ;;
      esac
      ;;
    *)
      error_exit "Unsupported OS: $OS"
      ;;
  esac

  success_msg "Detected: $OS $ARCH"
  echo "📦 Binary: $BINARY_FILE"
  echo ""

  # 创建目录
  show_progress "Creating directories..."
  mkdir -p "$BIN_DIR"
  mkdir -p "$INSTALL_DIR/config"
  mkdir -p "$INSTALL_DIR/logs"
  success_msg "Directories created"
  echo ""

  # 下载二进制文件（尝试多个源）
  show_progress "Downloading OpenClaw binary..."
  DOWNLOAD_SUCCESS=false

  for SOURCE in "${DOWNLOAD_SOURCES[@]}"; do
    DOWNLOAD_URL="$SOURCE/$BINARY_FILE"
    echo "📥 Trying: $DOWNLOAD_URL"

    if command -v curl &> /dev/null; then
      # 使用 curl 下载，带超时和进度
      if curl -L --max-time 300 --connect-timeout 30 --progress-bar \
        -o "$BIN_DIR/$BINARY_FILE.tmp" \
        "$DOWNLOAD_URL" 2>&1 | tee -a "$LOG_FILE"; then
        DOWNLOAD_SUCCESS=true
        break
      else
        echo "⚠️  curl failed, trying source"
      fi
    elif command -v wget &> /dev/null; then
      # 备用 wget
      if wget -T 10 -c --show-progress -O "$BIN_DIR/$BINARY_FILE.tmp" \
        "$DOWNLOAD_URL" 2>&1 | tee -a "$LOG_FILE"; then
        DOWNLOAD_SUCCESS=true
        break
      else
        echo "⚠️  wget failed, trying source"
      fi
    else
      echo "⚠️  No curl or wget available"
    fi
  done

  if [ "$DOWNLOAD_SUCCESS" = false ]; then
    error_exit "Failed to download from all sources. Check your internet connection."
  fi

  # 验证下载
  if [ ! -f "$BIN_DIR/$BINARY_FILE.tmp" ]; then
    error_exit "Downloaded file not found"
  fi

  # 重命名
  mv "$BIN_DIR/$BINARY_FILE.tmp" "$BIN_DIR/$BINARY_FILE"
  success_msg "Download complete: $BIN_DIR/$BINARY_FILE"
  echo ""

  # 验证二进制文件
  show_progress "Verifying binary..."
  if ! file "$BIN_DIR/$BINARY_FILE" | grep -q "Mach-O"; then
    error_exit "Downloaded file is not a valid binary"
  fi
  success_msg "Binary verified"
  echo ""

  # 设置执行权限
  show_progress "Setting permissions..."
  chmod +x "$BIN_DIR/$BINARY_FILE"
  success_msg "Permissions set"
  echo ""

  # 创建符号链接
  show_progress "Creating symlink..."
  rm -f "$INSTALL_DIR/openclaw" 2>/dev/null || true
  ln -s "$BIN_DIR/$BINARY_FILE" "$INSTALL_DIR/openclaw"
  success_msg "Symlink created: $INSTALL_DIR/openclaw -> $BIN_DIR/$BINARY_FILE"
  echo ""

  # 验证符号链接
  if [ ! -e "$INSTALL_DIR/openclaw" ]; then
    error_exit "Symlink verification failed"
  fi

  # 测试版本
  show_progress "Testing installation..."
  VERSION_OUTPUT=$("$INSTALL_DIR/openclaw" --version 2>&1 || echo "version check failed")
  if [ $? -eq 0 ]; then
    success_msg "Version check: $VERSION_OUTPUT"
  else
    echo -e "${YELLOW}⚠️  Version check had issues, but continuing...${NC}"
  fi
  echo ""

  # 创建启动脚本
  show_progress "Creating startup scripts..."

  # 快速启动脚本
  cat > "$HOME/openclaw-start.sh" << 'EOF'
#!/bin/bash
# OpenClaw Quick Start Script

export PATH="$HOME/.openclaw/bin:$PATH"
cd "$HOME/.openclaw"

# 检查是否已经在运行
if pgrep -f "openclaw" > /dev/null; then
  echo "⚠️  OpenClaw is already running"
  "$HOME/.openclaw/bin/openclaw" status
  exit 0
fi

# 启动 OpenClaw（前台）
echo "🚀 Starting OpenClaw..."
"$HOME/.openclaw/bin/openclaw"
EOF

  chmod +x "$HOME/openclaw-start.sh"
  success_msg "Quick start script created: ~/openclaw-start.sh"
  echo ""

  # 完整启动脚本
  cat > "$HOME/openclaw-run.sh" << 'EOF'
#!/bin/bash
# OpenClaw Complete Run Script (with logs)

export PATH="$HOME/.openclaw/bin:$PATH"

# 清理旧日志
[ -f "/tmp/openclaw.log" ] && rm -f "/tmp/openclaw.log"

# 后台启动
echo "🚀 Starting OpenClaw in background..."
nohup "$HOME/.openclaw/bin/openclaw" > /tmp/openclaw.log 2>&1 &

# 显示信息
sleep 2
echo "✅ OpenClaw started"
echo ""
echo "📊 Status:"
"$HOME/.openclaw/bin/openclaw" status
echo ""
echo "📋 Logs: /tmp/openclaw.log"
echo "🛑 Stop: pkill -f openclaw"
echo ""
echo "View logs: tail -f /tmp/openclaw.log"
EOF

  chmod +x "$HOME/openclaw-run.sh"
  success_msg "Run script created: ~/openclaw-run.sh"
  echo ""

  # 配置 PATH
  show_progress "Configuring PATH..."
  BASHRC_CONFIG="export PATH=\"$HOME/.openclaw/bin:\$PATH\""

  if [ -f "$HOME/.bashrc" ]; then
    if ! grep -q "openclaw/bin" "$HOME/.bashrc"; then
      echo "" >> "$HOME/.bashrc"
      echo "# OpenClaw" >> "$HOME/.bashrc"
      echo "$BASHRC_CONFIG" >> "$HOME/.bashrc"
      echo "" >> "$HOME/.bashrc"
      success_msg "Added to ~/.bashrc"
      echo "   Run: source ~/.bashrc"
    else
      success_msg "Already in ~/.bashrc"
    fi
  elif [ -f "$HOME/.bash_profile" ]; then
    if ! grep -q "openclaw/bin" "$HOME/.bash_profile"; then
      echo "" >> "$HOME/.bash_profile"
      echo "# OpenClaw" >> "$HOME/.bash_profile"
      echo "$BASHRC_CONFIG" >> "$HOME/.bash_profile"
      echo "" >> "$HOME/.bash_profile"
      success_msg "Added to ~/.bash_profile"
      echo "   Run: source ~/.bash_profile"
    else
      success_msg "Already in ~/.bash_profile"
    fi
  fi
  echo ""

  # 安装完成摘要
  echo -e "${GREEN}═════════════════════════════════${NC}"
  echo -e "${GREEN}  OpenClaw Installation Complete!${NC}"
  echo -e "${GREEN}═════════════════════════════════${NC}"
  echo ""
  echo "📋 Installation Summary:"
  echo "   Binary: $INSTALL_DIR/openclaw"
  echo "   Version: $VERSION_OUTPUT"
  echo "   Config:  $INSTALL_DIR/config"
  echo "   Logs:   $INSTALL_DIR/logs"
  echo ""
  echo "🚀 Quick Start:"
  echo "   ~/openclaw-start.sh"
  echo ""
  echo "🔧 Full Run (with logs):"
  echo "   ~/openclaw-run.sh"
  echo ""
  echo "📊 Commands:"
  echo "   openclaw status"
  echo "   openclaw sessions list"
  echo "   openclaw --help"
  echo ""
  echo "📖 Documentation:"
  echo "   https://docs.openclaw.ai"
  echo ""
  echo "🔧 Configuration:"
  echo "   $INSTALL_DIR/openclaw config"
  echo ""
  echo -e "${GREEN}═════════════════════════════════${NC}"
  echo ""
  echo -e "${GREEN}✨ You can now use OpenClaw!${NC}"
  echo ""
  echo "📋 Full log: $LOG_FILE"
}

# 卸载函数
uninstall_openclaw() {
  echo -e "${BLUE}🧹 Uninstalling OpenClaw...${NC}"
  echo ""

  if [ ! -d "$INSTALL_DIR" ]; then
    echo -e "${YELLOW}⚠️  OpenClaw is not installed${NC}"
    return
  fi

  # 停止运行
  if pgrep -f "openclaw" > /dev/null; then
    echo "🛑 Stopping OpenClaw..."
    pkill -f openclaw
    sleep 2
  fi

  # 清理
  cleanup

  # 移除 PATH 配置
  if [ -f "$HOME/.bashrc" ]; then
    sed -i.bak '/# OpenClaw/,/^export PATH=.*openclaw/d' "$HOME/.bashrc"
    echo "✅ Removed from ~/.bashrc"
  fi

  if [ -f "$HOME/.bash_profile" ]; then
    sed -i.bak '/# OpenClaw/,/^export PATH=.*openclaw/d' "$HOME/.bash_profile"
    echo "✅ Removed from ~/.bash_profile"
  fi

  # 删除启动脚本
  rm -f "$HOME/openclaw-start.sh" "$HOME/openclaw-run.sh" 2>/dev/null || true
  echo "✅ Startup scripts removed"
  echo ""

  success_msg "OpenClaw uninstalled!"
  echo ""
}

# 状态检查
check_status() {
  echo -e "${BLUE}📊 OpenClaw Status Check${NC}"
  echo -e "${BLUE}─────────────────────${NC}"
  echo ""

  # 安装状态
  if [ -d "$INSTALL_DIR" ]; then
    echo "✅ Installation: $INSTALL_DIR"
    if [ -f "$INSTALL_DIR/openclaw" ]; then
      BINARY=$("$INSTALL_DIR/openclaw" --version 2>&1 || echo "N/A")
      echo "   Version: $BINARY"
    else
      echo "   Version: Not installed (symlink missing)"
    fi
  else
    echo "❌ Installation: Not found"
  fi
  echo ""

  # 运行状态
  if pgrep -f "openclaw" > /dev/null; then
    RUNNING=$(pgrep -f "openclaw" | wc -l | awk '{print $1}')
    PID=$(pgrep -f "openclaw" | head -1 | awk '{print $1}')
    echo "✅ Status: Running (PID: $PID, $RUNNING process)"
  else
    echo "❌ Status: Not running"
  fi
  echo ""

  # 日志
  if [ -f "/tmp/openclaw.log" ]; then
    SIZE=$(ls -lh /tmp/openclaw.log | awk '{print $5}')
    LINES=$(wc -l < /tmp/openclaw.log)
    echo "📋 Log file: /tmp/openclaw.log ($SIZE, $LINES lines)"
  else
    echo "📋 Log file: Not found"
  fi
  echo ""

  # PATH 状态
  if grep -q "openclaw/bin" ~/.bashrc 2>/dev/null || \
     grep -q "openclaw/bin" ~/.bash_profile 2>/dev/null; then
    echo "✅ PATH: Configured"
  else
    echo "⚠️  PATH: Not configured (run: source ~/.bashrc)"
  fi
  echo ""
}

# 查看日志
view_logs() {
  echo -e "${BLUE}📋 OpenClaw Logs${NC}"
  echo -e "${BLUE}────────────${NC}"
  echo ""

  if [ ! -f "/tmp/openclaw.log" ]; then
    echo "❌ Log file not found: /tmp/openclaw.log"
    return
  fi

  if command -v tail &> /dev/null; then
    echo "Recent logs (last 50 lines):"
    echo ""
    tail -50 /tmp/openclaw.log
  else
    echo "⚠️  'tail' command not available"
  fi

  echo ""
  echo "Full log location: /tmp/openclaw.log"
  echo ""
}

# 主菜单循环
if [ $# -eq 0 ]; then
  show_menu
  echo -e "${BLUE}Enter choice [1-5]: ${NC}"
  read -r CHOICE

  case "$CHOICE" in
    1)
      install_openclaw
      ;;
    2)
      uninstall_openclaw
      ;;
    3)
      check_status
      ;;
    4)
      view_logs
      ;;
    5)
      echo "👋 Goodbye!"
      exit 0
      ;;
    *)
      echo -e "${RED}Invalid choice${NC}"
      ;;
  esac
elif [ "$1" = "install" ]; then
  install_openclaw
elif [ "$1" = "uninstall" ]; then
  uninstall_openclaw
elif [ "$1" = "status" ]; then
  check_status
elif [ "$1" = "logs" ]; then
  view_logs
else
  echo "Usage: $0 [install|uninstall|status|logs]"
  echo "  (no args for interactive menu)"
fi
