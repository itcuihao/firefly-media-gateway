package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"firefly-media-gateway/internal/provider"

	qtfaststart "github.com/qkzsky/go-qt-faststart"
	_ "golang.org/x/image/webp"
)

const (
	minImageSizeBytes int64 = 100                 // 100 bytes minimum to reject 1x1 transparent dummy PNGs (e.g. 67B) and empty files
	maxImageSizeBytes int64 = 10 * 1024 * 1024   // 10MB for images
	maxVideoSizeBytes int64 = 2000 * 1024 * 1024 // 2GB limit for videos
	maxAudioSizeBytes int64 = 50 * 1024 * 1024   // 50MB for audio
	maxFileSizeBytes  int64 = 500 * 1024 * 1024  // 500MB for generic files
	chunkSize         int64 = 15 * 1024 * 1024   // 15MB per chunk (under Telegram download limit of 20MB)

	accessCacheTTL  = 45 * time.Minute
	metaCacheTTL    = 15 * time.Minute
	sessionExpireTTL = 2 * time.Hour
)

type cachedAccessEntry struct {
	result    provider.AccessResult
	expiresAt time.Time
}

type cachedAssetEntry struct {
	asset     Asset
	expiresAt time.Time
}

type cachedChunksEntry struct {
	chunks    []Chunk
	expiresAt time.Time
}

type uploadSession struct {
	mu           sync.Mutex
	id           string
	project      string
	usage        string
	fileName     string
	declaredMIME string
	sizeBytes    int64
	totalChunks  int
	tmpFile      *os.File
	sniff        []byte
	received     map[int]bool
	createdAt    time.Time
}

type InitUploadInput struct {
	Project          string
	Usage            string
	FileName         string
	DeclaredMIMEType string
	SizeBytes        int64
	SHA256           string
	OverrideProvider provider.StorageProvider
}

type InitUploadOutput struct {
	Duplicated  bool   `json:"duplicated"`
	Asset       *Asset `json:"asset,omitempty"`
	UploadID    string `json:"uploadId,omitempty"`
	ChunkSize   int64  `json:"chunkSize,omitempty"`
	TotalChunks int    `json:"totalChunks,omitempty"`
}

type UploadRequest struct {
	ID                  string // Optional: explicit 32-char hex asset ID
	Project             string
	Usage               string
	FileName            string
	DeclaredContentType string
	Reader              io.Reader
	IsMember            bool // Deprecated: preserved for backward compatibility; all uploads auto-chunk
	OverrideProvider    provider.StorageProvider
}

type Service struct {
	repo              Repository
	providers         map[string]provider.StorageProvider
	defaultProvider   string
	publicBaseURL     string
	uploadConcurrency int

	cacheMu     sync.RWMutex
	accessCache map[string]cachedAccessEntry
	assetCache  map[string]cachedAssetEntry
	chunksCache map[string]cachedChunksEntry

	sessionsMu     sync.Mutex
	uploadSessions map[string]*uploadSession
}

func NewService(repo Repository, providers map[string]provider.StorageProvider, defaultProvider, publicBaseURL string) *Service {
	return &Service{
		repo:              repo,
		providers:         providers,
		defaultProvider:   defaultProvider,
		publicBaseURL:     strings.TrimRight(publicBaseURL, "/"),
		uploadConcurrency: 3,
		accessCache:       make(map[string]cachedAccessEntry),
		assetCache:        make(map[string]cachedAssetEntry),
		chunksCache:       make(map[string]cachedChunksEntry),
		uploadSessions:    make(map[string]*uploadSession),
	}
}

func (s *Service) SetUploadConcurrency(c int) {
	if c > 0 {
		s.uploadConcurrency = c
	}
}

