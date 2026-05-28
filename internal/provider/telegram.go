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
	"time"
)

type TelegramProvider struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramProvider 创建 Telegram provider（单 bot）
func NewTelegramProvider(botToken, chatID string, timeout time.Duration) *TelegramProvider {
	return &TelegramProvider{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: timeout},
	}
}

// NewTelegramProviderWithConfig 创建 Telegram provider（支持多 bot）
func NewTelegramProviderWithConfig(botToken, defaultGroup string, timeout time.Duration) *TelegramProvider {
	return &TelegramProvider{
		botToken: botToken,
		chatID:   defaultGroup, // 使用 default_group 作为 chat_id
		client:   &http.Client{Timeout: timeout},
	}
}

func (p *TelegramProvider) Name() string {
	return "tg"
}

func (p *TelegramProvider) Upload(ctx context.Context, in UploadInput) (UploadResult, error) {
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

	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", p.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("build sendDocument request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("call sendDocument: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UploadResult{}, fmt.Errorf("sendDocument status=%d body=%s", resp.StatusCode, string(b))
	}

	var result sendDocumentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

func (p *TelegramProvider) Delete(_ context.Context, _ string, _ *string) error {
	// Telegram Bot API does not provide direct file deletion by file_id.
	// We keep metadata status as deleted in DB for MVP.
	return nil
}

func (p *TelegramProvider) GetAccessURL(ctx context.Context, providerFileID string, _ *string) (string, error) {
	payload := url.Values{}
	payload.Set("file_id", providerFileID)
	u := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?%s", p.botToken, payload.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("build getFile request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call getFile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("getFile status=%d body=%s", resp.StatusCode, string(b))
	}

	var result getFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode getFile response: %w", err)
	}
	if !result.OK || result.Result.FilePath == "" {
		return "", fmt.Errorf("getFile failed: %s", result.Description)
	}

	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", p.botToken, path.Clean(result.Result.FilePath)), nil
}

type sendDocumentResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
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
