#!/usr/bin/env bash
# =================================================================
# Firefly Media Gateway — 一键服务器后端部署脚本
# =================================================================
set -euo pipefail

# ── 颜色 ──────────────────────────────────────────────────────────
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${CYAN}[Deployer]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[SUCCESS]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARNING]${NC} %s\n" "$*"; }
err()  { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

log "========================================================"
log "      Firefly Media Gateway 后端一键部署与配置工具"
log "========================================================"

# ── 选项选择 ──────────────────────────────────────────────────────
echo "请选择部署方式："
echo "  1) Docker Compose 容器化部署 (推荐，包含 PostgreSQL 支持)"
echo "  2) Systemd 本地二进制运行 (最轻量，使用 SQLite 数据库)"
echo -n "请输入选项 (1 或 2): "
read -r DEPLOY_OPTION

# ── 交互式配置环境变量 ──────────────────────────────────────────
echo
log "正在配置环境变量 (.env)..."

# 1. Access Token
echo -n "1. 设置网关 Access Token (Bearer 鉴权令牌，默认 firefly): "
read -r GATEWAY_TOKEN
GATEWAY_TOKEN="${GATEWAY_TOKEN:-firefly}"

# 2. Storage Mode
echo -n "2. 选择存储模式 [proxy (通过 CF Worker) / direct (直连 TG)] (默认 proxy): "
read -r MODE
MODE="${MODE:-proxy}"

# 3. Mode configs
WORKER_URL=""
WORKER_TOKEN=""
TG_TOKEN=""
TG_CHAT=""

if [ "$MODE" = "proxy" ]; then
    echo -n "  请输入 Cloudflare Worker 基础地址 (例如 https://firefly.workers.dev): "
    read -r WORKER_URL
    if [ -z "$WORKER_URL" ]; then
        err "Proxy 模式必须提供 WORKER_BASE_URL"
    fi
    echo -n "  请输入 Cloudflare Worker 鉴权 Token (如未配置可留空): "
    read -r WORKER_TOKEN
else
    echo -n "  请输入 Telegram Bot Token: "
    read -r TG_TOKEN
    echo -n "  请输入 Telegram Chat / Group ID: "
    read -r TG_CHAT
    if [ -z "$TG_TOKEN" ] || [ -z "$TG_CHAT" ]; then
        err "Direct 模式下 Telegram 凭证不能为空"
    fi
fi

# 4. Public base URL
echo -n "3. 请输入公网访问基准地址 (例如 http://your-ip-or-domain:8088): "
read -r BASE_URL
if [ -z "$BASE_URL" ]; then
    err "基准地址不能为空"
fi

# ── 写入 .env 文件 ──────────────────────────────────────────────
write_env_file() {
    local target_path="$1"
    mkdir -p "$(dirname "$target_path")"
    cat > "$target_path" <<EOF
# Firefly Media Gateway Configuration
MEDIA_GATEWAY_TOKEN=${GATEWAY_TOKEN}
STORAGE_MODE=${MODE}
PUBLIC_BASE_URL=${BASE_URL}
UPLOAD_TIMEOUT_SECONDS=120
EOF

    if [ "$MODE" = "proxy" ]; then
        cat >> "$target_path" <<EOF
WORKER_BASE_URL=${WORKER_URL}
WORKER_AUTH_TOKEN=${WORKER_TOKEN}
EOF
    else
        cat >> "$target_path" <<EOF
TELEGRAM_BOT_TOKEN=${TG_TOKEN}
TELEGRAM_CHAT_ID=${TG_CHAT}
MEDIA_PROVIDER_DEFAULT=tg
EOF
    fi
}

# ── 运行部署 ──────────────────────────────────────────────────────
if [ "$DEPLOY_OPTION" = "1" ]; then
    log "开始 Docker Compose 部署流程..."
    
    # 检查 Docker 是否安装
    command -v docker >/dev/null 2>&1 || err "未找到 docker 命令，请先安装 Docker。"
    command -v docker-compose >/dev/null 2>&1 || err "未找到 docker-compose 命令，请先安装 Docker Compose。"
    
    # 写入根目录的 .env
    write_env_file "./.env"
    
    log "启动 Docker 容器组..."
    if [ "$MODE" = "proxy" ]; then
        docker-compose --profile proxy up -d --build
        ok "Docker 部署成功！访问地址: ${BASE_URL}/admin/"
    else
        docker-compose up -d --build
        ok "Docker 部署成功！访问地址: ${BASE_URL}/admin/"
    fi
    
elif [ "$DEPLOY_OPTION" = "2" ]; then
    log "开始 Systemd 二进制部署流程..."
    
    DEPLOY_DIR="/opt/media-gateway"
    
    # 检查 Go 环境以进行现场编译，若没有 Go 环境则需要用户已存在 server 文件
    if ! command -v go >/dev/null 2>&1; then
        if [ ! -f "./server" ]; then
            err "未在系统检测到 go 环境，且当前目录不存在编译好的 ./server 二进制文件，无法进行部署。"
        fi
        log "检测到本地已有编译文件 ./server，正在复制..."
    else
        log "检测到 Go 环境，正在编译最新版本服务与内联前端资源..."
        # Vite 8 requires Node >= 22.12.0
        node_version=$(node -v 2>/dev/null | sed 's/^v//' | cut -d. -f1)
        if [ "${node_version:-0}" -lt 22 ]; then
            err "Vite 8 需要 Node.js >= 22.12.0，当前版本 $(node -v)。请先升级 Node.js。"
        fi
        # 编译前端和后端
        cd frontend && npm install --prefer-offline && npm run build && cd ..
        go build -o ./server ./cmd/server
        ok "前端与 Go 服务端编译完成！"
    fi
    
    # 准备目标目录
    log "正在配置目标目录 ${DEPLOY_DIR}..."
    sudo mkdir -p "${DEPLOY_DIR}/data"
    sudo cp ./server "${DEPLOY_DIR}/media-gateway"
    
    # 写入环境变量文件到部署目录
    sudo touch "${DEPLOY_DIR}/.env"
    sudo chmod 600 "${DEPLOY_DIR}/.env"
    
    # 构建 SQLite 所需的 DATABASE_URL
    write_env_file "./.env"
    cat >> "./.env" <<EOF
DATABASE_URL=${DEPLOY_DIR}/data/gateway.db
DATABASE_DRIVER=sqlite
APP_LISTEN_ADDR=:8088
EOF
    sudo mv ./.env "${DEPLOY_DIR}/.env"
    
    # 配置 Systemd
    log "正在生成 systemd 服务单元..."
    sudo cat > /etc/systemd/system/media-gateway.service <<EOF
[Unit]
Description=Firefly Media Gateway Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${DEPLOY_DIR}
ExecStart=${DEPLOY_DIR}/media-gateway
Restart=always
RestartSec=5
EnvironmentFile=${DEPLOY_DIR}/.env

[Install]
WantedBy=multi-user.target
EOF
    
    # 启动服务
    log "启动守护服务..."
    sudo systemctl daemon-reload
    sudo systemctl enable media-gateway --now
    
    ok "Systemd 部署成功！"
    log "服务状态："
    sudo systemctl status media-gateway --no-pager
    log "--------------------------------------------------------"
    log "管理控制台访问路径: ${BASE_URL}/admin/"
    log "数据存放目录: ${DEPLOY_DIR}/data/"
    log "--------------------------------------------------------"
else
    err "无效的选择。请输入 1 或 2。"
fi
