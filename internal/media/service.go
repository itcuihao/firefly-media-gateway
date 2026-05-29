package media

import (
	"bytes"
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
	maxImageSizeBytes      int64 = 10 * 1024 * 1024  // 10MB for images
	maxVideoSizeBytes      int64 = 50 * 1024 * 1024  // 50MB Telegram limit
	maxVideoSizeBytesChunk int64 = 200 * 1024 * 1024 // 200MB for members with chunking
	chunkSize              int64 = 50 * 1024 * 1024  // 50MB per chunk
)

type UploadRequest struct {
	Project             string
	Usage               string
	FileName            string
	DeclaredContentType string
	Reader              io.Reader
	IsMember            bool // Enable chunked upload for large videos
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

	// Read entire file to temp file
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

	// Validate size limits
	if mediaKind == "image" && sizeBytes > maxImageSizeBytes {
		return Asset{}, fmt.Errorf("image exceeds %d bytes: %w", maxImageSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "video" && !req.IsMember && sizeBytes > maxVideoSizeBytes {
		return Asset{}, fmt.Errorf("video exceeds %d bytes (upgrade to member for larger files): %w", maxVideoSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "video" && req.IsMember && sizeBytes > maxVideoSizeBytesChunk {
		return Asset{}, fmt.Errorf("video exceeds %d bytes: %w", maxVideoSizeBytesChunk, ErrFileTooLarge)
	}

	// Check if chunking is needed
	if sizeBytes <= chunkSize || !req.IsMember {
		return s.uploadSingle(ctx, p, tmpFile, req, mimeType, sizeBytes, shaHex)
	}

	// Chunked upload for large videos (member only)
	return s.uploadChunked(ctx, p, tmpFile, req, mimeType, sizeBytes, shaHex)
}

// uploadSingle handles normal single-file upload
func (s *Service) uploadSingle(ctx context.Context, p provider.StorageProvider, tmpFile *os.File, req UploadRequest, mimeType string, sizeBytes int64, shaHex string) (Asset, error) {
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
		IsChunked:            false,
	})
	if err != nil {
		return Asset{}, fmt.Errorf("save media metadata failed: %w", err)
	}

	return asset, nil
}

// uploadChunked handles chunked upload for large videos
func (s *Service) uploadChunked(ctx context.Context, p provider.StorageProvider, tmpFile *os.File, req UploadRequest, mimeType string, sizeBytes int64, shaHex string) (Asset, error) {
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return Asset{}, fmt.Errorf("read temp file: %w", err)
	}

	chunkCount := int((sizeBytes + chunkSize - 1) / chunkSize)
	chunks := make([]Chunk, 0, chunkCount)
	var providerBucketOrChat *string

	for i := 0; i < chunkCount; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize
		if end > sizeBytes {
			end = sizeBytes
		}

		chunkData := data[start:end]
		chunkReader := bytes.NewReader(chunkData)

		chunkName := fmt.Sprintf("%s.chunk%d", req.FileName, i)
		upResult, err := p.Upload(ctx, provider.UploadInput{
			FileName: chunkName,
			MIMEType: mimeType,
			Reader:   chunkReader,
		})
		if err != nil {
			return Asset{}, fmt.Errorf("upload chunk %d failed: %w", i, err)
		}

		chunks = append(chunks, Chunk{
			ChunkIndex:  i,
			ChunkFileID: upResult.ProviderFileID,
		})
		if providerBucketOrChat == nil {
			providerBucketOrChat = upResult.ProviderBucketOrChat
		}
	}

	assetID := newUUID()
	publicURL := fmt.Sprintf("%s/api/v1/media/%s", s.publicBaseURL, assetID)

	sha := shaHex
	asset, err := s.repo.Create(ctx, CreateAssetInput{
		ID:                   assetID,
		Provider:             p.Name(),
		ProviderFileID:       "",
		ProviderBucketOrChat: providerBucketOrChat,
		PublicURL:            publicURL,
		MIMEType:             mimeType,
		SizeBytes:            sizeBytes,
		SHA256:               &sha,
		Project:              strings.TrimSpace(req.Project),
		Usage:                req.Usage,
		IsChunked:            true,
	})
	if err != nil {
		return Asset{}, fmt.Errorf("save media metadata failed: %w", err)
	}

	// Set assetID on chunks before saving
	for i := range chunks {
		chunks[i].AssetID = assetID
	}
	if err := s.repo.SaveChunks(ctx, assetID, chunks); err != nil {
		return Asset{}, fmt.Errorf("save chunks failed: %w", err)
	}

	return asset, nil
}

func (s *Service) GetMeta(ctx context.Context, id string) (Asset, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Asset, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
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
	result, err := p.GetAccess(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

type StreamInfo struct {
	IsChunked  bool     `json:"isChunked"`
	StreamURL  string   `json:"streamUrl,omitempty"`
	TotalBytes int64    `json:"totalBytes"`
	MIMEType   string   `json:"mimeType"`
	ChunkCount int      `json:"chunkCount,omitempty"`
	ChunkURLs  []string `json:"chunkUrls,omitempty"`
}

// StreamAsset returns stream URLs for an asset
func (s *Service) StreamAsset(ctx context.Context, id string) (StreamInfo, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return StreamInfo{}, err
	}
	if asset.Status != StatusActive {
		return StreamInfo{}, ErrNotFound
	}

	p, ok := s.providers[asset.Provider]
	if !ok {
		return StreamInfo{}, fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
	}

	if !asset.IsChunked {
		result, err := p.GetAccess(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat)
		if err != nil {
			return StreamInfo{}, err
		}
		return StreamInfo{
			IsChunked:  false,
			StreamURL:  result.URL,
			TotalBytes: asset.SizeBytes,
			MIMEType:   asset.MIMEType,
		}, nil
	}

	// Chunked file - get chunks then resolve URLs
	chunks, err := s.repo.GetChunks(ctx, id)
	if err != nil {
		return StreamInfo{}, fmt.Errorf("get chunks: %w", err)
	}

	chunkURLs := make([]string, 0, len(chunks))
	for _, c := range chunks {
		cr, err := p.GetAccess(ctx, c.ChunkFileID, asset.ProviderBucketOrChat)
		if err != nil {
			return StreamInfo{}, fmt.Errorf("get chunk URL failed: %w", err)
		}
		chunkURLs = append(chunkURLs, cr.URL)
	}

	return StreamInfo{
		IsChunked:  true,
		TotalBytes: asset.SizeBytes,
		MIMEType:   asset.MIMEType,
		ChunkCount: len(chunkURLs),
		ChunkURLs:  chunkURLs,
	}, nil
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

	if asset.IsChunked {
		chunks, err := s.repo.GetChunks(ctx, id)
		if err != nil {
			return Asset{}, fmt.Errorf("get chunks for delete: %w", err)
		}
		for _, c := range chunks {
			if err := p.Delete(ctx, c.ChunkFileID, asset.ProviderBucketOrChat); err != nil {
				return Asset{}, fmt.Errorf("delete chunk from provider %q failed: %w", p.Name(), err)
			}
		}
		if err := s.repo.DeleteChunks(ctx, id); err != nil {
			return Asset{}, fmt.Errorf("delete chunk records: %w", err)
		}
	} else if err := p.Delete(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat); err != nil {
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

	const absoluteMax = 200 * 1024 * 1024

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			total += int64(n)
			if total > absoluteMax {
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
