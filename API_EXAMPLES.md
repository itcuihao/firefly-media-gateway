# Firefly Media Gateway - API 测试示例

## 环境变量

```bash
# Worker API
WORKER_URL="https://your-worker.workers.dev"
WORKER_TOKEN="your_worker_auth_token"

# Go API
GO_API="https://api.example.com"
API_TOKEN="your_api_token"

# 测试文件
TEST_IMAGE="test.jpg"
TEST_VIDEO="test.mp4"
```

---

## Worker API 测试

### 1. 健康检查

```bash
curl $WORKER_URL/health

# 响应
{
  "success": true,
  "service": "firefly-media-gateway",
  "version": "1.0.0",
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

### 2. 上传文件

```bash
# 使用 Form 数据
curl -X POST $WORKER_URL/upload \
  -H "Authorization: Bearer $WORKER_TOKEN" \
  -F "file=@$TEST_IMAGE" \
  -F "bot=bot1" \
  -F "group=-1001234567890"

# 响应
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

> ⚠️ `file_url` 包含 TG token，仅用于调试。

### 3. 获取访问 URL

```bash
# 正常模式（返回安全的 stream_url）
curl "$WORKER_URL/get?file_id=AgACAgIAAxkBAAI..."

# 响应
{
  "success": true,
  "file_id": "AgACAgIAAxkBAAI...",
  "bot_name": "bot1",
  "stream_url": "https://your-worker.workers.dev/stream?file_id=AgACAgIAAxkBAAI...",
  "mime_type": "image/jpeg",
  "file_size": 123456
}

# Debug 模式（额外返回 tg_url，包含 token）
curl "$WORKER_URL/get?file_id=AgACAgIAAxkBAAI...&debug=true"

# 响应
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

> ⚠️ Debug 模式的 `tg_url` 包含 bot token，请勿在客户端直接使用。

### 4. 流式下载文件（支持 Range）

```bash
# 简洁模式（Worker 自动匹配 bot）
curl "$WORKER_URL/stream?file_id=AgACAgIAAxkBAAI..." -o image.jpg

# 指定 bot（通过 Header，性能更好）
curl "$WORKER_URL/stream?file_id=AgACAgIAAxkBAAI..." \
  -H "X-Bot-Name: bot1" \
  -o image.jpg

# Range 请求（视频分段）
curl "$WORKER_URL/stream?file_id=AgACAgIAAxkBAAI..." \
  -H "Range: bytes=0-1023" \
  -o first_1kb.bin

# 响应头
HTTP/1.1 206 Partial Content
Content-Type: image/jpeg
Content-Length: 1024
Accept-Ranges: bytes
X-Served-By-Bot: bot1
```

### 5. 删除文件

```bash
# DELETE 方法
curl -X DELETE "$WORKER_URL/delete?message_id=123&group_id=-1001234567890&bot_name=bot1"

# POST 方法（JSON）
curl -X POST $WORKER_URL/delete \
  -H "Authorization: Bearer $WORKER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_id": 123, "group_id": "-1001234567890", "bot_name": "bot1"}'

# 响应
{
  "success": true,
  "bot_name": "bot1",
  "message_id": 123,
  "deleted": true
}
```

---

## Go REST API 测试

### 1. 健康检查

```bash
curl $GO_API/health

# 响应
{
  "status": "ok",
  "storage_mode": "proxy",
  "providers": ["tg", "worker"]
}
```

### 2. 上传文件

```bash
curl -X POST $GO_API/api/v1/media \
  -H "Authorization: Bearer $API_TOKEN" \
  -F "file=@$TEST_IMAGE" \
  -F "project=test-project" \
  -F "usage=cover" \
  -F "filename=test-image.jpg"

# 响应
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "tg",
  "provider_file_id": "AgACAgIAAxkBAAI...",
  "public_url": "https://api.example.com/api/v1/media/550e8400-e29b-41d4-a716-446655440000",
  "mime_type": "image/jpeg",
  "size_bytes": 123456,
  "sha256": "a1b2c3d4e5f6...",
  "project": "test-project",
  "usage": "cover",
  "status": "active",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### 3. 获取文件元数据

```bash
curl $GO_API/api/v1/media/550e8400-e29b-41d4-a716-446655440000

# 响应
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "tg",
  "public_url": "https://api.example.com/api/v1/media/550e8400-e29b-41d4-a716-446655440000",
  "mime_type": "image/jpeg",
  "size_bytes": 123456,
  "sha256": "a1b2c3d4e5f6...",
  "project": "test-project",
  "usage": "cover",
  "status": "active",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### 4. 访问文件

```bash
curl -L $GO_API/api/v1/media/550e8400-e29b-41d4-a716-446655440000

