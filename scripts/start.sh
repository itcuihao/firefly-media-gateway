#!/usr/bin/env bash
# =================================================================
# Firefly Media Gateway — 统一启动脚本
#
# 用法:
#   ./scripts/start.sh           本地开发模式（自动加载 .env，同时启动 Vite + Go）
#   ./scripts/start.sh build     生产构建（前端构建 + Go 编译为单二进制）
#   ./scripts/start.sh run       生产构建后直接运行
# =================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
BINARY="$PROJECT_ROOT/server"

# ── 颜色 ──────────────────────────────────────────────────────────
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${CYAN}[Firefly]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[  OK  ]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[ WARN ]${NC} %s\n" "$*"; }
err()  { printf "${RED}[ ERR  ]${NC} %s\n" "$*" >&2; exit 1; }

# ── 加载 .env ─────────────────────────────────────────────────────
load_env() {
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        if [ -f "$PROJECT_ROOT/.env.example" ]; then
            cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
            warn ".env 不存在，已从 .env.example 复制"
            warn "请编辑 .env，填写 TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID / MEDIA_GATEWAY_TOKEN"
            exit 1
        else
            err ".env 不存在，也没有 .env.example 可复制"
        fi
    fi
    log "加载 .env ..."
    set -a; source "$PROJECT_ROOT/.env"; set +a
}

# ── 设置默认环境变量 ──────────────────────────────────────────────
set_defaults() {
    export STORAGE_MODE="${STORAGE_MODE:-direct}"
    export MEDIA_PROVIDER_DEFAULT="${MEDIA_PROVIDER_DEFAULT:-tg}"
    export APP_LISTEN_ADDR="${APP_LISTEN_ADDR:-:8080}"
    export PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-http://localhost:8080}"

    if [ -z "${DATABASE_URL:-}" ]; then
        export DATABASE_URL="data/media_gateway.db"
        export DATABASE_DRIVER="sqlite"
    fi
}

# ── 校验必填变量 ──────────────────────────────────────────────────
validate_env() {
    [ -z "${MEDIA_GATEWAY_TOKEN:-}" ] && err "MEDIA_GATEWAY_TOKEN 未设置，请在 .env 中配置"

    if [ "${STORAGE_MODE}" = "direct" ] && [ "${MEDIA_PROVIDER_DEFAULT}" = "tg" ]; then
        if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
            if [ -z "${TELEGRAM_BOTS_CONFIG:-}" ]; then
                err "direct 模式需要 TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID（或 TELEGRAM_BOTS_CONFIG）"
            fi
        fi
    fi
}

# ── 构建前端 ──────────────────────────────────────────────────────
build_frontend() {
    log "正在构建前端 Vue 3 + Naive UI 资源..."
    cd "$FRONTEND_DIR"
    npm install --prefer-offline
    npm run build
    cd "$PROJECT_ROOT"
    ok "前端构建完成 → uiembed/dist"
}

# ── 编译 Go 二进制 ────────────────────────────────────────────────
build_server() {
    log "正在编译 Go 服务端二进制..."
    cd "$PROJECT_ROOT"
    go build -o "$BINARY" ./cmd/server
    ok "编译完成 → $BINARY"
}

# ── 主流程 ────────────────────────────────────────────────────────
MODE="${1:-dev}"

case "$MODE" in
  build)
    # 纯构建，不运行，不需要 .env
    build_frontend
    build_server
    ok "构建完成，可通过 ./server 或 ./scripts/start.sh run 启动"
    ;;

  run)
    # 生产构建后运行，需要 .env
    build_frontend
    build_server
    load_env
    set_defaults
    validate_env
    mkdir -p "$PROJECT_ROOT/data"
    ok "存储模式: ${STORAGE_MODE} | 数据库: ${DATABASE_URL} | 监听: ${APP_LISTEN_ADDR}"
    log "🚀 启动 Firefly Media Gateway..."
    exec "$BINARY"
    ;;

  dev | *)
    # 本地开发模式：加载 .env + 同时启动 Vite HMR 和 Go 后端
    command -v node >/dev/null 2>&1 || err "未找到 node，请先安装 Node.js 18+"
    command -v go   >/dev/null 2>&1 || err "未找到 go，请先安装 Go 1.21+"

    load_env
    set_defaults
    validate_env
    mkdir -p "$PROJECT_ROOT/data"

    log "========================================"
    log "  Firefly Media Gateway — 本地开发模式"
    log "========================================"
    ok "存储模式: ${STORAGE_MODE}"
    ok "数据库:   ${DATABASE_URL}"
    ok "后端地址: http://localhost:${APP_LISTEN_ADDR##*:}"
    ok "前端 HMR: http://localhost:5173/admin/"
    log "（/api/v1 请求自动代理到后端，Ctrl+C 停止所有进程）"
    log ""

    # 安装前端依赖（如需要）
    if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
        log "首次运行，安装前端 npm 依赖..."
        cd "$FRONTEND_DIR" && npm install && cd "$PROJECT_ROOT"
    fi

    # 并发启动前端 + 后端，Ctrl+C 一并退出
    trap 'log "正在停止所有进程..."; kill 0' SIGINT SIGTERM

    (cd "$FRONTEND_DIR" && npm run dev) &
    go run "$PROJECT_ROOT/cmd/server" &

    wait
    ;;
esac
