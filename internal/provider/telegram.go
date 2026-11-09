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
	"path"
	"sync"
	"time"
)

const telegramAccessCacheTTL = 45 * time.Minute

type cachedTelegramAccess struct {
	url       string
	expiresAt time.Time
}

type TelegramProvider struct {
	botToken    string
	chatID      string
	client      *http.Client
	cacheMu     sync.RWMutex
	accessCache map[string]cachedTelegramAccess
}

// NewTelegramProvider 创建 Telegram provider（单 bot）
func NewTelegramProvider(botToken, chatID string, timeout time.Duration) *TelegramProvider {
	if timeout < 180*time.Second {
		timeout = 180 * time.Second
	}
	return &TelegramProvider{
		botToken:    botToken,
		chatID:      chatID,
		client:      &http.Client{Timeout: timeout},
		accessCache: make(map[string]cachedTelegramAccess),
	}
}

// NewTelegramProviderWithConfig 创建 Telegram provider（支持多 bot）
func NewTelegramProviderWithConfig(botToken, defaultGroup string, timeout time.Duration) *TelegramProvider {
	if timeout < 180*time.Second {
		timeout = 180 * time.Second
	}
	return &TelegramProvider{
		botToken:    botToken,
		chatID:      defaultGroup, // 使用 default_group 作为 chat_id
		client:      &http.Client{Timeout: timeout},
		accessCache: make(map[string]cachedTelegramAccess),
	}
}

func (p *TelegramProvider) Name() string {
	return "tg"
}

func (p *TelegramProvider) Upload(ctx context.Context, in UploadInput) (UploadResult, error) {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", p.botToken)

	// Check if in.Reader supports seeking for zero-copy streaming multipart upload
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
		contentType string
		headerBytes []byte
		footerBytes []byte
		reqBytes    []byte
	)

	if canSeek {
		var headerBuf bytes.Buffer
		mw := multipart.NewWriter(&headerBuf)
		if err := mw.WriteField("chat_id", p.chatID); err != nil {
			return UploadResult{}, fmt.Errorf("write chat_id field: %w", err)
		}
		if _, err := mw.CreateFormFile("document", in.FileName); err != nil {
			return UploadResult{}, fmt.Errorf("create form file: %w", err)
		}
		contentType = mw.FormDataContentType()
		headerBytes = headerBuf.Bytes()
		footerBytes = []byte(fmt.Sprintf("\r\n--%s--\r\n", mw.Boundary()))
	} else {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		if err := mw.WriteField("chat_id", p.chatID); err != nil {
			return UploadResult{}, fmt.Errorf("write chat_id field: %w", err)
		}
		part, err := mw.CreateFormFile("document", in.FileName)
		if err != nil {
			return UploadResult{}, fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, in.Reader); err != nil {
			return UploadResult{}, fmt.Errorf("copy file body: %w", err)
		}
		if err := mw.Close(); err != nil {
			return UploadResult{}, fmt.Errorf("close multipart writer: %w", err)
		}
		contentType = mw.FormDataContentType()
		reqBytes = body.Bytes()
	}

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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bodyReader)
		if err != nil {
			return UploadResult{}, fmt.Errorf("build sendDocument request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.ContentLength = contentLen

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call sendDocument: %w", err)
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}

		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			var tgErr sendDocumentResponse
			_ = json.Unmarshal(b, &tgErr)
			waitSec := 3
			if tgErr.Parameters != nil && tgErr.Parameters.RetryAfter > 0 {
				waitSec = tgErr.Parameters.RetryAfter
			}
			lastErr = fmt.Errorf("telegram rate limit (429): retry after %d seconds", waitSec)
			select {
			case <-ctx.Done():
				return UploadResult{}, ctx.Err()
			case <-time.After(time.Duration(waitSec)*time.Second + 200*time.Millisecond):
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("sendDocument status=%d body=%s", resp.StatusCode, string(b))
			if resp.StatusCode >= 500 && attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 1 * time.Second)
				continue
			}
			return UploadResult{}, lastErr
		}

		var result sendDocumentResponse
		if err := json.Unmarshal(b, &result); err != nil {
			return UploadResult{}, fmt.Errorf("decode sendDocument response: %w", err)
		}
		if !result.OK || result.Result.Document.FileID == "" {
			return UploadResult{}, fmt.Errorf("sendDocument failed: %s", result.Description)
		}

		chat := p.chatID
		return UploadResult{
			ProviderFileID:       result.Result.Document.FileID,
			ProviderBucketOrChat: &chat,
		}, nil
	}

	return UploadResult{}, lastErr
}

func (p *TelegramProvider) Delete(_ context.Context, _ string, _ *string) error {
	// Telegram Bot API does not provide direct file deletion by file_id.
	// We keep metadata status as deleted in DB for MVP.
	return nil
}

func (p *TelegramProvider) GetAccess(ctx context.Context, providerFileID string, _ *string) (AccessResult, error) {
	now := time.Now()
	p.cacheMu.RLock()
	if cached, ok := p.accessCache[providerFileID]; ok && now.Before(cached.expiresAt) {
		p.cacheMu.RUnlock()
		return AccessResult{URL: cached.url}, nil
	}
	p.cacheMu.RUnlock()

	payload := url.Values{}
	payload.Set("file_id", providerFileID)
	u := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?%s", p.botToken, payload.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return AccessResult{}, fmt.Errorf("build getFile request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return AccessResult{}, fmt.Errorf("call getFile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return AccessResult{}, fmt.Errorf("getFile status=%d body=%s", resp.StatusCode, string(b))
	}

	var result getFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AccessResult{}, fmt.Errorf("decode getFile response: %w", err)
	}
	if !result.OK || result.Result.FilePath == "" {
		return AccessResult{}, fmt.Errorf("getFile failed: %s", result.Description)
	}

	resolvedURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", p.botToken, path.Clean(result.Result.FilePath))
	p.cacheMu.Lock()
	if p.accessCache == nil {
		p.accessCache = make(map[string]cachedTelegramAccess)
	}
	p.accessCache[providerFileID] = cachedTelegramAccess{
		url:       resolvedURL,
		expiresAt: time.Now().Add(telegramAccessCacheTTL),
	}
	p.cacheMu.Unlock()

	return AccessResult{
		URL: resolvedURL,
	}, nil
}

type sendDocumentResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters,omitempty"`
	Result struct {
		Document struct {
			FileID string `json:"file_id"`
		} `json:"document"`
	} `json:"result"`
}

type getFileResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		FilePath string `json:"file_path"`
	} `json:"result"`
}
