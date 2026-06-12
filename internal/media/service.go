package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"firefly-media-gateway/internal/provider"

	qtfaststart "github.com/qkzsky/go-qt-faststart"
)

const (
	maxImageSizeBytes      int64 = 10 * 1024 * 1024   // 10MB for images
	maxVideoSizeBytes      int64 = 50 * 1024 * 1024   // 50MB Telegram limit
	maxVideoSizeBytesChunk int64 = 2000 * 1024 * 1024 // 2GB limit for chunking
	chunkSize              int64 = 15 * 1024 * 1024   // 15MB per chunk (under Telegram download limit of 20MB)
)

type UploadRequest struct {
	Project             string
	Usage               string
	FileName            string
	DeclaredContentType string
	Reader              io.Reader
	IsMember            bool // Enable chunked upload for large videos
	OverrideProvider    provider.StorageProvider
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

	p := req.OverrideProvider
	if p == nil {
		var ok bool
		p, ok = s.providers[s.defaultProvider]
		if !ok {
			return Asset{}, fmt.Errorf("provider %q: %w", s.defaultProvider, ErrProviderDisabled)
		}
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

	// Apply MP4 faststart (move moov atom to file head) for streaming playback
	if mediaKind == "video" && mimeType == "video/mp4" {
		if fsFile, err := fastStartMP4(tmpFile); err != nil {
			log.Printf("mp4 faststart failed, using original file: %v", err)
		} else if fsFile != tmpFile {
			defer os.Remove(fsFile.Name())
			defer fsFile.Close()
			tmpFile = fsFile
		}
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
	ext := extByMIME(mimeType)
	publicURL := fmt.Sprintf("/media/%s%s", assetID, ext)

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
	chunkCount := int((sizeBytes + chunkSize - 1) / chunkSize)
	chunks := make([]Chunk, 0, chunkCount)
	var providerBucketOrChat *string

	for i := 0; i < chunkCount; i++ {
		start := int64(i) * chunkSize
		if _, err := tmpFile.Seek(start, io.SeekStart); err != nil {
			return Asset{}, fmt.Errorf("seek temp file to chunk %d failed: %w", i, err)
		}

		chunkReader := io.LimitReader(tmpFile, chunkSize)
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
	ext := extByMIME(mimeType)
	publicURL := fmt.Sprintf("/media/%s%s", assetID, ext)

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
	IsChunked  bool              `json:"isChunked"`
	StreamURL  string            `json:"streamUrl,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	TotalBytes int64             `json:"totalBytes"`
	MIMEType   string            `json:"mimeType"`
	ChunkCount int               `json:"chunkCount,omitempty"`
	ChunkURLs  []string          `json:"chunkUrls,omitempty"`
	ChunkSize  int64             `json:"chunkSize,omitempty"`
}

// StreamAsset returns stream URLs for an asset
func (s *Service) StreamAsset(ctx context.Context, id string, overrideProvider provider.StorageProvider) (StreamInfo, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return StreamInfo{}, err
	}
	if asset.Status != StatusActive {
		return StreamInfo{}, ErrNotFound
	}

	p := overrideProvider
	if p == nil {
		var ok bool
		p, ok = s.providers[asset.Provider]
		if !ok {
			return StreamInfo{}, fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
		}
	}

	if !asset.IsChunked {
		result, err := p.GetAccess(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat)
		if err != nil {
			return StreamInfo{}, err
		}
		var headers map[string]string
		if len(result.Header) > 0 {
			headers = make(map[string]string)
			for k, vs := range result.Header {
				if len(vs) > 0 {
					headers[k] = vs[0]
				}
			}
		}
		return StreamInfo{
			IsChunked:  false,
			StreamURL:  result.URL,
			Headers:    headers,
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
	var headers map[string]string
	for _, c := range chunks {
		cr, err := p.GetAccess(ctx, c.ChunkFileID, asset.ProviderBucketOrChat)
		if err != nil {
			return StreamInfo{}, fmt.Errorf("get chunk URL failed: %w", err)
		}
		chunkURLs = append(chunkURLs, cr.URL)
		if headers == nil && len(cr.Header) > 0 {
			headers = make(map[string]string)
			for k, vs := range cr.Header {
				if len(vs) > 0 {
					headers[k] = vs[0]
				}
			}
		}
	}

	return StreamInfo{
		IsChunked:  true,
		TotalBytes: asset.SizeBytes,
		MIMEType:   asset.MIMEType,
		ChunkCount: len(chunkURLs),
		ChunkURLs:  chunkURLs,
		Headers:    headers,
		ChunkSize:  deduceChunkSize(asset.SizeBytes, len(chunkURLs)),
	}, nil
}

func (s *Service) Delete(ctx context.Context, id string, overrideProvider provider.StorageProvider) (Asset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status == StatusDeleted {
		return asset, nil
	}

	p := overrideProvider
	if p == nil {
		var ok bool
		p, ok = s.providers[asset.Provider]
		if !ok {
			return Asset{}, fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
		}
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

	const absoluteMax = 2000 * 1024 * 1024

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

func extByMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	default:
		return ""
	}
}

// fastStartMP4 moves the moov atom to the beginning of an MP4 file for streaming.
// Returns a new temp file with the faststarted data, or the original file if already faststart.
// Caller is responsible for cleaning up both the original and returned temp files.
func fastStartMP4(tmpFile *os.File) (*os.File, error) {
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	f, err := qtfaststart.Read(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("parse mp4: %w", err)
	}

	if f.FastStartEnabled() {
		return tmpFile, nil
	}

	if err := f.Convert(false); err != nil {
		return nil, fmt.Errorf("convert: %w", err)
	}

	out, err := os.CreateTemp("", "media-faststart-*")
	if err != nil {
		return nil, fmt.Errorf("create output temp: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, fmt.Errorf("read converted data: %w", err)
	}
	if _, err := out.Write(data); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, fmt.Errorf("write faststarted data: %w", err)
	}
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, fmt.Errorf("seek output: %w", err)
	}
	return out, nil
}

func deduceChunkSize(totalSize int64, chunkCount int) int64 {
	if chunkCount <= 1 {
		return totalSize
	}

	standards := []int64{
		15 * 1024 * 1024, // 15MB
		50 * 1024 * 1024, // 50MB
		10 * 1024 * 1024, // 10MB
	}
	for _, s := range standards {
		calculatedChunks := int((totalSize + s - 1) / s)
		if calculatedChunks == chunkCount {
			return s
		}
	}

	return (totalSize + int64(chunkCount) - 1) / int64(chunkCount)
}
