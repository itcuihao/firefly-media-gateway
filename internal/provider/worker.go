package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// WorkerProvider 通过 Worker API 进行文件操作
type WorkerProvider struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewWorkerProvider 创建 Worker provider
func NewWorkerProvider(baseURL, authToken string) *WorkerProvider {
	return &WorkerProvider{
		baseURL:   baseURL,
		authToken: authToken,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *WorkerProvider) Name() string {
	return "worker"
}

// Upload 上传文件到 Worker
func (p *WorkerProvider) Upload(ctx context.Context, in UploadInput) (UploadResult, error) {
	// 准备表单数据
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// 添加文件
	part, err := mw.CreateFormFile("file", in.FileName)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, in.Reader); err != nil {
		return UploadResult{}, fmt.Errorf("copy file body: %w", err)
	}

	if err := mw.Close(); err != nil {
		return UploadResult{}, fmt.Errorf("close multipart writer: %w", err)
	}

	// 构建请求
	reqURL := p.baseURL + "/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.authToken)

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("call Worker upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UploadResult{}, fmt.Errorf("Worker upload status=%d body=%s", resp.StatusCode, string(b))
	}

	// 解析响应
	var result workerUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return UploadResult{}, fmt.Errorf("decode upload response: %w", err)
	}
	if !result.Success {
		return UploadResult{}, fmt.Errorf("Worker upload failed: %s", result.Error)
	}

	return UploadResult{
		ProviderFileID:       result.FileID,
		ProviderBucketOrChat: &result.GroupID,
	}, nil
}

// Delete 删除文件（通过 Worker）
func (p *WorkerProvider) Delete(ctx context.Context, providerFileID string, bucketOrChat *string) error {
	// 注意：Worker 需要消息 ID 而非 file_id
	// 这里我们无法直接删除，因为缺少 message_id
	// 返回 nil 表示已标记删除（业务方在 DB 中标记）
	return nil
}

// GetAccessURL 获取文件访问 URL（使用 Worker /get 端点）
func (p *WorkerProvider) GetAccessURL(ctx context.Context, providerFileID string, _ *string) (string, error) {
	// 调用 Worker 的 /get 端点获取 stream_url
	params := url.Values{}
	params.Set("file_id", providerFileID)

	reqURL := fmt.Sprintf("%s/get?%s", p.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("build get request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.authToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Worker get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("Worker get status=%d body=%s", resp.StatusCode, string(b))
	}

	var result workerGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode get response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("Worker get failed: %s", result.Error)
	}

	// 返回 Worker stream URL（不包含 TG token）
	return result.StreamURL, nil
}

type workerUploadResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	FileID  string `json:"file_id"`
	GroupID string `json:"group_id"`
}

type workerGetResponse struct {
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	FileID   string `json:"file_id"`
	StreamURL string `json:"stream_url"`
	MimeType string `json:"mime_type"`
	FileSize int    `json:"file_size"`
}