func (s *Service) Upload(ctx context.Context, req UploadRequest) (Asset, error) {
	if strings.TrimSpace(req.Project) == "" {
		return Asset{}, fmt.Errorf("project is required")
	}
	validUsages := map[string]bool{"cover": true, "scene": true, "avatar": true, "audio": true, "file": true}
	if !validUsages[req.Usage] {
		return Asset{}, fmt.Errorf("usage must be one of: cover, scene, avatar, audio, file")
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

	// Validate size limits and image sanity
	if mediaKind == "image" {
		if mimeType != "image/svg+xml" && sizeBytes < minImageSizeBytes {
			return Asset{}, fmt.Errorf("image file too small (%d bytes, min %d bytes required), empty or 1x1 dummy placeholders are rejected: %w", sizeBytes, minImageSizeBytes, ErrInvalidFileType)
		}
		if sizeBytes > maxImageSizeBytes {
			return Asset{}, fmt.Errorf("image exceeds %d bytes: %w", maxImageSizeBytes, ErrFileTooLarge)
		}

		// Decode dimensions to reject 1x1 dummy/placeholder images
		if cfg, _, err := image.DecodeConfig(tmpFile); err == nil {
			if cfg.Width <= 1 && cfg.Height <= 1 {
				return Asset{}, fmt.Errorf("image dimensions too small (%dx%d), 1x1 dummy placeholders are rejected: %w", cfg.Width, cfg.Height, ErrInvalidFileType)
			}
		}
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return Asset{}, fmt.Errorf("seek tmpFile failed: %w", err)
		}
	}
	if mediaKind == "video" && sizeBytes > maxVideoSizeBytes {
		return Asset{}, fmt.Errorf("video exceeds %d bytes: %w", maxVideoSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "audio" && sizeBytes > maxAudioSizeBytes {
		return Asset{}, fmt.Errorf("audio exceeds %d bytes: %w", maxAudioSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "file" && sizeBytes > maxFileSizeBytes {
		return Asset{}, fmt.Errorf("file exceeds %d bytes: %w", maxFileSizeBytes, ErrFileTooLarge)
	}

	// Check if file already exists in active assets (Soft Deduplication - Scheme B)
	if existingAsset, err := s.repo.GetActiveBySHA256(ctx, shaHex); err == nil {
		assetID := req.ID
		if assetID == "" {
			assetID = newUUID()
		}
		ext := extByMIME(mimeType)
		publicURL := fmt.Sprintf("/media/%s%s", assetID, ext)

		// Create metadata entry pointing to the same provider resources
		asset, err := s.repo.Create(ctx, CreateAssetInput{
			ID:                   assetID,
			Provider:             existingAsset.Provider,
			ProviderFileID:       existingAsset.ProviderFileID,
			ProviderBucketOrChat: existingAsset.ProviderBucketOrChat,
			PublicURL:            publicURL,
			MIMEType:             mimeType,
			SizeBytes:            sizeBytes,
			SHA256:               &shaHex,
			Project:              strings.TrimSpace(req.Project),
			Usage:                req.Usage,
			IsChunked:            existingAsset.IsChunked,
		})
		if err != nil {
			return Asset{}, fmt.Errorf("duplicate asset metadata creation failed: %w", err)
		}

		// If chunked, replicate the chunk records
		if existingAsset.IsChunked {
			chunks, err := s.repo.GetChunks(ctx, existingAsset.ID)
			if err != nil {
				return Asset{}, fmt.Errorf("duplicate asset get chunks failed: %w", err)
			}
			newChunks := make([]Chunk, len(chunks))
			for i, c := range chunks {
				newChunks[i] = Chunk{
					AssetID:     assetID,
					ChunkIndex:  c.ChunkIndex,
					ChunkFileID: c.ChunkFileID,
				}
			}
			if err := s.repo.SaveChunks(ctx, assetID, newChunks); err != nil {
				return Asset{}, fmt.Errorf("duplicate asset save chunks failed: %w", err)
			}
		}

		return asset, nil
	}

	// Apply MP4 faststart (move moov atom to file head) for streaming playback
	if mediaKind == "video" && mimeType == "video/mp4" {
		if fsFile, err := fastStartMP4(tmpFile); err != nil {
			log.Printf("mp4 faststart failed, using original file: %v", err)
		} else if fsFile != tmpFile {
			defer os.Remove(fsFile.Name())
			defer fsFile.Close()
			if st, statErr := fsFile.Stat(); statErr == nil && st.Size() > 0 {
				sizeBytes = st.Size()
			}
			tmpFile = fsFile
		}
	}

	// Check if chunking is needed (auto-chunk any file exceeding chunk size to stay under Telegram 20MB getFile limit)
	if sizeBytes <= chunkSize {
		return s.uploadSingle(ctx, p, tmpFile, req, mimeType, sizeBytes, shaHex)
	}

	// Chunked upload for large files/videos
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

	assetID := req.ID
	if assetID == "" {
		assetID = newUUID()
	}
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
	chunks := make([]Chunk, chunkCount)

	concurrency := s.uploadConcurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	if concurrency > chunkCount {
		concurrency = chunkCount
	}

	type chunkTask struct {
		index int
	}
	type chunkResult struct {
		index                int
		chunkFileID          string
		providerBucketOrChat *string
		err                  error
	}

	taskCh := make(chan chunkTask, chunkCount)
	resCh := make(chan chunkResult, chunkCount)

	for i := 0; i < chunkCount; i++ {
		taskCh <- chunkTask{index: i}
	}
	close(taskCh)

	var wg sync.WaitGroup
	ctxWorker, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				select {
				case <-ctxWorker.Done():
					resCh <- chunkResult{index: task.index, err: ctxWorker.Err()}
					continue
				default:
				}

				start := int64(task.index) * chunkSize
				currentChunkSize := chunkSize
				if start+currentChunkSize > sizeBytes {
					currentChunkSize = sizeBytes - start
				}

				var upResult provider.UploadResult
				var uploadErr error

				// Retry up to 3 times per chunk
				for attempt := 0; attempt < 3; attempt++ {
					chunkReader := io.NewSectionReader(tmpFile, start, currentChunkSize)
					chunkName := fmt.Sprintf("%s.chunk%d", req.FileName, task.index)
					upResult, uploadErr = p.Upload(ctxWorker, provider.UploadInput{
						FileName: chunkName,
						MIMEType: mimeType,
						Reader:   chunkReader,
					})
					if uploadErr == nil {
						break
					}
					select {
					case <-ctxWorker.Done():
						uploadErr = ctxWorker.Err()
						break
					case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
					}
				}

				if uploadErr != nil {
					cancelWorker()
					resCh <- chunkResult{index: task.index, err: uploadErr}
					return
				}

				resCh <- chunkResult{
					index:                task.index,
					chunkFileID:          upResult.ProviderFileID,
					providerBucketOrChat: upResult.ProviderBucketOrChat,
				}
			}
		}()
	}

	wg.Wait()
	close(resCh)

	var firstErr error
	var providerBucketOrChat *string

	for res := range resCh {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
		}
		if res.err == nil {
			chunks[res.index] = Chunk{
				ChunkIndex:  res.index,
				ChunkFileID: res.chunkFileID,
			}
			if providerBucketOrChat == nil && res.providerBucketOrChat != nil {
				providerBucketOrChat = res.providerBucketOrChat
			}
		}
	}

	if firstErr != nil {
		return Asset{}, fmt.Errorf("chunked upload failed: %w", firstErr)
	}

	assetID := req.ID
	if assetID == "" {
		assetID = newUUID()
	}
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
	now := time.Now()
	s.cacheMu.RLock()
	if entry, ok := s.assetCache[id]; ok && now.Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.asset, nil
	}
	s.cacheMu.RUnlock()

	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Asset{}, err
	}
	if asset.Status == StatusActive {
		s.cacheMu.Lock()
		if len(s.assetCache) > 4096 {
			s.assetCache = make(map[string]cachedAssetEntry)
		}
		s.assetCache[id] = cachedAssetEntry{
			asset:     asset,
			expiresAt: now.Add(metaCacheTTL),
		}
		s.cacheMu.Unlock()
	}
	return asset, nil
}

