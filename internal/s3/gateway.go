package s3

import (
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
}

// NewGateway 创建 S3 网关
func NewGateway(svc *media.Service, publicBaseURL string) *Gateway {
	return &Gateway{
		mediaService:  svc,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// Handler 返回 HTTP 处理器
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	// S3 兼容端点
	mux.HandleFunc("/s3/", g.handleS3Request)

	return mux
}

// handleS3Request 处理 S3 风格的请求
// 路径格式: /s3/{bucket}/{key}
func (g *Gateway) handleS3Request(w http.ResponseWriter, r *http.Request) {
	// 解析 bucket 和 key
	// /s3/media-assets/user123/avatar.jpg
	// bucket = media-assets
	// key = user123/avatar.jpg

	prefix := "/s3/"
	requestPath := r.URL.Path

	if !strings.HasPrefix(requestPath, prefix) {
		http.Error(w, "Invalid S3 path", http.StatusBadRequest)
		return
	}

	remaining := strings.TrimPrefix(requestPath, prefix)
	parts := strings.SplitN(remaining, "/", 2)

	if len(parts) < 1 {
		http.Error(w, "Bucket name required", http.StatusBadRequest)
		return
	}

	bucket := parts[0]
	key := ""
	if len(parts) == 2 {
		key = parts[1]
	}

	switch r.Method {
	case http.MethodPut:
		if key == "" {
			http.Error(w, "Object key required", http.StatusBadRequest)
			return
		}
		g.handlePutObject(w, r, bucket, key)
	case http.MethodGet:
		if key == "" {
			g.handleListObjects(w, r, bucket)
		} else {
			g.handleGetObject(w, r, bucket, key)
		}
	case http.MethodDelete:
		if key == "" {
			http.Error(w, "Object key required", http.StatusBadRequest)
			return
		}
		g.handleDeleteObject(w, r, bucket, key)
	case http.MethodHead:
		if key == "" {
			http.Error(w, "Object key required", http.StatusBadRequest)
			return
		}
		g.handleHeadObject(w, r, bucket, key)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// PutObject 上传对象
func (g *Gateway) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// 从 key 解析参数
	// 格式: {project}/{usage}/{filename}
	// 例如: project1/scene/image.jpg
	keyParts := strings.Split(key, "/")
	if len(keyParts) < 3 {
		g.writeError(w, "InvalidKey", "Key format must be {project}/{usage}/{filename}", http.StatusBadRequest)
		return
	}

	project := keyParts[0]
	usage := keyParts[1]
	filename := path.Join(keyParts[2:]...)

	// 验证 usage
	if usage != "cover" && usage != "scene" {
		g.writeError(w, "InvalidUsage", "Usage must be 'cover' or 'scene'", http.StatusBadRequest)
		return
	}

	// 获取 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 调用上传服务
	req := media.UploadRequest{
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
	w.WriteHeader(http.StatusOK)
}

// GetObject 获取对象
func (g *Gateway) handleGetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// 从 key 解析 asset_id
	// 假设 key 包含 asset_id 或通过查询参数
	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		// 尝试从 key 获取（这里简化处理）
		g.writeError(w, "MissingAssetID", "asset_id query parameter required", http.StatusBadRequest)
		return
	}

	// 获取访问 URL
	accessURL, err := g.mediaService.ResolveAccessURL(ctx, assetID)
	if err != nil {
		g.writeError(w, "NotFound", "Asset not found", http.StatusNotFound)
		return
	}

	// 重定向到实际 URL
	http.Redirect(w, r, accessURL, http.StatusTemporaryRedirect)
}

// DeleteObject 删除对象
func (g *Gateway) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		g.writeError(w, "MissingAssetID", "asset_id query parameter required", http.StatusBadRequest)
		return
	}

	_, err := g.mediaService.Delete(ctx, assetID, nil)
	if err != nil {
		g.writeError(w, "DeleteFailed", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HeadObject 获取对象元数据
func (g *Gateway) handleHeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		g.writeError(w, "MissingAssetID", "asset_id query parameter required", http.StatusBadRequest)
		return
	}

	asset, err := g.mediaService.GetMeta(ctx, assetID)
	if err != nil {
		g.writeError(w, "NotFound", "Asset not found", http.StatusNotFound)
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

// ListObjects 列举对象
func (g *Gateway) handleListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// 解析查询参数
	limit := 1000
	if l := r.URL.Query().Get("max-keys"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// 调用列举服务
	assets, err := g.mediaService.List(ctx, limit, 0)
	if err != nil {
		g.writeError(w, "ListFailed", err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回 S3 风格 XML
	g.writeListResult(w, bucket, assets)
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
func (g *Gateway) writeListResult(w http.ResponseWriter, bucket string, assets []media.Asset) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Name>%s</Name>
	<Prefix></Prefix>
	<KeyCount>%d</KeyCount>
	<MaxKeys>%d</MaxKeys>
	<IsTruncated>false</IsTruncated>
`, bucket, len(assets), len(assets))

	for _, asset := range assets {
		// 构造 key: project/usage/asset_id
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

// parseKeyFromAssetID 从 asset_id 解析 key
func parseKeyFromAssetID(asset *media.Asset) string {
	return fmt.Sprintf("%s/%s/%s", asset.Project, asset.Usage, asset.ID)
}
