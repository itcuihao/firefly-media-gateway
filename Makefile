# Firefly Media Gateway — Makefile
# ======================================
# 用法:
#   make dev       本地开发（前端 Vite HMR + 后端 go run）
#   make build     生产构建（先构建前端，再编译 Go 二进制）
#   make run       生产构建后直接运行
#   make test      运行所有 Go 单元测试
#   make clean     清理构建产物

BINARY   := ./server
GO_CMD   := go
NPM_CMD  := npm
FRONTEND := ./frontend

.PHONY: all build run dev test clean frontend-build frontend-dev

# ── 默认目标 ────────────────────────────────────────
all: build

# ── 生产构建（前端 + Go 二进制一体打包）──────────────
build: frontend-build
	@echo "📦 正在编译 Go 服务端二进制..."
	$(GO_CMD) build -o $(BINARY) ./cmd/server
	@echo "✅ 编译完成: $(BINARY)"

# ── 编译后直接运行 ────────────────────────────────────
run: build
	@echo "🚀 启动 Firefly Media Gateway..."
	$(BINARY)

# ── 仅构建前端 ────────────────────────────────────────
frontend-build:
	@echo "🔨 正在构建前端 Vue 3 资源..."
	cd $(FRONTEND) && $(NPM_CMD) install --prefer-offline && $(NPM_CMD) run build
	@echo "✅ 前端构建完成: $(FRONTEND)/dist"

# ── 本地开发模式 ──────────────────────────────────────
# 同时启动:
#   1. 后端 go run（热重启需搭配 air）
#   2. 前端 Vite 开发服务器（端口 5173，自动代理 /api/v1 到后端 8080）
dev:
	@echo "🔧 启动本地开发环境..."
	@echo "   后端: http://localhost:8080"
	@echo "   前端 (Vite HMR): http://localhost:5173/debug/ui/"
	@echo ""
	@# 并发启动前端开发服务器和后端
	@trap 'kill 0' SIGINT; \
	  (cd $(FRONTEND) && $(NPM_CMD) install --prefer-offline && $(NPM_CMD) run dev) & \
	  $(GO_CMD) run ./cmd/server ; \
	  wait

# ── 单元测试 ──────────────────────────────────────────
test:
	@echo "🧪 运行 Go 单元测试..."
	$(GO_CMD) test ./...

# ── 清理 ──────────────────────────────────────────────
clean:
	@echo "🧹 清理构建产物..."
	rm -f $(BINARY)
	rm -rf ./uiembed/dist
	@echo "✅ 清理完成"
