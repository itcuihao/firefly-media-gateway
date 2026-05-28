# Firefly Media Gateway - 部署文档

## 架构概述

Firefly Media Gateway 支持三种部署模式：

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
| **Mode 1: Worker Only** | CF Workers | 无服务器、全球加速 | 无枚举能力 |
| **Mode 2: Go + Worker** | Go 后端 + Workers | 完整功能、边缘加速 | 需管理服务器 |
| **Mode 3: Go Direct** | Go 后端直连 TG | 完整功能、无依赖 | 无边缘加速 |

---

## 模式 1：Worker Only（轻量模式）

### 部署步骤

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

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `TELEGRAM_BOTS_CONFIG` | 是 | JSON 格式多 bot 配置 |
| `AUTH_TOKEN` | 否 | API 鉴权 token |

### API 端点

```
POST   /upload           # 上传文件
GET    /get              # 获取文件 URL
DELETE /delete           # 删除文件
GET    /health           # 健康检查
```

---

## 模式 2：Go + Worker（推荐）

### 架构图

```
                ┌─────────────────┐
                │   S3 Client     │
                │   HTTP Client   │
                └────────┬────────┘
                         │
                ┌────────▼────────┐
                │  Go Backend     │
                │  - 元数据 CRUD  │
                │  - 枚举/搜索    │
                │  - S3 Gateway   │
                └────────┬────────┘
                         │
                ┌────────▼────────┐
                │  CF Worker      │
                │  - 上传代理     │
                │  - 下载代理     │
                └────────┬────────┘
                         │
                ┌────────▼────────┐
                │  Telegram       │
                └─────────────────┘
```

### 部署步骤

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
# 构建镜像
docker build -t firefly-media-gateway .

# 运行
docker run -d \
  -e DATABASE_URL=postgres://user:pass@host:5432/dbname \
  -e STORAGE_MODE=proxy \
  -e WORKER_BASE_URL=https://your-worker.workers.dev \
  -e WORKER_AUTH_TOKEN=your_worker_token \
  -e MEDIA_GATEWAY_TOKEN=your_api_token \
  -e PUBLIC_BASE_URL=https://your-api.com \
  -p 8080:8080 \
  firefly-media-gateway
```

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `DATABASE_URL` | 是 | PostgreSQL 连接字符串 |
| `STORAGE_MODE` | 是 | 设为 `proxy` |
| `WORKER_BASE_URL` | 是 | Worker 服务 URL |
| `WORKER_AUTH_TOKEN` | 是 | Worker 鉴权 token |
| `MEDIA_GATEWAY_TOKEN` | 是 | Go API 鉴权 token |
| `PUBLIC_BASE_URL` | 是 | 公共访问 URL |

### API 端点

```
# REST API
POST   /api/v1/media              # 上传文件
GET    /api/v1/media/:id          # 获取元数据
GET    /api/v1/media              # 列举文件
DELETE /api/v1/media/:id          # 删除文件

# S3 兼容 API
PUT    /s3/{bucket}/{project}/{usage}/{filename}  # 上传
GET    /s3/{bucket}?asset_id=xxx                 # 下载
DELETE /s3/{bucket}?asset_id=xxx                 # 删除
GET    /s3/{bucket}                              # 列举
```

---

## 模式 3：Go Direct（直连模式）

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `DATABASE_URL` | 是 | PostgreSQL 连接字符串 |
| `STORAGE_MODE` | 是 | 设为 `direct` |
| `TELEGRAM_BOTS_CONFIG` | 是* | JSON 格式多 bot 配置 |
| `TELEGRAM_BOT_TOKEN` | 是* | 单 bot token（兼容） |
| `TELEGRAM_CHAT_ID` | 是* | 单 bot chat id（兼容） |
| `MEDIA_GATEWAY_TOKEN` | 是 | Go API 鉴权 token |
| `PUBLIC_BASE_URL` | 是 | 公共访问 URL |

* 多 bot 配置时使用 `TELEGRAM_BOTS_CONFIG`，单 bot 时使用 `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID`

### 单 Bot 配置示例

```bash
docker run -d \
  -e DATABASE_URL=postgres://user:pass@host:5432/dbname \
  -e STORAGE_MODE=direct \
  -e TELEGRAM_BOT_TOKEN=123456:ABC-DEF \
  -e TELEGRAM_CHAT_ID=-1001234567890 \
  -e MEDIA_GATEWAY_TOKEN=secret \
  -e PUBLIC_BASE_URL=https://api.example.com \
  -p 8080:8080 \
  firefly-media-gateway
```

### 多 Bot 配置示例

```bash
docker run -d \
  -e DATABASE_URL=postgres://user:pass@host:5432/dbname \
  -e STORAGE_MODE=direct \
  -e TELEGRAM_BOTS_CONFIG='{"bot1":{"token":"xxx","default_group":"-100xxx"},"bot2":{"token":"yyy","default_group":"-100yyy"}}' \
  -e MEDIA_GATEWAY_TOKEN=secret \
  -e PUBLIC_BASE_URL=https://api.example.com \
  -p 8080:8080 \
  firefly-media-gateway
```

---

## 数据库初始化

```bash
# 使用 Docker Compose
docker-compose -f docker-compose.yml up -d postgres

# 运行迁移
psql $DATABASE_URL < migrations/001_init.sql
```

---

## 验证部署

### Worker 健康检查

```bash
curl https://your-worker.workers.dev/health
```

### Go 后端健康检查

```bash
curl https://your-api.com/health
```

---

## 客户端使用示例

### cURL

```bash
# 上传文件
curl -X POST https://api.example.com/api/v1/media \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@image.jpg" \
  -F "project=myproject" \
  -F "usage=cover"

# 获取文件
curl https://api.example.com/api/v1/media/{asset_id}

# 删除文件
curl -X DELETE https://api.example.com/api/v1/media/{asset_id}
```

### AWS S3 SDK (Go)

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

## 故障排查

| 问题 | 解决方案 |
|------|----------|
| Worker 上传失败 | 检查 `TELEGRAM_BOTS_CONFIG` 格式 |
| Go 无法连接 Worker | 检查 `WORKER_BASE_URL` 和 `WORKER_AUTH_TOKEN` |
| S3 列举返回空 | 检查数据库连接和索引 |
| 文件 URL 404 | 检查 `PUBLIC_BASE_URL` 配置 |
