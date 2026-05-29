#!/usr/bin/env bash
# =================================================================
# Firefly Media Gateway — 一键启动脚本
# 
# 用法:
#   ./scripts/start.sh          本地开发模式（前后端分别启动）
#   ./scripts/start.sh build    生产构建（前端构建 + Go 编译）
#   ./scripts/start.sh run      生产构建后直接运行
# =================================================================
set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
BINARY="$PROJECT_ROOT/server"

COLOR_CYAN='\033[0;36m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_RED='\033[0;31m'
COLOR_RESET='\033[0m'

log()    { echo -e "${COLOR_CYAN}[Firefly]${COLOR_RESET} $*"; }
ok()     { echo -e "${COLOR_GREEN}[  OK  ]${COLOR_RESET} $*"; }
warn()   { echo -e "${COLOR_YELLOW}[ WARN ]${COLOR_RESET} $*"; }
err()    { echo -e "${COLOR_RED}[ ERR  ]${COLOR_RESET} $*"; exit 1; }

build_frontend() {
    log "正在构建前端 Vue 3 + Naive UI 资源..."
    cd "$FRONTEND_DIR"
    npm install --prefer-offline
    npm run build
    cd "$PROJECT_ROOT"
    ok "前端构建完成 → $FRONTEND_DIR/dist"
}

build_server() {
    log "正在编译 Go 服务端二进制..."
    go build -o "$BINARY" ./cmd/server
    ok "服务端编译完成 → $BINARY"
}

MODE="${1:-dev}"

case "$MODE" in
  build)
    # ── 生产构建（前端 + Go） ──
    build_frontend
    build_server
    ;;

  run)
    # ── 生产构建后运行 ──
    build_frontend
    build_server
    log "🚀 启动 Firefly Media Gateway..."
    exec "$BINARY"
    ;;

  dev | *)
    # ── 本地开发模式（默认）──
    # 检查 Node 和 Go 是否可用
    command -v node >/dev/null 2>&1 || err "未找到 node，请先安装 Node.js 18+"
    command -v go   >/dev/null 2>&1 || err "未找到 go，请先安装 Go 1.21+"

    log "=================================="
    log "  Firefly Media Gateway — 开发模式"
    log "=================================="
    log "后端地址:    http://localhost:8080"
    log "前端 (HMR):  http://localhost:5173/debug/ui/"
    log "注意: 开发时访问 Vite 地址，/api/v1 请求自动代理到后端"
    log "按 Ctrl+C 停止所有进程"
    log ""

    # 安装前端依赖（如需要）
    if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
        log "安装前端 npm 依赖..."
        cd "$FRONTEND_DIR" && npm install && cd "$PROJECT_ROOT"
    fi

    # 并发启动前端 Vite 开发服务器 + Go 后端
    trap 'log "正在停止所有进程..."; kill 0' SIGINT SIGTERM

    (cd "$FRONTEND_DIR" && npm run dev) &
    VITE_PID=$!

    go run ./cmd/server &
    BACKEND_PID=$!

    wait $VITE_PID $BACKEND_PID
    ;;
esac
