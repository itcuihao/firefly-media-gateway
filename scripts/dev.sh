#!/bin/bash
set -euo pipefail

CDPATH="" cd -- "$(dirname -- "$0")/.." || exit 1

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { printf "${CYAN}[INFO]${NC}  %s\n" "$1"; }
ok()    { printf "${GREEN}[OK]${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${NC}  %s\n" "$1"; }
die()   { printf "${RED}[ERROR]${NC} %s\n" "$1" >&2; exit 1; }

# ── .env ──────────────────────────────────────────────
if [ ! -f .env ]; then
  if [ -f .env.example ]; then
    cp .env.example .env
    warn ".env not found, copied from .env.example"
    warn "Please edit .env and fill in TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID / MEDIA_GATEWAY_TOKEN"
    exit 1
  else
    die ".env not found and no .env.example to copy from"
  fi
fi

info "Loading .env ..."
set -a; source .env; set +a

# ── Defaults for local dev ────────────────────────────
export STORAGE_MODE="${STORAGE_MODE:-direct}"
export MEDIA_PROVIDER_DEFAULT="${MEDIA_PROVIDER_DEFAULT:-tg}"
export APP_LISTEN_ADDR="${APP_LISTEN_ADDR:-:8080}"
export PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-http://localhost:8080}"

# Default to SQLite if no DATABASE_URL set
if [ -z "${DATABASE_URL:-}" ]; then
  export DATABASE_URL="data/media_gateway.db"
  export DATABASE_DRIVER="sqlite"
fi

# ── Validate ──────────────────────────────────────────
[ -z "${MEDIA_GATEWAY_TOKEN:-}" ] && die "MEDIA_GATEWAY_TOKEN is required in .env"

if [ "$STORAGE_MODE" = "direct" ] && [ "${MEDIA_PROVIDER_DEFAULT}" = "tg" ]; then
  if [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; then
    if [ -z "${TELEGRAM_BOTS_CONFIG:-}" ]; then
      die "TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID (or TELEGRAM_BOTS_CONFIG) required for direct mode"
    fi
  fi
fi

# ── Prepare ───────────────────────────────────────────
mkdir -p data
ok "Storage mode: ${STORAGE_MODE}"
ok "Database:     ${DATABASE_URL}"
ok "Listen:       ${APP_LISTEN_ADDR}"

# ── Run ───────────────────────────────────────────────
info "Starting Firefly Media Gateway ..."
exec go run ./cmd/server
