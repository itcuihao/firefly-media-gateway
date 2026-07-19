#!/usr/bin/env bash
# =================================================================
# Firefly Nest (萤火虫之巢) — 生产环境一键拉取代码、重新编译和部署脚本
# =================================================================
set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { printf "${CYAN}[Deployer]${NC} %s\n" "$*"; }
ok()   { printf "${GREEN}[SUCCESS]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[WARNING]${NC} %s\n" "$*"; }
err()  { printf "${RED}[ERROR]${NC} %s\n" "$*" >&2; exit 1; }

# 确保在项目根目录运行
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

log "开始执行生产环境一键更新与重启流程..."

# 1. 清理本地追踪文件冲突（保留 .env 和 data/ 数据库，因为它们在 .gitignore 里）
log "清理本地代码仓库潜在冲突文件..."
git reset --hard
git clean -fd

# 2. 拉取最新代码
log "拉取最新 Git 代码..."
git pull

# 3. 强制无缓存编译最新的 Docker 镜像
log "构建最新 Docker 镜像（包含前端打包和 Go 二进制编译，不使用缓存确保代码更新）..."
docker build --no-cache --build-arg VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -t firefly-media-gateway:latest .

# 4. 重新启动并应用新镜像
log "更新并重启 Docker 容器..."
docker compose -f docker-compose.prod.yml up -d

# 5. 清理旧的无用镜像释放磁盘空间
log "清理废弃的 Docker 镜像..."
docker image prune -f

# 6. 检查状态
log "检查服务运行状态..."
sleep 2
docker ps -f name=firefly-media-gateway

ok "服务更新与重启完成！"
