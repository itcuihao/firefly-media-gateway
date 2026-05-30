<div align="center">
  <img src="icon.png" width="120" alt="Firefly Media Gateway" />
</div>

# firefly-media-gateway

A self-hosted media storage gateway that uses Telegram as unlimited storage backend. Provides unified upload, access, deletion and metadata management via REST and S3-compatible APIs, with a built-in admin console.

自托管的媒体存储网关，通过 Telegram 作为无限存储后端，提供统一的上传、访问、删除与元数据管理接口，支持 REST API 和 S3 兼容 API，内置管理控制台。

## 功能

- 统一上传接口（图片/视频）
- 统一访问接口（二进制流式代理，支持 Range）
- 元数据入库（默认 SQLite，可配置 PostgreSQL）
- Provider 抽象（`tg` 已实现，`r2` 占位）
- 基础 Bearer Token 鉴权
- S3 兼容 API
- 内嵌管理 UI（`/admin/`，Vue 3 + Naive UI）

## API

### REST API

- `POST   /api/v1/media/upload` — 上传文件
- `GET    /api/v1/media/{mediaId}` — 获取媒体二进制内容
- `GET    /api/v1/media/{mediaId}/meta` — 获取元数据
- `GET    /api/v1/media/{mediaId}/stream` — 流式代理（支持 Range）
- `DELETE /api/v1/media/{mediaId}` — 删除文件
- `GET    /api/v1/media` — 列举文件
- `GET    /api/v1/health` — 健康检查

### S3 兼容 API

- `PUT    /s3/{bucket}/{project}/{usage}/{filename}` — 上传
- `GET    /s3/{bucket}?asset_id=xxx` — 下载
- `DELETE /s3/{bucket}?asset_id=xxx` — 删除
- `GET    /s3/{bucket}` — 列举

## 上传限制

- 图片：`jpg/jpeg/png/webp`，最大 `10MB`
- 视频：`mp4/webm/mov`，最大 `120MB`

## 鉴权

以下接口需要在请求头中携带 Token：

```text
Authorization: Bearer <MEDIA_GATEWAY_TOKEN>
```

`GET /api/v1/media/{mediaId}` 与 `GET /api/v1/media/{mediaId}/stream` 均返回媒体二进制内容，需要鉴权。`/stream` 会代理底层 provider 的文件流；单文件支持透传 `Range` 请求，分片文件暂不支持 Range。

## 数据库

默认使用无 CGO 的 SQLite，数据库文件为 `data/media_gateway.db`，启动时自动建表，无需额外配置。

如需 PostgreSQL：

```text
DATABASE_DRIVER=postgres
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=disable
```

PostgreSQL 初始化 SQL：`migrations/001_init.sql`

核心表：`media_assets`

---

## 部署

### 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                        业务方 / SDK                              │
└────────────────────────────┬────────────────────────────────────┘
                             │
        ┌────────────────────┴────────────────────┐
        │                                         │
   ┌────▼────┐                              ┌────▼────┐
   │  Mode 1 │                              │  Mode 2 │
   │  Worker │                              │ Go +    │
   │  Only   │                              │ Worker  │
   └──────────┘                              └────┬────┘
        │                                        │
        │                                  ┌────▼────┐
        │                                  │   Go    │
        │                                  │  Direct │
        │                                  └──────────┘
        │                                        │
        └────────────────┬───────────────────────┘
                         │
                   ┌─────▼─────┐
                   │ Telegram  │
                   │  Storage  │
                   └───────────┘
```

| 模式 | 组件 | 优点 | 缺点 |
|------|------|------|------|
| **Worker Only** | CF Workers | 无服务器、全球加速 | 无枚举能力 |
| **Go + Worker** | Go 后端 + Workers | 完整功能、边缘加速 | 需管理服务器 |
| **Go Direct** | Go 后端直连 TG | 完整功能、无依赖 | 无边缘加速 |

### 前置准备

| 项目 | 说明 |
|------|------|
| Telegram Bot Token | 从 [@BotFather](https://t.me/BotFather) 创建 Bot 获取 |
| Telegram Chat ID | 创建群组/频道，将 Bot 加入，获取群组 ID（负数，如 `-1001234567890`） |
| MEDIA_GATEWAY_TOKEN | 自定义一个强密钥，用于 API 鉴权 |
| 服务器 + 域名 | 运行 Docker，提供公网访问（`PUBLIC_BASE_URL`） |
| Cloudflare 账号 | 仅 Worker 相关模式需要（免费版即可） |

---

### 快速启动（Go Direct，最简模式）

```bash
cp .env.example .env
# 编辑 .env，填写 TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID / MEDIA_GATEWAY_TOKEN
docker compose up --build
```

服务默认监听 `http://localhost:8080`。