func (s *Service) getChunksCached(ctx context.Context, id string) ([]Chunk, error) {
	now := time.Now()
	s.cacheMu.RLock()
	if entry, ok := s.chunksCache[id]; ok && now.Before(entry.expiresAt) {
		s.cacheMu.RUnlock()
		return entry.chunks, nil
	}
	s.cacheMu.RUnlock()

	chunks, err := s.repo.GetChunks(ctx, id)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	if len(s.chunksCache) > 2048 {
		s.chunksCache = make(map[string]cachedChunksEntry)
	}
	s.chunksCache[id] = cachedChunksEntry{
		chunks:    chunks,
		expiresAt: now.Add(metaCacheTTL),
	}
	s.cacheMu.Unlock()
	return chunks, nil
}

func (s *Service) resolveAccessCached(ctx context.Context, p provider.StorageProvider, fileID string, bucketOrChat *string, isOverride bool) (provider.AccessResult, error) {
	if !isOverride {
		cacheKey := p.Name() + ":" + fileID
		now := time.Now()
		s.cacheMu.RLock()
		if entry, ok := s.accessCache[cacheKey]; ok && now.Before(entry.expiresAt) {
			s.cacheMu.RUnlock()
			return entry.result, nil
		}
		s.cacheMu.RUnlock()

		res, err := p.GetAccess(ctx, fileID, bucketOrChat)
		if err != nil {
			return provider.AccessResult{}, err
		}
		s.cacheMu.Lock()
		if len(s.accessCache) > 8192 {
			s.accessCache = make(map[string]cachedAccessEntry)
		}
		s.accessCache[cacheKey] = cachedAccessEntry{
			result:    res,
			expiresAt: time.Now().Add(accessCacheTTL),
		}
		s.cacheMu.Unlock()
		return res, nil
	}
	return p.GetAccess(ctx, fileID, bucketOrChat)
}

