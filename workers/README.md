# Firefly Media Gateway - Cloudflare Workers

轻量级媒体上传网关，将文件转发到 Telegram 存储，返回元数据供业务方自行处理。

## 特性

- **多 Bot/Group 支持**：支持配置多个 bot 和多个 group
- **无状态**：不存储元数据，业务方自行管理
- **全球边缘**：Cloudflare Workers 全球部署
- **简单鉴权**：可选 Bearer Token
- **类型安全**：TypeScript + 类型定义

## 快速开始

### 1. 安装依赖

```bash
cd workers
npm install
```

### 2. 配置环境变量

**方式一：单 Bot 配置**

```bash
wrangler secret put TELEGRAM_BOT_TOKEN
wrangler secret put TELEGRAM_CHAT_ID
```

**方式二：多 Bot 配置（推荐）**

```bash
wrangler secret put TELEGRAM_BOTS_CONFIG
# 输入 JSON 格式配置：
# {
#   "bot1": {
#     "token": "bot_token_1",
#     "default_group": "-1001234567890"
#   },
#   "bot2": {
#     "token": "bot_token_2",
#     "default_group": "-1009876543210"
#   }
# }
```

**可选配置**：

```bash
# 设置鉴权 token
wrangler secret put AUTH_TOKEN
```

### 3. 本地开发

```bash
npm run dev
```

### 4. 部署

```bash
npm run deploy
```

### 5. GitHub Actions 自动部署

仓库已提供 `.github/workflows/worker-deploy.yml`，当 `main` 分支的 `workers/**` 发生变化时会自动部署 Worker，也可以在 GitHub Actions 页面手动触发。

需要在 GitHub 仓库 Secrets 中配置：

```text
CLOUDFLARE_API_TOKEN
CLOUDFLARE_ACCOUNT_ID
```

Worker 运行时 secrets 仍通过 Wrangler 管理：

```bash
wrangler secret put TELEGRAM_BOTS_CONFIG
wrangler secret put AUTH_TOKEN
```

## API 使用

### POST /upload

上传文件到 Telegram。

**单 Bot 请求**

单 Bot 配置时不需要传 `bot` 或 `X-Bot-Name`，Worker 会直接使用 `TELEGRAM_BOT_TOKEN` 和 `TELEGRAM_CHAT_ID`。

```bash
curl -X POST https://your-worker.workers.dev/upload \
  -H "Authorization: Bearer YOUR_AUTH_TOKEN" \
  -F "file=@/path/to/image.jpg"
```

**多 Bot 请求：使用 Header 指定 Bot**

```bash
curl -X POST https://your-worker.workers.dev/upload \
  -H "Authorization: Bearer YOUR_AUTH_TOKEN" \
  -H "X-Bot-Name: bot1" \
  -F "file=@/path/to/image.jpg" \
  -F "group=-1001234567890"
```

**多 Bot 请求：使用 Form 字段指定 Bot**

```bash
curl -X POST https://your-worker.workers.dev/upload \
  -H "Authorization: Bearer YOUR_AUTH_TOKEN" \
  -F "file=@/path/to/image.jpg" \
  -F "bot=bot1" \
  -F "group=-1001234567890"
```

**参数说明**：

| 参数 | 位置 | 必填 | 说明 |
|------|------|------|------|
| file | form | 是 | 上传的文件 |
| bot | header/form | 否 | Bot 名称，单 bot 时不需要，多 bot 时用于指定目标 bot |
| group | form | 否* | 目标 group ID，不填使用 bot 默认值 |

*单 bot 时使用 `TELEGRAM_CHAT_ID`。多 bot 时如果不传 `group`，使用该 bot 的 `default_group`。

**成功响应** (201)：