> Docker 三阶段构建自动完成：Node.js 编译前端 → Go 编译二进制（`go:embed` 嵌入前端）→ Distroless 运行镜像。

#### 本地运行（不走 Docker）

```bash
cp .env.example .env
set -a; source .env; set +a
GOCACHE="$(pwd)/.gocache" GOMODCACHE="$(pwd)/.gomodcache" go run ./cmd/server
```

---

### 模式 1：Worker Only（轻量模式）

```bash
cd workers
npm install

# 配置 secrets
wrangler secret put TELEGRAM_BOTS_CONFIG
# 输入: {"bot1":{"token":"xxx","default_group":"-100xxx"}}

# 可选：鉴权 token
wrangler secret put AUTH_TOKEN

# 部署
npm run deploy
```

**环境变量：**

| 变量 | 必填 | 说明 |
|------|------|------|
| `TELEGRAM_BOTS_CONFIG` | 是 | JSON 格式多 Bot 配置 |
| `AUTH_TOKEN` | 否 | API 鉴权 Token |

**API 端点：**

```
POST   /upload           # 上传文件
GET    /get              # 获取文件 URL
DELETE /delete           # 删除文件
GET    /health           # 健康检查
```

---

### 模式 2：Go + Worker（推荐）

```
┌─────────────────┐
│   S3 / HTTP     │
└────────┬────────┘
┌────────▼────────┐
│  Go Backend     │
│  - 元数据 CRUD  │
│  - S3 Gateway   │
└────────┬────────┘
┌────────▼────────┐
│  CF Worker      │
│  - 上传/下载代理 │
└────────┬────────┘
┌────────▼────────┐
│  Telegram       │
└─────────────────┘
```

#### 1. 部署 Worker

```bash
cd workers
npm install
wrangler secret put TELEGRAM_BOTS_CONFIG
wrangler secret put AUTH_TOKEN
npm run deploy
```

#### 2. 部署 Go 后端

```bash
docker build -t firefly-media-gateway .

docker run -d \
  -e STORAGE_MODE=proxy \
  -e WORKER_BASE_URL=https://your-worker.workers.dev \
  -e WORKER_AUTH_TOKEN=your_worker_token \
  -e MEDIA_GATEWAY_TOKEN=your_api_token \
  -e PUBLIC_BASE_URL=https://your-api.com \
  -v media_gateway_data:/app/data \
  -p 8080:8080 \
  firefly-media-gateway
```

**环境变量：**

| 变量 | 必填 | 说明 |
|------|------|------|
| `STORAGE_MODE` | 是 | 设为 `proxy` |
| `WORKER_BASE_URL` | 是 | Worker 服务 URL |
| `WORKER_AUTH_TOKEN` | 是 | Worker 鉴权 Token |
| `MEDIA_GATEWAY_TOKEN` | 是 | Go API 鉴权 Token |
| `PUBLIC_BASE_URL` | 是 | 公共访问 URL |
| `DATABASE_DRIVER` | 否 | `sqlite` 或 `postgres`，不填时根据 `DATABASE_URL` 自动判断 |
| `DATABASE_URL` | 否 | 不填默认 SQLite: `data/media_gateway.db` |

---

### 模式 3：Go Direct（直连模式）

**环境变量：**

| 变量 | 必填 | 说明 |
|------|------|------|
| `STORAGE_MODE` | 是 | 设为 `direct` |
| `TELEGRAM_BOTS_CONFIG` | 是* | JSON 格式多 Bot 配置 |
| `TELEGRAM_BOT_TOKEN` | 是* | 单 Bot Token（兼容） |
| `TELEGRAM_CHAT_ID` | 是* | 单 Bot Chat ID（兼容） |
| `MEDIA_GATEWAY_TOKEN` | 是 | Go API 鉴权 Token |
| `PUBLIC_BASE_URL` | 是 | 公共访问 URL |
| `DATABASE_DRIVER` | 否 | `sqlite` 或 `postgres`，不填时根据 `DATABASE_URL` 自动判断 |
| `DATABASE_URL` | 否 | 不填默认 SQLite: `data/media_gateway.db` |

