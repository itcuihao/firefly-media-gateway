package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"firefly-media-gateway/internal/provider"
)

const (
	maxImageSizeBytes int64 = 10 * 1024 * 1024
	maxVideoSizeBytes int64 = 120 * 1024 * 1024
)

type UploadRequest struct {
	Project             string
	Usage               string
	FileName            string
	DeclaredContentType string
	Reader              io.Reader
}

type Service struct {
	repo            Repository
	providers       map[string]provider.StorageProvider
	defaultProvider string
	publicBaseURL   string
}

func NewService(repo Repository, providers map[string]provider.StorageProvider, defaultProvider, publicBaseURL string) *Service {
	return &Service{
		repo:            repo,
		providers:       providers,
		defaultProvider: defaultProvider,
		publicBaseURL:   strings.TrimRight(publicBaseURL, "/"),
	}
}

func (s *Service) Upload(ctx context.Context, req UploadRequest) (Asset, error) {
	if strings.TrimSpace(req.Project) == "" {
		return Asset{}, fmt.Errorf("project is required")
	}
	if req.Usage != "cover" && req.Usage != "scene" {
		return Asset{}, fmt.Errorf("usage must be cover or scene")
	}
	if strings.TrimSpace(req.FileName) == "" {
		return Asset{}, fmt.Errorf("file name is required")
	}

	p, ok := s.providers[s.defaultProvider]
	if !ok {
		return Asset{}, fmt.Errorf("provider %q: %w", s.defaultProvider, ErrProviderDisabled)
	}

	tmpFile, sizeBytes, shaHex, sniff, err := persistUpload(req.Reader)
	if err != nil {
		return Asset{}, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	mimeType, mediaKind, err := normalizeAndValidateMIME(req.FileName, req.DeclaredContentType, sniff)
	if err != nil {
		return Asset{}, err
	}
	if mediaKind == "image" && sizeBytes > maxImageSizeBytes {
		return Asset{}, fmt.Errorf("image exceeds %d bytes: %w", maxImageSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "video" && sizeBytes > maxVideoSizeBytes {
		return Asset{}, fmt.Errorf("video exceeds %d bytes: %w", maxVideoSizeBytes, ErrFileTooLarge)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return Asset{}, fmt.Errorf("rewind temp file: %w", err)
	}

	upResult, err := p.Upload(ctx, provider.UploadInput{
		FileName: req.FileName,
		MIMEType: mimeType,
		Reader:   tmpFile,
	})
	if err != nil {
		return Asset{}, fmt.Errorf("upload to provider %q failed: %w", p.Name(), err)
	}

	assetID := newUUID()
	publicURL := fmt.Sprintf("%s/api/v1/media/%s", s.publicBaseURL, assetID)

	sha := shaHex
	asset, err := s.repo.Create(ctx, CreateAssetInput{
		ID:                   assetID,
		Provider:             p.Name(),
		ProviderFileID:       upResult.ProviderFileID,
		ProviderBucketOrChat: upResult.ProviderBucketOrChat,
		PublicURL:            publicURL,
		MIMEType:             mimeType,
		SizeBytes:            sizeBytes,
		SHA256:               &sha,
		Project:              strings.TrimSpace(req.Project),
		Usage:                req.Usage,
	})
	if err != nil {
		return Asset{}, fmt.Errorf("save media metadata failed: %w", err)
	}

	return asset, nil
}

func (s *Service) GetMeta(ctx context.Context, id string) (Asset, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ResolveAccessURL(ctx context.Context, id string) (string, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if asset.Status != StatusActive {
		return "", ErrNotFound
	}
	p, ok := s.providers[asset.Provider]
	if !ok {
		return "", fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
	}
	return p.GetAccessURL(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat)
}

func (s *Service) Delete(ctx context.Context, id string) (Asset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status == StatusDeleted {
		return asset, nil
	}

	p, ok := s.providers[asset.Provider]
	if !ok {
		return Asset{}, fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
	}
	if err := p.Delete(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat); err != nil {
		return Asset{}, fmt.Errorf("delete from provider %q failed: %w", p.Name(), err)
	}

	return s.repo.MarkDeleted(ctx, id)
}

func persistUpload(src io.Reader) (*os.File, int64, string, []byte, error) {
	tmpFile, err := os.CreateTemp("", "media-upload-*")
	if err != nil {
		return nil, 0, "", nil, fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()
	buf := make([]byte, 32*1024)
	sniff := make([]byte, 0, 512)
	var total int64

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			total += int64(n)
			if total > maxVideoSizeBytes {
				tmpFile.Close()
				_ = os.Remove(tmpFile.Name())
				return nil, 0, "", nil, ErrFileTooLarge
			}
			if len(sniff) < 512 {
				need := 512 - len(sniff)
				if need > n {
					need = n
				}
				sniff = append(sniff, chunk[:need]...)
			}
			if _, err := h.Write(chunk); err != nil {
				tmpFile.Close()
				_ = os.Remove(tmpFile.Name())
				return nil, 0, "", nil, fmt.Errorf("update sha256: %w", err)
			}
			if _, err := tmpFile.Write(chunk); err != nil {
				tmpFile.Close()
				_ = os.Remove(tmpFile.Name())
				return nil, 0, "", nil, fmt.Errorf("write temp file: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, 0, "", nil, fmt.Errorf("read upload stream: %w", readErr)
		}
	}

	return tmpFile, total, hex.EncodeToString(h.Sum(nil)), sniff, nil
}

func normalizeAndValidateMIME(fileName, declared string, sniff []byte) (mimeType string, mediaKind string, err error) {
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(sniff)))
	declared = strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	fromExt := mimeByExt(fileName)

	candidates := []string{detected, declared, fromExt}
	for _, c := range candidates {
		if kind := mediaKindByMIME(c); kind != "" {
			return c, kind, nil
		}
	}

	return "", "", ErrInvalidFileType
}

func mimeByExt(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	default:
		return ""
	}
}

func mediaKindByMIME(m string) string {
	switch m {
	case "image/jpeg", "image/png", "image/webp":
		return "image"
	case "video/mp4", "video/webm", "video/quicktime":
		return "video"
	default:
		return ""
	}
}
