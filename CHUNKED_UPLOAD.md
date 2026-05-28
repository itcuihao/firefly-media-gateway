# 分片上传功能说明

## 功能概述

对于超过 50MB 的视频文件，支持自动分片上传到 Telegram，每片最大 50MB。

## 限制

| 用户类型 | 单文件限制 | 分片支持 | 总大小限制 |
|---------|-----------|---------|-----------|
| 免费用户 | 50MB | ❌ | 50MB |
| 会员用户 | 50MB | ✅ | 200MB (4×50MB) |

## API 使用

### 上传（普通模式）

```bash
# 普通用户上传（< 50MB）
curl -X POST $GO_API/api/v1/media/upload \
  -H "Authorization: Bearer $API_TOKEN" \
  -F "file=@video.mp4" \
  -F "project=test" \
  -F "usage=scene"
```

### 上传（会员分片模式）

```bash
# 会员用户上传大视频（> 50MB，自动分片）
curl -X POST $GO_API/api/v1/media/upload \
  -H "Authorization: Bearer $API_TOKEN" \
  -F "file=@large_video.mp4" \
  -F "project=test" \
  -F "usage=scene" \
  -F "member=true"
```

**响应示例**：

```json
{
  "mediaId": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "tg",
  "mimeType": "video/mp4",
  "sizeBytes": 52428800,
  "isChunked": true,
  "chunkCount": 4,
  "chunkIds": ["chunk1", "chunk2", "chunk3", "chunk4"],
  "totalBytes": 209715200,
  "status": "active",
  "createdAt": "2024-01-01T00:00:00Z"
}
```

### 获取分片流信息

```bash
# 获取资产的流信息（单文件返回 URL，分片返回多个 URL）
curl "$GO_API/api/v1/media/550e8400-e29b-41d4-a716-446655440000/stream" \
  -H "Authorization: Bearer $API_TOKEN"
```

**响应（单文件）**：

```json
{
  "isChunked": false,
  "streamUrl": "https://your-worker.workers.dev/stream?file_id=xxx",
  "totalBytes": 12345678,
  "mimeType": "video/mp4"
}
```

**响应（分片文件）**：

```json
{
  "isChunked": true,
  "totalBytes": 209715200,
  "mimeType": "video/mp4",
  "chunkCount": 4,
  "chunkUrls": [
    "https://your-worker.workers.dev/stream?file_id=chunk1",
    "https://your-worker.workers.dev/stream?file_id=chunk2",
    "https://your-worker.workers.dev/stream?file_id=chunk3",
    "https://your-worker.workers.dev/stream?file_id=chunk4"
  ]
}
```

## 工作原理

### 上传流程

```
用户上传 200MB 视频
    ↓
Go 后端检查大小
    ↓ (> 50MB 且是会员)
切分成 4 片 (每片 50MB)
    ↓
并行上传到 Telegram
    ↓
返回 asset_id + chunk_ids
```

### 下载流程

```
用户请求 stream endpoint
    ↓
Go 查询数据库：是分片文件？
    ↓
获取所有 chunk 的 stream_url
    ↓
返回 chunkUrls 数组给客户端
```

## 客户端下载示例

```javascript
// 获取流信息
const resp = await fetch(`/api/v1/media/${assetId}/stream`);
const streamInfo = await resp.json();

if (!streamInfo.isChunked) {
  // 单文件：直接下载
  window.open(streamInfo.streamUrl);
} else {
  // 分片：依次下载并拼接
  const blobs = [];
  for (const url of streamInfo.chunkUrls) {
    const resp = await fetch(url);
    blobs.push(await resp.blob());
  }
  const merged = new Blob(blobs, { type: streamInfo.mimeType });
  const url = URL.createObjectURL(merged);
  window.open(url);
}
```

## 数据库结构

```sql
-- media_assets 表新增字段
is_chunked  BOOLEAN NOT NULL DEFAULT FALSE,
chunk_count INTEGER NOT NULL DEFAULT 0,
chunk_ids   TEXT[] NOT NULL DEFAULT '{}',
total_bytes BIGINT NOT NULL DEFAULT 0
```

## 错误处理

| 错误 | 状态码 | 说明 |
|------|-------|------|
| 非会员上传 > 50MB | 413 | 需要升级为会员 |
| 视频超过 200MB | 413 | 超过最大限制 |
| 分片上传失败 | 500 | 部分分片已上传，需要重试 |