```json
{
  "success": true,
  "provider": "telegram",
  "bot_name": "bot1",
  "group_id": "-1001234567890",
  "file_id": "AgACAgIAAxkBAAI...",
  "file_unique_id": "AQADAgAT4xgyDw...",
  "file_url": "https://api.telegram.org/file/bot<token>/docs/file_1.jpg",
  "mime_type": "image/jpeg",
  "file_size": 123456,
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

> ⚠️ **安全提示**：`file_url` 包含 Telegram bot token，仅用于调试。

**业务方需要**：
- 存储 `file_id`（用于后续获取/删除）
- 存储 `bot_name` 和 `group_id`（用于确定文件位置）
- 关联自己的业务数据（如用户 ID、项目 ID 等）

### GET /get

获取文件访问 URL。

**请求（简洁模式，Worker 自动匹配 bot）**：

```bash
curl "https://your-worker.workers.dev/get?file_id=AgACAgIAAxkBAAI..."
```

**成功响应** (200)：

```json
{
  "success": true,
  "file_id": "AgACAgIAAxkBAAI...",
  "bot_name": "bot1",
  "stream_url": "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI...",
  "mime_type": "image/jpeg",
  "file_size": 123456
}
```

**Debug 模式（额外返回 TG URL，包含 token）**：

```bash
curl "https://your-worker.workers.dev/get?file_id=AgACAgIAAxkBAAI...&debug=true"
```

**响应（Debug 模式）** (200)：

```json
{
  "success": true,
  "file_id": "AgACAgIAAxkBAAI...",
  "bot_name": "bot1",
  "stream_url": "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI...",
  "tg_url": "https://api.telegram.org/file/bot<token>/documents/file_1.jpg",
  "mime_type": "image/jpeg",
  "file_size": 123456
}
```

> ⚠️ **警告**：Debug 模式的 `tg_url` 包含 bot token，请勿在客户端直接使用。

### GET /stream

流式下载文件（不暴露 token，支持 Range 请求）。

**请求（简洁模式，Worker 自动匹配 bot）**：

```bash
curl "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI..." -o image.jpg
```

**请求（指定 bot，通过 Header，性能更好）**：

```bash
curl "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI..." \
  -H "X-Bot-Name: bot1" \
  -o image.jpg
```

**支持 Range 请求（视频分段）**：

```bash
curl "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI..." \
  -H "Range: bytes=0-1023" \
  -o first_1kb.bin
```

**响应**：
- 状态码：`200 OK` 或 `206 Partial Content`（Range 请求）
- 响应头包含：
  - `Content-Type`: 文件 MIME 类型
  - `Content-Length`: 文件大小
  - `Accept-Ranges: bytes`：声明支持 Range
  - `X-Served-By-Bot`: 实际使用的 bot 名称（调试用）
- 响应体：文件二进制数据

### DELETE /delete 或 POST /delete

删除文件（通过删除 Telegram 消息）。

**注意**：Telegram 不支持直接通过 `file_id` 删除文件，需要通过 `message_id` 删除消息。

**请求（Query 参数）**：

```bash
curl -X DELETE "https://your-worker.workers.dev/delete?message_id=123&group_id=-1001234567890&bot_name=bot1"
```

**请求（JSON Body）**：

```bash
curl -X POST https://your-worker.workers.dev/delete \
  -H "Authorization: Bearer YOUR_AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id": 123, "group_id": "-1001234567890", "bot_name": "bot1"}'
```

**成功响应** (200)：

```json
{
  "success": true,
  "bot_name": "bot1",
  "message_id": 123,
  "deleted": true
}
```

### GET /health

健康检查。

**响应**：

```json
{
  "success": true,
  "service": "firefly-media-gateway",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

## 支持的文件类型

- 图片：JPEG, PNG, WebP
- 视频：MP4, WebM, QuickTime (.mov)

## 多 Bot/Group 使用场景

| 场景 | 配置方式 |
|------|----------|
| 不同项目用不同 bot | `{"project_a": {...}, "project_b": {...}}` |
| 负载分散到多个 group | 每个请求指定不同 `group` |
| 容灾备份 | 主 bot 失败时切换到备用 bot |

## 限制

| 限制项 | 值 |
|--------|-----|
| 最大文件大小 | 50MB（Telegram 限制） |
| 免费请求次数 | 100,000 次/天 |
| 执行时间 | CPU time limit |

## 支持的文件类型

- **图片**：JPEG, PNG, WebP
- **视频**：MP4, WebM, QuickTime (.mov)

> **注意**：所有文件通过 `document` 类型上传，保持原始质量不压缩。

## 后续获取文件 URL

业务方可用 `file_id` 调用 Telegram API：

```bash
curl "https://api.telegram.org/bot<BOT_TOKEN>/getFile?file_id=<file_id>"
```

返回的 `file_path` 拼接成完整 URL：

```
https://api.telegram.org/file/bot<BOT_TOKEN>/<file_path>
```
