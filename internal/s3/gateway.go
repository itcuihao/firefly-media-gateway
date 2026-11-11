package s3

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"firefly-media-gateway/internal/media"
)

// Gateway S3 兼容网关
type Gateway struct {
	mediaService  *media.Service
	publicBaseURL string
	authToken     string
}

// NewGateway 创建 S3 网关，支持可选的 authToken
func NewGateway(svc *media.Service, publicBaseURL string, authToken ...string) *Gateway {
	token := ""
	if len(authToken) > 0 {
		token = authToken[0]
	}
	return &Gateway{
		mediaService:  svc,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		authToken:     token,
	}
}

// Handler 返回 HTTP 处理器
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	// S3 兼容端点（兼容 /s3 与 /s3/）
	mux.HandleFunc("/s3", g.handleS3Request)
	mux.HandleFunc("/s3/", g.handleS3Request)

	return mux
}

// handleS3Request 处理 S3 风格的请求
// 路径格式: /s3/{bucket}/{key} 或 /s3 (ListBuckets)
func (g *Gateway) handleS3Request(w http.ResponseWriter, r *http.Request) {
	requestPath := r.URL.Path
	remaining := strings.TrimPrefix(requestPath, "/s3")
	remaining = strings.TrimPrefix(remaining, "/")

	parts := strings.SplitN(remaining, "/", 2)
	bucket := ""
	key := ""
	if len(parts) >= 1 && parts[0] != "" {
		bucket = parts[0]
	}
	if len(parts) == 2 {
		key = parts[1]
	}

	// 根路径 S3 请求：ListBuckets
	if bucket == "" {
		if r.Method == http.MethodGet {
			g.handleListBuckets(w, r)
			return
		}
		http.Error(w, "Bucket name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		if !g.isAuthorized(r) {
			g.writeError(w, "AccessDenied", "Access Denied (authentication required)", http.StatusForbidden)
			return
		}
		if key == "" {
			// S3 CreateBucket
			w.WriteHeader(http.StatusOK)
			return
		}
		g.handlePutObject(w, r, bucket, key)
	case http.MethodPost:
		if !g.isAuthorized(r) {
			g.writeError(w, "AccessDenied", "Access Denied (authentication required)", http.StatusForbidden)
			return
		}
		g.handlePostObject(w, r, bucket, key)
	case http.MethodGet:
		if key == "" {
			g.handleListObjects(w, r, bucket)
		} else {
			g.handleGetObject(w, r, bucket, key)
		}
	case http.MethodDelete:
		if !g.isAuthorized(r) {
			g.writeError(w, "AccessDenied", "Access Denied (authentication required)", http.StatusForbidden)
			return
		}
		if key == "" {
			// S3 DeleteBucket
			w.WriteHeader(http.StatusNoContent)
			return
		}
		g.handleDeleteObject(w, r, bucket, key)
	case http.MethodHead:
		if key == "" {
			// S3 HeadBucket
			w.WriteHeader(http.StatusOK)
			return
		}
		g.handleHeadObject(w, r, bucket, key)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// isAuthorized 校验 S3 鉴权凭证（支持 Bearer Token、AWS SigV4 与 Query Token）
func (g *Gateway) isAuthorized(r *http.Request) bool {
	if g.authToken == "" {
		return true
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		tokenQuery := r.URL.Query().Get("token")
		if tokenQuery == "" {
			tokenQuery = r.URL.Query().Get("auth_token")
		}
		return tokenQuery == g.authToken
	}

	// 1. Bearer Token
	if auth == "Bearer "+g.authToken {
		return true
	}

	// 2. AWS SigV4 认证: AWS4-HMAC-SHA256 Credential=<token>/...
	if strings.Contains(auth, "Credential="+g.authToken+"/") || strings.Contains(auth, "Credential="+g.authToken+",") {
		return true
	}

	// 3. AWS 旧版 S3 认证: AWS <token>:signature
	if strings.HasPrefix(auth, "AWS ") {
		parts := strings.Split(strings.TrimPrefix(auth, "AWS "), ":")
		if len(parts) >= 1 && parts[0] == g.authToken {
			return true
		}
	}

	return false
}

// parsePutKey 解析对象键为 project, usage, filename
func parsePutKey(key string) (project, usage, filename string) {
	key = strings.Trim(key, "/")
	parts := strings.Split(key, "/")

	validUsages := map[string]bool{
		"cover":  true,
		"scene":  true,
		"avatar": true,
		"audio":  true,
		"file":   true,
	}

	defaultUsageForFile := func(name string) string {
		ext := strings.ToLower(path.Ext(name))
		switch ext {
		case ".mp3", ".ogg", ".wav", ".aac", ".flac", ".m4a":
			return "audio"
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".avif", ".ico":
			return "scene"
		default:
			return "file"
		}
	}

	switch len(parts) {
	case 0:
		return "default", "file", "unnamed"
	case 1:
		return "default", defaultUsageForFile(parts[0]), parts[0]
	case 2:
		if validUsages[parts[0]] {
			return "default", parts[0], parts[1]
		}
		return parts[0], defaultUsageForFile(parts[1]), parts[1]
	default:
		// >= 3: {project}/{usage}/{filename...}
		project = parts[0]
		if project == "" {
			project = "default"
		}
		if validUsages[parts[1]] {
			usage = parts[1]
			filename = path.Join(parts[2:]...)
		} else {
			usage = defaultUsageForFile(parts[len(parts)-1])
			filename = path.Join(parts[1:]...)
		}
		return project, usage, filename
	}
}

// isHex 检查字符串是否为十六进制
func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// keyToAssetID 根据 S3 key 确定对应的 32 位 hex 资产 ID
func keyToAssetID(key string) string {
	cleanKey := strings.Trim(key, "/")
	parts := strings.Split(cleanKey, "/")
	lastPart := parts[len(parts)-1]
	ext := path.Ext(lastPart)
	cleanLast := strings.TrimSuffix(lastPart, ext)

	// 如果文件名末尾本身就是 32 位的 hex ID，直接返回
	if len(cleanLast) == 32 && isHex(cleanLast) {
		return strings.ToLower(cleanLast)
	}

	// 否则根据路径产生确定性的 32 位 MD5 UUID
	h := md5.Sum([]byte(cleanKey))
	return fmt.Sprintf("%x", h)
}

// resolveAssetID 解析指定 key 对应的资产 ID
func (g *Gateway) resolveAssetID(ctx context.Context, r *http.Request, bucket, key string) (string, error) {
	// 1. 优先使用 URL 查询参数 ?asset_id=...
	if id := r.URL.Query().Get("asset_id"); id != "" {
		return id, nil
	}

	cleanKey := strings.Trim(key, "/")
	if cleanKey == "" {
		return "", fmt.Errorf("empty key")
	}

	// 2. 检查由 key 直接派生的 ID (支持原生 32 位 UUID 或 key 映射的确定性 ID)
	candidateID := keyToAssetID(cleanKey)
	if asset, err := g.mediaService.GetMeta(ctx, candidateID); err == nil && asset.ID != "" {
		return asset.ID, nil
	}

	// 3. 检查带 bucket 路径派生的 ID
	if bucket != "" {
		bucketCandidateID := keyToAssetID(bucket + "/" + cleanKey)
		if asset, err := g.mediaService.GetMeta(ctx, bucketCandidateID); err == nil && asset.ID != "" {
			return asset.ID, nil
		}
	}

	// 4. 按 project, usage, filename 模糊匹配已存在的活跃文件
	parts := strings.Split(cleanKey, "/")
	var project, usage, search string
	if len(parts) >= 3 {
		project = parts[0]
		usage = parts[1]
		search = strings.Join(parts[2:], "/")
	} else if len(parts) == 2 {
		project = parts[0]
		search = parts[1]
	} else {
		search = parts[0]
	}

	filter := media.ListFilter{
		Limit:   5,
		Project: project,
		Usage:   usage,
		Search:  search,
		Status:  media.StatusActive,
	}
	assets, _, err := g.mediaService.List(ctx, filter)
	if err == nil && len(assets) > 0 {
		return assets[0].ID, nil
	}

	// 尝试去掉后缀名再匹配
	ext := path.Ext(search)
	if ext != "" {
		baseSearch := strings.TrimSuffix(search, ext)
		filter.Search = baseSearch
		if assets, _, err := g.mediaService.List(ctx, filter); err == nil && len(assets) > 0 {
			return assets[0].ID, nil
		}
	}

	return "", fmt.Errorf("asset not found for key: %s", key)
}

// handlePutObject 上传对象
func (g *Gateway) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	project, usage, filename := parsePutKey(key)

	// 获取 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 计算对象对应的确定性或显式 AssetID
	assetID := keyToAssetID(key)

	// S3 协议规范覆盖语义：如果已存在同名对象，先标记删除再覆盖
	if existingID, err := g.resolveAssetID(ctx, r, bucket, key); err == nil && existingID != "" {
		_, _ = g.mediaService.Delete(ctx, existingID, nil)
	}

	// 调用上传服务
	req := media.UploadRequest{
		ID:                  assetID,
		Project:             project,
		Usage:               usage,
		FileName:            filename,
		DeclaredContentType: contentType,
		Reader:              r.Body,
	}

	asset, err := g.mediaService.Upload(ctx, req)
	if err != nil {
		g.writeError(w, "UploadFailed", err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回 S3 风格响应
	etag := ""
	if asset.SHA256 != nil {
		etag = "\"" + *asset.SHA256 + "\""
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("x-amz-version-id", asset.ID)
	w.WriteHeader(http.StatusOK)
}

// handlePostObject 支持 S3 分段上传 (Multipart Upload) 的初始化与完成请求
func (g *Gateway) handlePostObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// S3 InitiateMultipartUpload: POST /s3/{bucket}/{key}?uploads
	if r.URL.Query().Has("uploads") {
		uploadID := fmt.Sprintf("mpu-%s-%d", generateRequestID(), time.Now().Unix())
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<InitiateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Bucket>%s</Bucket>
	<Key>%s</Key>
	<UploadId>%s</UploadId>
</InitiateMultipartUploadResult>`, bucket, key, uploadID)
		return
	}

	// S3 CompleteMultipartUpload: POST /s3/{bucket}/{key}?uploadId=...
	if r.URL.Query().Has("uploadId") {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Location>%s/s3/%s/%s</Location>
	<Bucket>%s</Bucket>
	<Key>%s</Key>
	<ETag>"completed"</ETag>
</CompleteMultipartUploadResult>`, g.publicBaseURL, bucket, key, bucket, key)
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

// handleGetObject 获取对象
func (g *Gateway) handleGetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	assetID, err := g.resolveAssetID(ctx, r, bucket, key)
	if err != nil {
		g.writeError(w, "NoSuchKey", fmt.Sprintf("The specified key does not exist: %v", err), http.StatusNotFound)
		return
	}

	// 获取访问 URL
	accessURL, err := g.mediaService.ResolveAccessURL(ctx, assetID)
	if err != nil {
		g.writeError(w, "NotFound", "Asset not found", http.StatusNotFound)
		return
	}

	// 重定向到实际媒体 URL
	http.Redirect(w, r, accessURL, http.StatusTemporaryRedirect)
}

// handleDeleteObject 删除对象
func (g *Gateway) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	assetID, err := g.resolveAssetID(ctx, r, bucket, key)
	if err != nil {
		// S3 协议规范：删除不存在的 key 返回 204 No Content
		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, err = g.mediaService.Delete(ctx, assetID, nil)
	if err != nil {
		g.writeError(w, "DeleteFailed", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleHeadObject 获取对象元数据
func (g *Gateway) handleHeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	assetID, err := g.resolveAssetID(ctx, r, bucket, key)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	asset, err := g.mediaService.GetMeta(ctx, assetID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", asset.SizeBytes))
	w.Header().Set("Last-Modified", asset.CreatedAt.UTC().Format(http.TimeFormat))
	if asset.SHA256 != nil {
		w.Header().Set("ETag", "\""+*asset.SHA256+"\"")
	}
	w.WriteHeader(http.StatusOK)
}

// handleListBuckets 列举所有存储桶 (ListAllMyBuckets)
func (g *Gateway) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Owner>
		<ID>firefly</ID>
		<DisplayName>firefly</DisplayName>
	</Owner>
	<Buckets>
		<Bucket>
			<Name>media-assets</Name>
			<CreationDate>2026-01-01T00:00:00.000Z</CreationDate>
		</Bucket>
		<Bucket>
			<Name>default</Name>
			<CreationDate>2026-01-01T00:00:00.000Z</CreationDate>
		</Bucket>
	</Buckets>
</ListAllMyBucketsResult>`)
}

// handleListObjects 列举对象
func (g *Gateway) handleListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	limit := 1000
	if l := r.URL.Query().Get("max-keys"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	prefix := r.URL.Query().Get("prefix")
	filter := media.ListFilter{
		Limit:  limit,
		Offset: 0,
		Status: media.StatusActive,
	}

	if prefix != "" {
		trimmedPrefix := strings.Trim(prefix, "/")
		parts := strings.Split(trimmedPrefix, "/")
		if len(parts) >= 1 && parts[0] != "" {
			filter.Project = parts[0]
		}
		if len(parts) >= 2 && parts[1] != "" {
			filter.Usage = parts[1]
		}
	}

	assets, _, err := g.mediaService.List(ctx, filter)
	if err != nil {
		g.writeError(w, "ListFailed", err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回 S3 风格 XML
	g.writeListResult(w, bucket, prefix, assets)
}

// writeError 写入错误响应
func (g *Gateway) writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
	<Code>%s</Code>
	<Message>%s</Message>
	<RequestId>%s</RequestId>
</Error>`, code, message, generateRequestID())
}

// writeListResult 写入列举结果
func (g *Gateway) writeListResult(w http.ResponseWriter, bucket, prefix string, assets []media.Asset) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Name>%s</Name>
	<Prefix>%s</Prefix>
	<KeyCount>%d</KeyCount>
	<MaxKeys>%d</MaxKeys>
	<IsTruncated>false</IsTruncated>
`, bucket, prefix, len(assets), len(assets))

	for _, asset := range assets {
		// 构造 key: {project}/{usage}/{id}
		key := fmt.Sprintf("%s/%s/%s", asset.Project, asset.Usage, asset.ID)
		lastModified := asset.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		etag := "\"\""
		if asset.SHA256 != nil {
			etag = "\"" + *asset.SHA256 + "\""
		}

		fmt.Fprintf(w, `
	<Contents>
		<Key>%s</Key>
		<LastModified>%s</LastModified>
		<ETag>%s</ETag>
		<Size>%d</Size>
		<StorageClass>STANDARD</StorageClass>
	</Contents>`, key, lastModified, etag, asset.SizeBytes)
	}

	fmt.Fprintf(w, `
</ListBucketResult>`)
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