# 响应：重定向到实际文件 URL 或流式返回
```

### 5. 列举文件

```bash
curl "$GO_API/api/v1/media?limit=10&offset=0"

# 响应
{
  "total": 100,
  "limit": 10,
  "offset": 0,
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "project": "test-project",
      "usage": "cover",
      "mime_type": "image/jpeg",
      "size_bytes": 123456,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

### 6. 删除文件

```bash
curl -X DELETE $GO_API/api/v1/media/550e8400-e29b-41d4-a716-446655440000

# 响应
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "deleted",
  "deleted_at": "2024-01-01T00:05:00Z"
}
```

---

## S3 兼容 API 测试

### 1. 上传文件（PutObject）

```bash
# S3 路径格式: /s3/{bucket}/{project}/{usage}/{filename}
curl -X PUT "$GO_API/s3/media-assets/test-project/cover/test.jpg" \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: image/jpeg" \
  --data-binary @$TEST_IMAGE

# 响应
# HTTP/1.1 200 OK
# ETag: "a1b2c3d4e5f6..."
```

### 2. 获取文件（GetObject）

```bash
# 通过 asset_id 获取
curl "$GO_API/s3/media-assets?asset_id=550e8400-e29b-41d4-a716-446655440000"

# 响应：302 重定向到实际文件 URL
```

### 3. 列举对象（ListObjects）

```bash
curl "$GO_API/s3/media-assets?max-keys=10"

# 响应 (XML)
<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
    <Name>media-assets</Name>
    <Prefix></Prefix>
    <KeyCount>2</KeyCount>
    <MaxKeys>10</MaxKeys>
    <IsTruncated>false</IsTruncated>
    <Contents>
        <Key>test-project/cover/550e8400-e29b-41d4-a716-446655440000</Key>
        <LastModified>2024-01-01T00:00:00.000Z</LastModified>
        <ETag>"a1b2c3d4e5f6..."</ETag>
        <Size>123456</Size>
        <StorageClass>STANDARD</StorageClass>
    </Contents>
</ListBucketResult>
```

### 4. 删除对象（DeleteObject）

```bash
curl -X DELETE "$GO_API/s3/media-assets?asset_id=550e8400-e29b-41d4-a716-446655440000"

# 响应
# HTTP/1.1 204 No Content
```

### 5. 获取元数据（HeadObject）

```bash
curl -I "$GO_API/s3/media-assets?asset_id=550e8400-e29b-41d4-a716-446655440000"

# 响应
# HTTP/1.1 200 OK
# Content-Type: image/jpeg
# Content-Length: 123456
# Last-Modified: Tue, 01 Jan 2024 00:00:00 GMT
# ETag: "a1b2c3d4e5f6..."
```

---

## AWS CLI 测试

```bash
# 配置 AWS CLI
aws configure set endpoint.url $GO_API
aws configure set region auto

# 上传
aws s3 cp $TEST_IMAGE s3://media-assets/test-project/cover/test.jpg

# 下载
aws s3 cp s3://media-assets/test-project/cover/test.jpg downloaded.jpg

# 列举
aws s3 ls s3://media-assets/test-project/cover/

# 删除
aws s3 rm s3://media-assets/test-project/cover/test.jpg
```

---

## Python SDK 测试

```python
import boto3

# 配置 S3 客户端
s3 = boto3.client(
    's3',
    endpoint_url='https://api.example.com',
    region_name='auto',
)

# 上传
with open('test.jpg', 'rb') as f:
    s3.put_object(
        Bucket='media-assets',
        Key='test-project/cover/test.jpg',
        Body=f,
        ContentType='image/jpeg'
    )

# 列举
response = s3.list_objects_v2(
    Bucket='media-assets',
    MaxKeys=10
)
for obj in response.get('Contents', []):
    print(obj['Key'], obj['Size'])

# 删除
s3.delete_object(
    Bucket='media-assets',
    Key='test-project/cover/test.jpg'
)
```

---

## 错误响应示例

### 认证失败

```json
{
  "success": false,
  "error": "Unauthorized",
  "code": "UNAUTHORIZED"
}
```

### 文件不存在

```json
{
  "success": false,
  "error": "Asset not found",
  "code": "NOT_FOUND"
}
```

### 文件过大

```json
{
  "success": false,
  "error": "File size exceeds 52428800 bytes",
  "code": "FILE_TOO_LARGE"
}
```

### 无效的文件类型

```json
{
  "success": false,
  "error": "Invalid MIME type: application/x-msdownload",
  "code": "INVALID_MIME"
}
```