\* 多 Bot 配置时使用 `TELEGRAM_BOTS_CONFIG`，单 Bot 时使用 `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID`

**单 Bot：**

```bash
docker run -d \
  -e STORAGE_MODE=direct \
  -e TELEGRAM_BOT_TOKEN=123456:ABC-DEF \
  -e TELEGRAM_CHAT_ID=-1001234567890 \
  -e MEDIA_GATEWAY_TOKEN=secret \
  -e PUBLIC_BASE_URL=https://api.example.com \
  -v media_gateway_data:/app/data \
  -p 8080:8080 \
  firefly-media-gateway
```

**多 Bot：**

```bash
docker run -d \
  -e STORAGE_MODE=direct \
  -e TELEGRAM_BOTS_CONFIG='{"bot1":{"token":"xxx","default_group":"-100xxx"},"bot2":{"token":"yyy","default_group":"-100yyy"}}' \
  -e MEDIA_GATEWAY_TOKEN=secret \
  -e PUBLIC_BASE_URL=https://api.example.com \
  -v media_gateway_data:/app/data \
  -p 8080:8080 \
  firefly-media-gateway
```

---

### 客户端使用示例

#### cURL

```bash
# 上传文件
curl -X POST https://api.example.com/api/v1/media \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@image.jpg" \
  -F "project=myproject" \
  -F "usage=cover"

# 获取文件
curl https://api.example.com/api/v1/media/{asset_id} \
  -H "Authorization: Bearer $TOKEN"

# 删除文件
curl -X DELETE https://api.example.com/api/v1/media/{asset_id} \
  -H "Authorization: Bearer $TOKEN"
```

#### AWS S3 SDK (Go)

```go
cfg, _ := config.LoadDefaultConfig(context.TODO(),
    config.WithEndpointURL("https://api.example.com"),
    config.WithRegion("auto"),
)
client := s3.NewFromConfig(cfg)

// 上传
_, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
    Bucket: aws.String("media-assets"),
    Key:    aws.String("myproject/cover/image.jpg"),
    Body:   bytes.NewReader(data),
})

// 下载
resp, _ := client.GetObject(context.TODO(), &s3.GetObjectInput{
    Bucket: aws.String("media-assets"),
    Key:    aws.String("myproject/cover/image.jpg"),
})
```

---

### 生产部署（CI/CD）

推送 `v*` 标签时，GitHub Actions 自动 SSH 到服务器执行 `git pull → docker build → docker compose up`。

#### 1. 服务器准备

```bash
# 克隆仓库到部署目录
git clone https://github.com/your-org/firefly-media-gateway.git /opt/firefly-media-gateway
cd /opt/firefly-media-gateway

# 创建 .env（不进 git）
cp .env.example .env
# 编辑填写必要配置，不设 DATABASE_URL 则默认 SQLite
```

`.env` 示例：

```bash
STORAGE_MODE=direct
TELEGRAM_BOT_TOKEN=xxx
TELEGRAM_CHAT_ID=-100xxx
MEDIA_GATEWAY_TOKEN=your_strong_secret
PUBLIC_BASE_URL=https://your-domain.com
```

#### 2. 配置 GitHub Secrets

在仓库 Settings → Secrets and variables → Actions 中添加：

| Secret | 说明 |
|--------|------|
| `DEPLOY_HOST` | 服务器 IP |
| `DEPLOY_USER` | SSH 用户名 |
| `DEPLOY_SSH_KEY` | SSH 私钥 |
| `DEPLOY_PORT` | SSH 端口（可选，默认 22） |

#### 3. 触发部署

```bash
git tag v1.0.0
git push --tags
```

也可在 GitHub Actions 页面手动触发（`workflow_dispatch`）。

---

### 验证部署

```bash
# Go 后端
curl https://your-api.com/api/v1/health

# Worker
curl https://your-worker.workers.dev/health
```

### 故障排查

| 问题 | 解决方案 |
|------|----------|
| Worker 上传失败 | 检查 `TELEGRAM_BOTS_CONFIG` JSON 格式是否正确 |
| Go 无法连接 Worker | 检查 `WORKER_BASE_URL` 和 `WORKER_AUTH_TOKEN` |
| S3 列举返回空 | 检查数据库连接和索引 |
| 文件 URL 404 | 检查 `PUBLIC_BASE_URL` 配置 |
| SQLite 错误 | 确认 `/app/data` 目录有写权限 |
