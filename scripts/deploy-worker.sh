#!/usr/bin/env bash
# =================================================================
# Firefly Media Gateway — One-Click Cloudflare Worker Deployer
# =================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKER_DIR="$PROJECT_ROOT/workers"

# ── Colors ──
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${CYAN}[Worker Deployer]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[SUCCESS]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARNING]${NC} %s\n" "$*"; }
err()  { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

log "Checking dependencies..."
command -v node >/dev/null 2>&1 || err "Node.js is required but not installed. Please install Node.js 18+ first."
command -v npm >/dev/null 2>&1 || err "NPM is required but not installed."

cd "$WORKER_DIR"

log "Installing wrangler dependencies..."
npm install --prefer-offline

# Check wrangler login status
log "Checking Cloudflare login status..."
if ! npx wrangler whoami >/dev/null 2>&1; then
    warn "You are not logged into Cloudflare. Launching login process..."
    npx wrangler login
fi

log "Deploying Cloudflare Worker..."
# Non-interactive wrangler deploy
npx wrangler deploy --yes

ok "Cloudflare Worker deployed successfully!"
log "--------------------------------------------------------"
log "Worker URL: https://firefly-media-gateway.itcuihao.workers.dev"
log "--------------------------------------------------------"

# Secrets Setup
echo
log "Do you want to configure/update Worker Secrets now? (y/N)"
read -r CONFIRM
if [[ "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo
    log "========================================================"
    log "            Cloudflare Worker Secrets 配置"
    log "========================================================"
    
    echo -n "请输入 Telegram Bot Token: "
    read -r TG_TOKEN
    [ -n "$TG_TOKEN" ] && echo -n "$TG_TOKEN" | npx wrangler secret put TELEGRAM_BOT_TOKEN
    
    echo -n "请输入 Telegram Chat ID / Group ID: "
    read -r TG_CHAT
    [ -n "$TG_CHAT" ] && echo -n "$TG_CHAT" | npx wrangler secret put TELEGRAM_CHAT_ID
    
    echo -n "请输入 Worker Auth Token (Bearer 鉴权，留空不使用): "
    read -r WORKER_AUTH
    if [ -n "$WORKER_AUTH" ]; then
        echo -n "$WORKER_AUTH" | npx wrangler secret put AUTH_TOKEN
    fi
    
    ok "Worker Secrets 配置成功！"
else
    log "跳过 Secrets 配置。您稍后可以使用 'npx wrangler secret put <NAME>' 手动设置。"
fi
