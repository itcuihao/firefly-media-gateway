# firefly-media-gateway (MVP)

Go 实现的媒体网关服务，提供统一上传、访问、删除与元数据查询接口。

## MVP 能力

- 统一上传接口（图片/视频）
- 统一访问接口（默认 302 跳转）
- 元数据入库（PostgreSQL）
- Provider 抽象（`tg` 已实现，`r2` 占位）
- 基础 Bearer Token 鉴权

## API

- `POST /api/v1/media/upload`
- `GET /api/v1/media/{mediaId}`
- `GET /api/v1/media/{mediaId}/meta`
- `DELETE /api/v1/media/{mediaId}`
- `GET /api/v1/health`

## 上传限制

- 图片：`jpg/jpeg/png/webp`，最大 `10MB`
- 视频：`mp4/webm/mov`，最大 `120MB`

## 快速启动（Docker）

```bash
cp .env.example .env
# 填写 TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID / MEDIA_GATEWAY_TOKEN
docker compose up --build
```

服务默认监听 `http://localhost:8080`。

## 本地运行（不走 Docker）

```bash
cp .env.example .env
# 修改 DATABASE_URL 指向你的 PostgreSQL
set -a; source .env; set +a

GOCACHE="$(pwd)/.gocache" GOMODCACHE="$(pwd)/.gomodcache" go run ./cmd/server
```

## 鉴权说明

以下接口需要请求头：

```text
Authorization: Bearer <MEDIA_GATEWAY_TOKEN>
```

- `POST /api/v1/media/upload`
- `GET /api/v1/media/{mediaId}/meta`
- `DELETE /api/v1/media/{mediaId}`

`GET /api/v1/media/{mediaId}` 为公开访问路径（用于分享页）。

## 数据库

初始化 SQL：`migrations/001_init.sql`

核心表：`media_assets`