func (s *Service) invalidateAssetCache(id string) {
	s.cacheMu.Lock()
	delete(s.assetCache, id)
	delete(s.chunksCache, id)
	s.cacheMu.Unlock()
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Asset, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	assets, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return assets, total, nil
}

func (s *Service) GetProjects(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
	return s.repo.GetDistinctProjects(ctx, onlyPublic, privateRules)
}

func (s *Service) GetUsages(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
	return s.repo.GetDistinctUsages(ctx, onlyPublic, privateRules)
}

func (s *Service) ResolveAccessURL(ctx context.Context, id string) (string, error) {
	asset, err := s.GetMeta(ctx, id)
	if err != nil {
		return "", err
	}
	if asset.Status != StatusActive {
		return "", ErrNotFound
	}
	if asset.IsChunked {
		return fmt.Sprintf("%s/media/%s", s.publicBaseURL, asset.ID), nil
	}
	p, ok := s.providers[asset.Provider]
	if !ok {
		return "", fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
	}
	result, err := s.resolveAccessCached(ctx, p, asset.ProviderFileID, asset.ProviderBucketOrChat, false)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

type StreamInfo struct {
	IsChunked    bool                                                              `json:"isChunked"`
	StreamURL    string                                                            `json:"streamUrl,omitempty"`
	Headers      map[string]string                                                 `json:"headers,omitempty"`
	TotalBytes   int64                                                             `json:"totalBytes"`
	MIMEType     string                                                            `json:"mimeType"`
	ChunkCount   int                                                               `json:"chunkCount,omitempty"`
	ChunkURLs    []string                                                          `json:"chunkUrls,omitempty"`
	ChunkSize    int64                                                             `json:"chunkSize,omitempty"`
	ResolveChunk func(ctx context.Context, chunkIndex int) (string, map[string]string, error) `json:"-"`
}

// StreamAsset returns stream info and an on-demand chunk URL resolver for an asset.
// By resolving chunk URLs lazily per requested byte range and caching them in memory,
// video Range requests only touch the exact chunks needed (1 chunk instead of N).
func (s *Service) StreamAsset(ctx context.Context, id string, overrideProvider provider.StorageProvider) (StreamInfo, error) {
	asset, err := s.GetMeta(ctx, id)
	if err != nil {
		return StreamInfo{}, err
	}
	if asset.Status != StatusActive {
		return StreamInfo{}, ErrNotFound
	}

	isOverride := overrideProvider != nil
	p := overrideProvider
	if p == nil {
		var ok bool
		p, ok = s.providers[asset.Provider]
		if !ok {
			return StreamInfo{}, fmt.Errorf("provider %q: %w", asset.Provider, ErrProviderDisabled)
		}
	}

	if !asset.IsChunked {
		result, err := s.resolveAccessCached(ctx, p, asset.ProviderFileID, asset.ProviderBucketOrChat, isOverride)
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

	// Chunked file - fetch chunk records (cached) and provide lazy per-chunk resolver
	chunks, err := s.getChunksCached(ctx, id)
	if err != nil {
		return StreamInfo{}, fmt.Errorf("get chunks: %w", err)
	}

	bucketOrChat := asset.ProviderBucketOrChat
	resolveChunk := func(reqCtx context.Context, chunkIndex int) (string, map[string]string, error) {
		if chunkIndex < 0 || chunkIndex >= len(chunks) {
			return "", nil, fmt.Errorf("chunk index %d out of bounds [0, %d)", chunkIndex, len(chunks))
		}
		cr, err := s.resolveAccessCached(reqCtx, p, chunks[chunkIndex].ChunkFileID, bucketOrChat, isOverride)
		if err != nil {
			return "", nil, err
		}
		var hdrs map[string]string
		if len(cr.Header) > 0 {
			hdrs = make(map[string]string, len(cr.Header))
			for k, vs := range cr.Header {
				if len(vs) > 0 {
					hdrs[k] = vs[0]
				}
			}
		}
		return cr.URL, hdrs, nil
	}

	return StreamInfo{
		IsChunked:    true,
		TotalBytes:   asset.SizeBytes,
		MIMEType:     asset.MIMEType,
		ChunkCount:   len(chunks),
		ChunkSize:    deduceChunkSize(asset.SizeBytes, len(chunks)),
		ResolveChunk: resolveChunk,
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

	// Check if this physical storage file is shared by other active assets (reference count check)
	skipPhysicalDelete := false
	if asset.SHA256 != nil {
		count, err := s.repo.CountActiveBySHA256(ctx, *asset.SHA256)
		if err == nil && count > 1 {
			skipPhysicalDelete = true
		}
	}

	if asset.IsChunked {
		if !skipPhysicalDelete {
			chunks, err := s.repo.GetChunks(ctx, id)
			if err != nil {
				return Asset{}, fmt.Errorf("get chunks for delete: %w", err)
			}
			for _, c := range chunks {
				if err := p.Delete(ctx, c.ChunkFileID, asset.ProviderBucketOrChat); err != nil {
					return Asset{}, fmt.Errorf("delete chunk from provider %q failed: %w", p.Name(), err)
				}
			}
		}
		if err := s.repo.DeleteChunks(ctx, id); err != nil {
			return Asset{}, fmt.Errorf("delete chunk records: %w", err)
		}
	} else {
		if !skipPhysicalDelete {
			if err := p.Delete(ctx, asset.ProviderFileID, asset.ProviderBucketOrChat); err != nil {
				return Asset{}, fmt.Errorf("delete from provider %q failed: %w", p.Name(), err)
			}
		}
	}

	s.invalidateAssetCache(id)
	return s.repo.MarkDeleted(ctx, id)
}

func (s *Service) cleanupExpiredSessionsLocked() {
	now := time.Now()
	for id, sess := range s.uploadSessions {
		if now.Sub(sess.createdAt) > sessionExpireTTL {
			_ = sess.tmpFile.Close()
			_ = os.Remove(sess.tmpFile.Name())
			delete(s.uploadSessions, id)
		}
	}
}

func (s *Service) InitChunkedUpload(ctx context.Context, in InitUploadInput) (InitUploadOutput, error) {
	project := strings.TrimSpace(in.Project)
	if project == "" {
		return InitUploadOutput{}, fmt.Errorf("project is required")
	}
	validUsages := map[string]bool{"cover": true, "scene": true, "avatar": true, "audio": true, "file": true}
	if !validUsages[in.Usage] {
		return InitUploadOutput{}, fmt.Errorf("usage must be one of: cover, scene, avatar, audio, file")
	}
	fileName := strings.TrimSpace(in.FileName)
	if fileName == "" {
		return InitUploadOutput{}, fmt.Errorf("file name is required")
	}
	if in.SizeBytes <= 0 {
		return InitUploadOutput{}, fmt.Errorf("sizeBytes must be positive")
	}
	if in.SizeBytes > maxVideoSizeBytes {
		return InitUploadOutput{}, fmt.Errorf("file exceeds %d bytes: %w", maxVideoSizeBytes, ErrFileTooLarge)
	}

	// Instant deduplication pre-check if client provided SHA-256
	shaHex := strings.ToLower(strings.TrimSpace(in.SHA256))
	if len(shaHex) == 64 {
		if existingAsset, err := s.repo.GetActiveBySHA256(ctx, shaHex); err == nil {
			assetID := newUUID()
			ext := extByMIME(existingAsset.MIMEType)
			publicURL := fmt.Sprintf("/media/%s%s", assetID, ext)
			asset, err := s.repo.Create(ctx, CreateAssetInput{
				ID:                   assetID,
				Provider:             existingAsset.Provider,
				ProviderFileID:       existingAsset.ProviderFileID,
				ProviderBucketOrChat: existingAsset.ProviderBucketOrChat,
				PublicURL:            publicURL,
				MIMEType:             existingAsset.MIMEType,
				SizeBytes:            existingAsset.SizeBytes,
				SHA256:               &shaHex,
				Project:              project,
				Usage:                in.Usage,
				IsChunked:            existingAsset.IsChunked,
			})
			if err != nil {
				return InitUploadOutput{}, fmt.Errorf("duplicate asset metadata creation failed: %w", err)
			}
			if existingAsset.IsChunked {
				chunks, err := s.repo.GetChunks(ctx, existingAsset.ID)
				if err != nil {
					return InitUploadOutput{}, fmt.Errorf("duplicate asset get chunks failed: %w", err)
				}
				newChunks := make([]Chunk, len(chunks))
				for i, c := range chunks {
					newChunks[i] = Chunk{
						AssetID:     assetID,
						ChunkIndex:  c.ChunkIndex,
						ChunkFileID: c.ChunkFileID,
					}
				}
				if err := s.repo.SaveChunks(ctx, assetID, newChunks); err != nil {
					return InitUploadOutput{}, fmt.Errorf("duplicate asset save chunks failed: %w", err)
				}
			}
			return InitUploadOutput{
				Duplicated: true,
				Asset:      &asset,
			}, nil
		}
	}

	tmpFile, err := os.CreateTemp("", "media-session-*")
	if err != nil {
		return InitUploadOutput{}, fmt.Errorf("create upload session file: %w", err)
	}

	totalChunks := int((in.SizeBytes + chunkSize - 1) / chunkSize)
	if totalChunks <= 0 {
		totalChunks = 1
	}

	uploadID := newUUID()
	sess := &uploadSession{
		id:           uploadID,
		project:      project,
		usage:        in.Usage,
		fileName:     fileName,
		declaredMIME: in.DeclaredMIMEType,
		sizeBytes:    in.SizeBytes,
		totalChunks:  totalChunks,
		tmpFile:      tmpFile,
		received:     make(map[int]bool, totalChunks),
		createdAt:    time.Now(),
	}

	s.sessionsMu.Lock()
	s.cleanupExpiredSessionsLocked()
	s.uploadSessions[uploadID] = sess
	s.sessionsMu.Unlock()

	return InitUploadOutput{
		Duplicated:  false,
		UploadID:    uploadID,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
	}, nil
}

func (s *Service) WriteUploadChunk(_ context.Context, uploadID string, chunkIndex int, src io.Reader) (int, int, error) {
	s.sessionsMu.Lock()
	sess, ok := s.uploadSessions[uploadID]
	s.sessionsMu.Unlock()
	if !ok {
		return 0, 0, ErrNotFound
	}

	if chunkIndex < 0 || chunkIndex >= sess.totalChunks {
		return 0, 0, fmt.Errorf("chunkIndex %d out of range [0, %d)", chunkIndex, sess.totalChunks)
	}

	startOffset := int64(chunkIndex) * chunkSize
	expectedMax := chunkSize
	if startOffset+expectedMax > sess.sizeBytes {
		expectedMax = sess.sizeBytes - startOffset
	}

	buf := make([]byte, 256*1024)
	var written int64
	var sniffHead []byte

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if written+int64(n) > expectedMax {
				return 0, 0, ErrFileTooLarge
			}
			if chunkIndex == 0 && len(sniffHead) < 512 {
				need := 512 - len(sniffHead)
				if need > n {
					need = n
				}
				sniffHead = append(sniffHead, buf[:need]...)
			}
			if _, wErr := sess.tmpFile.WriteAt(buf[:n], startOffset+written); wErr != nil {
				return 0, 0, fmt.Errorf("write chunk at offset: %w", wErr)
			}
			written += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, 0, fmt.Errorf("read chunk stream: %w", readErr)
		}
	}

	sess.mu.Lock()
	if chunkIndex == 0 && len(sniffHead) > 0 {
		sess.sniff = sniffHead
	}
	sess.received[chunkIndex] = true
	receivedCount := len(sess.received)
	totalChunks := sess.totalChunks
	sess.mu.Unlock()

	return receivedCount, totalChunks, nil
}

func (s *Service) CompleteChunkedUpload(ctx context.Context, uploadID string, overrideProvider provider.StorageProvider) (Asset, error) {
	s.sessionsMu.Lock()
	sess, ok := s.uploadSessions[uploadID]
	if ok {
		delete(s.uploadSessions, uploadID)
	}
	s.sessionsMu.Unlock()
	if !ok {
		return Asset{}, ErrNotFound
	}

	tmpFile := sess.tmpFile
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	sess.mu.Lock()
	if len(sess.received) < sess.totalChunks {
		sess.mu.Unlock()
		return Asset{}, fmt.Errorf("upload incomplete: received %d/%d chunks", len(sess.received), sess.totalChunks)
	}
	sniff := sess.sniff
	sess.mu.Unlock()

	p := overrideProvider
	if p == nil {
		var okProv bool
		p, okProv = s.providers[s.defaultProvider]
		if !okProv {
			return Asset{}, fmt.Errorf("provider %q: %w", s.defaultProvider, ErrProviderDisabled)
		}
	}

	mimeType, mediaKind, err := normalizeAndValidateMIME(sess.fileName, sess.declaredMIME, sniff)
	if err != nil {
		return Asset{}, err
	}

	sizeBytes := sess.sizeBytes
	if mediaKind == "image" {
		if mimeType != "image/svg+xml" && sizeBytes < minImageSizeBytes {
			return Asset{}, fmt.Errorf("image file too small (%d bytes, min %d bytes required), empty or 1x1 dummy placeholders are rejected: %w", sizeBytes, minImageSizeBytes, ErrInvalidFileType)
		}
		if sizeBytes > maxImageSizeBytes {
			return Asset{}, fmt.Errorf("image exceeds %d bytes: %w", maxImageSizeBytes, ErrFileTooLarge)
		}
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return Asset{}, fmt.Errorf("seek tmpFile failed: %w", err)
		}
		if cfg, _, err := image.DecodeConfig(tmpFile); err == nil {
			if cfg.Width <= 1 && cfg.Height <= 1 {
				return Asset{}, fmt.Errorf("image dimensions too small (%dx%d), 1x1 dummy placeholders are rejected: %w", cfg.Width, cfg.Height, ErrInvalidFileType)
			}
		}
	}
	if mediaKind == "video" && sizeBytes > maxVideoSizeBytes {
		return Asset{}, fmt.Errorf("video exceeds %d bytes: %w", maxVideoSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "audio" && sizeBytes > maxAudioSizeBytes {
		return Asset{}, fmt.Errorf("audio exceeds %d bytes: %w", maxAudioSizeBytes, ErrFileTooLarge)
	}
	if mediaKind == "file" && sizeBytes > maxFileSizeBytes {
		return Asset{}, fmt.Errorf("file exceeds %d bytes: %w", maxFileSizeBytes, ErrFileTooLarge)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return Asset{}, fmt.Errorf("rewind session file: %w", err)
	}
	h := sha256.New()
	if _, err := io.CopyBuffer(h, io.LimitReader(tmpFile, sizeBytes), make([]byte, 256*1024)); err != nil {
		return Asset{}, fmt.Errorf("hash session file: %w", err)
	}
	shaHex := hex.EncodeToString(h.Sum(nil))

	if existingAsset, err := s.repo.GetActiveBySHA256(ctx, shaHex); err == nil {
		assetID := newUUID()
		ext := extByMIME(mimeType)
		publicURL := fmt.Sprintf("/media/%s%s", assetID, ext)
		asset, err := s.repo.Create(ctx, CreateAssetInput{
			ID:                   assetID,
			Provider:             existingAsset.Provider,
			ProviderFileID:       existingAsset.ProviderFileID,
			ProviderBucketOrChat: existingAsset.ProviderBucketOrChat,
			PublicURL:            publicURL,
			MIMEType:             mimeType,
			SizeBytes:            sizeBytes,
			SHA256:               &shaHex,
			Project:              sess.project,
			Usage:                sess.usage,
			IsChunked:            existingAsset.IsChunked,
		})
		if err != nil {
			return Asset{}, fmt.Errorf("duplicate asset metadata creation failed: %w", err)
		}
		if existingAsset.IsChunked {
			chunks, err := s.repo.GetChunks(ctx, existingAsset.ID)
			if err != nil {
				return Asset{}, fmt.Errorf("duplicate asset get chunks failed: %w", err)
			}
			newChunks := make([]Chunk, len(chunks))
			for i, c := range chunks {
				newChunks[i] = Chunk{
					AssetID:     assetID,
					ChunkIndex:  c.ChunkIndex,
					ChunkFileID: c.ChunkFileID,
				}
			}
			if err := s.repo.SaveChunks(ctx, assetID, newChunks); err != nil {
				return Asset{}, fmt.Errorf("duplicate asset save chunks failed: %w", err)
			}
		}
		return asset, nil
	}

	if mediaKind == "video" && mimeType == "video/mp4" {
		if fsFile, err := fastStartMP4(tmpFile); err != nil {
			log.Printf("mp4 faststart failed, using original file: %v", err)
		} else if fsFile != tmpFile {
			defer os.Remove(fsFile.Name())
			defer fsFile.Close()
			if st, statErr := fsFile.Stat(); statErr == nil && st.Size() > 0 {
				sizeBytes = st.Size()
			}
			tmpFile = fsFile
		}
	}

	upReq := UploadRequest{
		Project:             sess.project,
		Usage:               sess.usage,
		FileName:            sess.fileName,
		DeclaredContentType: sess.declaredMIME,
		OverrideProvider:    overrideProvider,
	}

	if sizeBytes <= chunkSize {
		return s.uploadSingle(ctx, p, tmpFile, upReq, mimeType, sizeBytes, shaHex)
	}
	return s.uploadChunked(ctx, p, tmpFile, upReq, mimeType, sizeBytes, shaHex)
}

func persistUpload(src io.Reader) (*os.File, int64, string, []byte, error) {
	tmpFile, err := os.CreateTemp("", "media-upload-*")
	if err != nil {
		return nil, 0, "", nil, fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()
	buf := make([]byte, 256*1024)
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
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".heic":
		return "image/heic"
	case ".ico":
		return "image/x-icon"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar-compressed"
	case ".7z":
		return "application/x-7z-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	default:
		return ""
	}
}

func mediaKindByMIME(m string) string {
	switch m {
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/svg+xml", "image/avif", "image/heic", "image/x-icon", "image/vnd.microsoft.icon":
		return "image"
	case "video/mp4", "video/webm", "video/quicktime", "video/x-matroska":
		return "video"
	case "audio/mpeg", "audio/ogg", "audio/wav", "audio/aac", "audio/flac", "audio/mp4":
		return "audio"
	case "application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.ms-powerpoint",
		"text/csv",
		"text/plain",
		"application/zip",
		"application/x-rar-compressed",
		"application/x-7z-compressed",
		"application/x-tar",
		"application/gzip",
		"application/json",
		"text/yaml",
		"application/x-yaml":
		return "file"
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
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "image/avif":
		return ".avif"
	case "image/heic":
		return ".heic"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "video/x-matroska":
		return ".mkv"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "audio/aac":
		return ".aac"
	case "audio/flac":
		return ".flac"
	case "audio/mp4":
		return ".m4a"
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "text/csv":
		return ".csv"
	case "text/plain":
		return ".txt"
	case "application/zip":
		return ".zip"
	case "application/x-rar-compressed":
		return ".rar"
	case "application/x-7z-compressed":
		return ".7z"
	case "application/x-tar":
		return ".tar"
	case "application/gzip":
		return ".gz"
	case "application/json":
		return ".json"
	case "text/yaml", "application/x-yaml":
		return ".yaml"
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
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, fmt.Errorf("copy faststarted data: %w", err)
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
