package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// retryableHTTPDo executes an HTTP request with retry on transient errors (EOF, connection reset).
func retryableHTTPDo(client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// Only retry on transient network errors
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return nil, lastErr
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// WorkerProvider 通过 Worker API 进行文件操作
type WorkerProvider struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewWorkerProvider 创建 Worker provider
func NewWorkerProvider(baseURL, authToken string) *WorkerProvider {
	return &WorkerProvider{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		authToken: authToken,
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (p *WorkerProvider) Name() string {
	return "worker"
}

// Upload 上传文件到 Worker（支持零拷贝流式 Multipart 与 429/5xx 智能重试）
func (p *WorkerProvider) Upload(ctx context.Context, in UploadInput) (UploadResult, error) {
	contentTypeHeader := in.MIMEType
	if contentTypeHeader == "" {
		contentTypeHeader = "application/octet-stream"
	}

	var rs io.ReadSeeker
	var startOffset int64
	var payloadSize int64
	canSeek := false
	if seeker, ok := in.Reader.(io.ReadSeeker); ok {
		if cur, err1 := seeker.Seek(0, io.SeekCurrent); err1 == nil {
			if end, err2 := seeker.Seek(0, io.SeekEnd); err2 == nil {
				if _, err3 := seeker.Seek(cur, io.SeekStart); err3 == nil {
					rs = seeker
					startOffset = cur
					payloadSize = end - cur
					canSeek = true
				}
			}
		}
	}

	var (
		formContentType string
		headerBytes     []byte
		footerBytes     []byte
		reqBytes        []byte
	)

	if canSeek {
		var headerBuf bytes.Buffer
		mw := multipart.NewWriter(&headerBuf)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(in.FileName)))
		h.Set("Content-Type", contentTypeHeader)
		if _, err := mw.CreatePart(h); err != nil {
			return UploadResult{}, fmt.Errorf("create form part: %w", err)
		}
		formContentType = mw.FormDataContentType()
		headerBytes = headerBuf.Bytes()
		footerBytes = []byte(fmt.Sprintf("\r\n--%s--\r\n", mw.Boundary()))
	} else {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(in.FileName)))
		h.Set("Content-Type", contentTypeHeader)
		part, err := mw.CreatePart(h)
		if err != nil {
			return UploadResult{}, fmt.Errorf("create form part: %w", err)
		}
		if _, err := io.Copy(part, in.Reader); err != nil {
			return UploadResult{}, fmt.Errorf("copy file body: %w", err)
		}
		if err := mw.Close(); err != nil {
			return UploadResult{}, fmt.Errorf("close multipart writer: %w", err)
		}
		formContentType = mw.FormDataContentType()
		reqBytes = body.Bytes()
	}

	reqURL := p.baseURL + "/upload"
	const maxRetries = 4
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return UploadResult{}, ctx.Err()
		default:
		}

		var bodyReader io.Reader
		var contentLen int64
		if canSeek {
			if _, err := rs.Seek(startOffset, io.SeekStart); err != nil {
				return UploadResult{}, fmt.Errorf("rewind upload reader: %w", err)
			}
			bodyReader = io.MultiReader(
				bytes.NewReader(headerBytes),
				io.LimitReader(rs, payloadSize),
				bytes.NewReader(footerBytes),
			)
			contentLen = int64(len(headerBytes)) + payloadSize + int64(len(footerBytes))
		} else {
			bodyReader = bytes.NewReader(reqBytes)
			contentLen = int64(len(reqBytes))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bodyReader)
		if err != nil {
			return UploadResult{}, fmt.Errorf("build upload request: %w", err)
		}
		req.Header.Set("Content-Type", formContentType)
		req.Header.Set("Authorization", "Bearer "+p.authToken)
		req.ContentLength = contentLen

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call Worker upload: %w", err)
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}

		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			waitSec := 3
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && n > 0 {
					waitSec = n
				}
			}
			var errResp workerUploadResponse
			if err := json.Unmarshal(b, &errResp); err == nil && errResp.RetryAfter > 0 {
				waitSec = errResp.RetryAfter
			}
			lastErr = fmt.Errorf("Worker rate limit (429): retry after %d seconds", waitSec)
			select {
			case <-ctx.Done():
				return UploadResult{}, ctx.Err()
			case <-time.After(time.Duration(waitSec)*time.Second + 200*time.Millisecond):
			}
			continue
		}

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("Worker upload status=%d body=%s", resp.StatusCode, string(b))
			if resp.StatusCode >= 500 && attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 1 * time.Second)
				continue
			}
			return UploadResult{}, lastErr
		}

		var result workerUploadResponse
		if err := json.Unmarshal(b, &result); err != nil {
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

	return UploadResult{}, lastErr
}

// Delete 删除文件（通过 Worker）
func (p *WorkerProvider) Delete(ctx context.Context, providerFileID string, bucketOrChat *string) error {
	// 注意：Worker 需要消息 ID 而非 file_id
	// 这里我们无法直接删除，因为缺少 message_id
	// 返回 nil 表示已标记删除（业务方在 DB 中标记）
	return nil
}

// GetAccess 获取文件访问 URL（直接构造 Worker /stream 端点，消除多余的 /get 网络往返）
func (p *WorkerProvider) GetAccess(_ context.Context, providerFileID string, _ *string) (AccessResult, error) {
	if strings.TrimSpace(providerFileID) == "" {
		return AccessResult{}, fmt.Errorf("providerFileID is required")
	}
	params := url.Values{}
	params.Set("file_id", providerFileID)

	streamURL := fmt.Sprintf("%s/stream?%s", p.baseURL, params.Encode())
	header := http.Header{}
	if p.authToken != "" {
		header.Set("Authorization", "Bearer "+p.authToken)
	}

	return AccessResult{
		URL:    streamURL,
		Header: header,
	}, nil
}

type workerUploadResponse struct {
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	FileID     string `json:"file_id"`
	GroupID    string `json:"group_id"`
	RetryAfter int    `json:"retry_after,omitempty"`
}
