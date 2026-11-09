package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"

	"github.com/KarpelesLab/gowebp"
	_ "golang.org/x/image/bmp"
	"golang.org/x/sync/singleflight"
)

const maxRequestBodyBytes int64 = 2005 * 1024 * 1024 // 2GB + 5MB buffer

var streamCopyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 256*1024)
		return &buf
	},
}

type Server struct {
	svc              *media.Service
	authToken        string
	telegramBotToken string
	workerBaseURL    string
	workerAuthToken  string
	publicBaseURL    string
	privateRules     []string
	databaseDriver   string
	storageMode      string
	commit           string
	logger           *log.Logger
	sf               singleflight.Group
	diskCache        *DiskCache
	workerProviders  sync.Map
}

func NewServer(svc *media.Service, authToken, telegramBotToken, workerBaseURL, workerAuthToken, publicBaseURL string, privateRules []string, databaseDriver, storageMode, commit string, logger *log.Logger) *Server {
	cacheDir := os.Getenv("CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "data/cache"
	}
	cacheMaxGB := 20
	if v := os.Getenv("CACHE_MAX_GB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cacheMaxGB = n
		}
	}
	cacheMaxBytes := int64(cacheMaxGB) * 1024 * 1024 * 1024
	if v := os.Getenv("CACHE_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cacheMaxBytes = n
		}
	}
	dc, _ := NewDiskCache(cacheDir, cacheMaxBytes, logger)

	return &Server{
		svc:              svc,
		authToken:        authToken,
		telegramBotToken: telegramBotToken,
		workerBaseURL:    workerBaseURL,
		workerAuthToken:  workerAuthToken,
		publicBaseURL:    strings.TrimRight(publicBaseURL, "/"),
		privateRules:     privateRules,
		databaseDriver:   databaseDriver,
		storageMode:      storageMode,
		commit:           commit,
		logger:           logger,
		diskCache:        dc,
	}
}

// SetDiskCache overrides the server's disk cache (useful for testing or custom configuration).
func (s *Server) SetDiskCache(dc *DiskCache) {
	s.diskCache = dc
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)

	// Vue 3 + Naive UI 前端 SPA（嵌入到二进制，由 embed.go 提供）
	// /admin 无斜杠时重定向，/admin/ 开头统一走 SPA handler
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	mux.Handle("/admin/", http.StripPrefix("/admin", s.frontendHandler()))

	mux.HandleFunc("POST /api/v1/media/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("POST /api/v1/media/upload/init", s.withAuth(s.handleUploadInit))
	mux.HandleFunc("POST /api/v1/media/upload/{uploadId}/chunk", s.withAuth(s.handleUploadChunk))
	mux.HandleFunc("POST /api/v1/media/upload/{uploadId}/complete", s.withAuth(s.handleUploadComplete))
	mux.HandleFunc("GET /api/v1/media", s.handleListMedia)
	mux.HandleFunc("GET /api/v1/media/projects", s.handleGetProjects)
	mux.HandleFunc("GET /api/v1/media/usages", s.handleGetUsages)
	mux.HandleFunc("GET /api/v1/media/{mediaId}/meta", s.withAuth(s.handleGetMeta))
	mux.HandleFunc("DELETE /api/v1/media/{mediaId}", s.withAuth(s.handleDelete))
	mux.HandleFunc("GET /api/v1/telegram/chat-ids", s.withAuth(s.handleGetTelegramChatIDs))
	// Media retrieval endpoints handle access control dynamically in serveMediaBinary
	mux.HandleFunc("GET /api/v1/media/{mediaId}", s.handleGetMedia)
	mux.HandleFunc("GET /api/v1/media/{mediaId}/stream", s.handleStream)

	// Short public URL for media access (same handlers)
	mux.HandleFunc("GET /media/{mediaId}", s.handleGetMedia)
	mux.HandleFunc("GET /media/{mediaId}/stream", s.handleStream)

	// Provider verification endpoints
	mux.HandleFunc("POST /api/v1/provider/telegram/verify", s.withAuth(s.handleTelegramVerify))
	mux.HandleFunc("POST /api/v1/provider/telegram/chat-ids", s.withAuth(s.handleTelegramChatIDsPost))
	mux.HandleFunc("POST /api/v1/provider/discord/verify", s.withAuth(s.handleDiscordVerify))
	mux.HandleFunc("POST /api/v1/provider/discord/guilds", s.withAuth(s.handleDiscordGuilds))
	mux.HandleFunc("POST /api/v1/provider/worker/verify", s.withAuth(s.handleWorkerVerify))

	return s.withCORS(s.withLogging(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	storageMode := s.storageMode
	workerURL := s.workerBaseURL

	if strings.ToLower(r.Header.Get("X-Storage-Mode")) == "proxy" || r.Header.Get("X-Worker-Base-URL") != "" {
		storageMode = "proxy"
		workerURL = strings.TrimSpace(r.Header.Get("X-Worker-Base-URL"))
		if workerURL == "" {
			workerURL = s.workerBaseURL
		}
	}

	if workerURL != "" {
		if !strings.HasPrefix(workerURL, "http://") && !strings.HasPrefix(workerURL, "https://") {
			workerURL = "https://" + workerURL
		}
		workerURL = strings.TrimRight(workerURL, "/")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"time":            time.Now().UTC(),
		"commit":          s.commit,
		"database_driver": s.databaseDriver,
		"storage_driver":  storageMode,
		"worker_url":      workerURL,
	})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid multipart form", err)
		return
	}

	var (
		project             string
		usage               string
		isMember            bool
		fileName            string
		declaredContentType string
		bufferedTmpFile     *os.File
	)
	defer func() {
		if bufferedTmpFile != nil {
			_ = bufferedTmpFile.Close()
			_ = os.Remove(bufferedTmpFile.Name())
		}
	}()

	overrideProvider := s.resolveWorkerProvider(r)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
				s.writeError(w, r, http.StatusRequestEntityTooLarge, "file too large", err)
				return
			}
			s.writeError(w, r, http.StatusBadRequest, "invalid multipart form", err)
			return
		}

		formName := part.FormName()
		switch formName {
		case "project":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			project = strings.TrimSpace(string(b))
			_ = part.Close()
		case "usage":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			usage = strings.TrimSpace(string(b))
			_ = part.Close()
		case "member", "is_member":
			b, _ := io.ReadAll(io.LimitReader(part, 256))
			if strings.TrimSpace(string(b)) == "true" {
				isMember = true
			}
			_ = part.Close()
		case "file":
			fileName = part.FileName()
			declaredContentType = part.Header.Get("Content-Type")

			// Fast path: if project & usage were already parsed before the file part,
			// stream `part` directly into Service.Upload without an extra temporary file!
			if project != "" && usage != "" {
				asset, upErr := s.svc.Upload(r.Context(), media.UploadRequest{
					Project:             project,
					Usage:               usage,
					FileName:            fileName,
					DeclaredContentType: declaredContentType,
					Reader:              part,
					IsMember:            isMember,
					OverrideProvider:    overrideProvider,
				})
				_ = part.Close()
				if upErr != nil {
					if strings.Contains(strings.ToLower(upErr.Error()), "request body too large") {
						s.writeError(w, r, http.StatusRequestEntityTooLarge, "file too large", upErr)
						return
					}
					s.writeDomainError(w, r, upErr)
					return
				}
				writeJSON(w, http.StatusCreated, s.signAssetURL(asset))
				return
			}

			// Fallback when `file` field appears before `project`/`usage` in multipart form
			tmp, tmpErr := os.CreateTemp("", "http-multipart-*")
			if tmpErr != nil {
				_ = part.Close()
				s.writeError(w, r, http.StatusInternalServerError, "failed to create temp file", tmpErr)
				return
			}
			bufferedTmpFile = tmp
			bufPtr := streamCopyBufPool.Get().(*[]byte)
			_, copyErr := io.CopyBuffer(tmp, part, *bufPtr)
			streamCopyBufPool.Put(bufPtr)
			_ = part.Close()
			if copyErr != nil {
				if strings.Contains(strings.ToLower(copyErr.Error()), "request body too large") {
					s.writeError(w, r, http.StatusRequestEntityTooLarge, "file too large", copyErr)
					return
				}
				s.writeError(w, r, http.StatusBadRequest, "invalid multipart stream", copyErr)
				return
			}
		default:
			_ = part.Close()
		}
	}

	if bufferedTmpFile == nil || fileName == "" {
		s.writeError(w, r, http.StatusBadRequest, "file is required", nil)
		return
	}
	if _, err := bufferedTmpFile.Seek(0, io.SeekStart); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "rewind temp file failed", err)
		return
	}

	asset, err := s.svc.Upload(r.Context(), media.UploadRequest{
		Project:             project,
		Usage:               usage,
		FileName:            fileName,
		DeclaredContentType: declaredContentType,
		Reader:              bufferedTmpFile,
		IsMember:            isMember,
		OverrideProvider:    overrideProvider,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, s.signAssetURL(asset))
}

func (s *Server) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Project   string `json:"project"`
		Usage     string `json:"usage"`
		FileName  string `json:"fileName"`
		MIMEType  string `json:"mimeType"`
		SizeBytes int64  `json:"sizeBytes"`
		SHA256    string `json:"sha256"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&body); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	overrideProvider := s.resolveWorkerProvider(r)
	out, err := s.svc.InitChunkedUpload(r.Context(), media.InitUploadInput{
		Project:          body.Project,
		Usage:            body.Usage,
		FileName:         body.FileName,
		DeclaredMIMEType: body.MIMEType,
		SizeBytes:        body.SizeBytes,
		SHA256:           body.SHA256,
		OverrideProvider: overrideProvider,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	if out.Duplicated && out.Asset != nil {
		signed := s.signAssetURL(*out.Asset)
		out.Asset = &signed
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimSpace(r.PathValue("uploadId"))
	if uploadID == "" {
		s.writeError(w, r, http.StatusBadRequest, "uploadId is required", nil)
		return
	}
	idxStr := strings.TrimSpace(r.URL.Query().Get("index"))
	chunkIndex, err := strconv.Atoi(idxStr)
	if err != nil || chunkIndex < 0 {
		s.writeError(w, r, http.StatusBadRequest, "valid chunk index query parameter is required", err)
		return
	}

	// Limit single chunk request to 20MB
	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
	defer r.Body.Close()

	received, total, err := s.svc.WriteUploadChunk(r.Context(), uploadID, chunkIndex, r.Body)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId":    uploadID,
		"chunkIndex":  chunkIndex,
		"received":    received,
		"totalChunks": total,
	})
}

func (s *Server) handleUploadComplete(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimSpace(r.PathValue("uploadId"))
	if uploadID == "" {
		s.writeError(w, r, http.StatusBadRequest, "uploadId is required", nil)
		return
	}

	overrideProvider := s.resolveWorkerProvider(r)
	asset, err := s.svc.CompleteChunkedUpload(r.Context(), uploadID, overrideProvider)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, s.signAssetURL(asset))
}

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	s.serveMediaBinary(w, r)
}

func (s *Server) handleGetMeta(w http.ResponseWriter, r *http.Request) {
	mediaID := cleanMediaID(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.GetMeta(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, s.signAssetURL(asset))
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	mediaID := cleanMediaID(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	s.serveMediaBinary(w, r)
}

func (s *Server) serveMediaBinary(w http.ResponseWriter, r *http.Request) {
	mediaID := cleanMediaID(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.GetMeta(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	if !s.checkAccess(r, asset) {
		s.logger.Printf("[AUTH]  %s %s => 401 unauthorized for private asset %q", r.Method, r.URL.RequestURI(), mediaID)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}

	overrideProvider := s.resolveWorkerProvider(r)

	streamInfo, err := s.svc.StreamAsset(r.Context(), mediaID, overrideProvider)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	ext := extByMIME(asset.MIMEType)
	filename := asset.ID
	if ext != "" {
		filename += ext
	}
	disposition := "inline"
	if dl := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("download"))); dl == "1" || dl == "true" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, filename))
	w.Header().Set("Accept-Ranges", "bytes")

	isPublic := s.isAssetPublic(asset)
	cacheControl := "public, max-age=31536000, immutable"
	if !isPublic {
		cacheControl = "private, no-cache"
	}
	w.Header().Set("Cache-Control", cacheControl)

	// Calculate immutable ETag for caching
	etag := fmt.Sprintf("%q", asset.ID)
	if asset.SHA256 != nil && *asset.SHA256 != "" {
		etag = fmt.Sprintf("%q", *asset.SHA256)
	}

	willServeWebP := disposition == "inline" && !streamInfo.IsChunked && isCompressibleImage(asset.MIMEType) && r.URL.Query().Get("raw") != "true" && clientAcceptsWebP(r)
	if willServeWebP {
		etag = fmt.Sprintf("%q", "webp-"+asset.ID)
	}
	w.Header().Set("ETag", etag)

	// HEAD fast-path: return metadata headers without downloading payload
	if r.Method == http.MethodHead {
		if streamInfo.TotalBytes > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(streamInfo.TotalBytes, 10))
		}
		if streamInfo.MIMEType != "" {
			w.Header().Set("Content-Type", streamInfo.MIMEType)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// If-None-Match 304 handling (only for non-Range requests)
	if r.Header.Get("Range") == "" {
		if match := r.Header.Get("If-None-Match"); match != "" {
			if strings.Contains(match, etag) || match == "*" {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	if streamInfo.IsChunked {
		s.proxyChunkedMedia(w, r, streamInfo, mediaID, cacheControl)
		return
	}

	if disposition == "inline" && isCompressibleImage(asset.MIMEType) && r.URL.Query().Get("raw") != "true" && clientAcceptsWebP(r) {
		if s.tryServeWebPCache(w, r, mediaID, streamInfo, cacheControl) {
			return
		}
	}

	s.proxySingleMedia(w, r, streamInfo, mediaID, cacheControl)
}

func clientAcceptsWebP(r *http.Request) bool {
	if r.URL.Query().Get("format") == "webp" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(strings.ToLower(accept), "image/webp")
}

func isCompressibleImage(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	return m == "image/jpeg" || m == "image/png" || m == "image/jpg"
}

func (s *Server) tryServeWebPCache(w http.ResponseWriter, r *http.Request, mediaID string, streamInfo media.StreamInfo, cacheControl string) bool {
	cacheDir := "data/cache"
	if s.diskCache != nil {
		cacheDir = s.diskCache.Dir()
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		s.logger.Printf("[WEBP] failed to create cache directory: %v", err)
		return false
	}

	cachePath := filepath.Join(cacheDir, mediaID+".webp")

	// If cache already exists, serve immediately
	if _, err := os.Stat(cachePath); err == nil {
		s.logger.Printf("[WEBP] serving cached WebP for %q", mediaID)
		_ = os.Chtimes(cachePath, time.Now(), time.Now())
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", mediaID+".webp"))
		w.Header().Set("Cache-Control", cacheControl)
		http.ServeFile(w, r, cachePath)
		return true
	}

	// Use singleflight to coalesce concurrent cache misses for the same mediaID
	_, sfErr, _ := s.sf.Do("webp:"+mediaID, func() (any, error) {
		// Double-check inside singleflight lock
		if _, err := os.Stat(cachePath); err == nil {
			_ = os.Chtimes(cachePath, time.Now(), time.Now())
			return cachePath, nil
		}

		s.logger.Printf("[WEBP] cache miss for %q, downloading and converting...", mediaID)

		// Download original image using detached context to avoid cancellation if one caller drops
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, streamInfo.StreamURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build download request: %w", err)
		}
		for k, v := range streamInfo.Headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("download original image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("download status: %d", resp.StatusCode)
		}

		img, _, err := image.Decode(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}

		tmpFile, err := os.CreateTemp(cacheDir, "webp-tmp-*")
		if err != nil {
			return nil, fmt.Errorf("create temp file: %w", err)
		}
		tmpName := tmpFile.Name()
		defer func() {
			tmpFile.Close()
			os.Remove(tmpName)
		}()

		err = gowebp.Encode(tmpFile, img, &gowebp.Options{Lossy: true, Quality: 80})
		if err != nil {
			return nil, fmt.Errorf("encode webp: %w", err)
		}
		tmpFile.Close()

		if err := os.Rename(tmpName, cachePath); err != nil {
			return nil, fmt.Errorf("save cache file: %w", err)
		}

		_ = os.Chtimes(cachePath, time.Now(), time.Now())
		if s.diskCache != nil {
			s.diskCache.TriggerEviction()
		}

		s.logger.Printf("[WEBP] successfully created cache for %q", mediaID)
		return cachePath, nil
	})

	if sfErr != nil {
		s.logger.Printf("[WEBP] conversion failed for %q: %v", mediaID, sfErr)
		return false
	}

	if _, err := os.Stat(cachePath); err == nil {
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", mediaID+".webp"))
		w.Header().Set("Cache-Control", cacheControl)
		http.ServeFile(w, r, cachePath)
		return true
	}

	return false
}

func (s *Server) proxySingleMedia(w http.ResponseWriter, r *http.Request, streamInfo media.StreamInfo, mediaID string, cacheControl string) {
	// Attempt to serve from disk cache if file size is known and within reasonable bounds (<= 100MB)
	if s.diskCache != nil && streamInfo.TotalBytes > 0 && streamInfo.TotalBytes <= 100*1024*1024 {
		fileKey := fmt.Sprintf("%s.bin", mediaID)
		filePath, err := s.diskCache.GetOrFetchFile(r.Context(), fileKey, streamInfo.TotalBytes, streamInfo.StreamURL, streamInfo.Headers)
		if err == nil {
			f, err := os.Open(filePath)
			if err == nil {
				defer f.Close()
				if streamInfo.MIMEType != "" {
					w.Header().Set("Content-Type", streamInfo.MIMEType)
				}
				if w.Header().Get("Accept-Ranges") == "" {
					w.Header().Set("Accept-Ranges", "bytes")
				}
				w.Header().Set("Cache-Control", cacheControl)
				fi, _ := f.Stat()
				var modTime time.Time
				if fi != nil {
					modTime = fi.ModTime()
				}
				http.ServeContent(w, r, mediaID, modTime, f)
				return
			}
		}
	}

	// Live streaming proxy fallback
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, streamInfo.StreamURL, nil)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to build media request", err)
		return
	}
	for k, v := range streamInfo.Headers {
		req.Header.Set(k, v)
	}
	if rangeHeader := strings.TrimSpace(r.Header.Get("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "failed to fetch media", err)
		return
	}
	defer resp.Body.Close()

	copyMediaHeaders(w.Header(), resp.Header)
	// Force the verified MIMEType from database to avoid upstream generic octet-stream overrides
	if streamInfo.MIMEType != "" {
		w.Header().Set("Content-Type", streamInfo.MIMEType)
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	w.Header().Set("Cache-Control", cacheControl)

	w.WriteHeader(resp.StatusCode)
	bufPtr := streamCopyBufPool.Get().(*[]byte)
	defer streamCopyBufPool.Put(bufPtr)
	if _, err := io.CopyBuffer(w, resp.Body, *bufPtr); err != nil {
		s.logger.Printf("[WARN]  stream media failed: %v", err)
	}
}

func (s *Server) proxyChunkedMedia(w http.ResponseWriter, r *http.Request, streamInfo media.StreamInfo, mediaID string, cacheControl string) {
	chunkSize := streamInfo.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 15 * 1024 * 1024 // Fallback default chunk size
	}

	var start int64 = 0
	var end int64 = streamInfo.TotalBytes - 1
	isRange := false

	if rangeHeader := strings.TrimSpace(r.Header.Get("Range")); rangeHeader != "" {
		ranges, err := parseRangeHeader(rangeHeader, streamInfo.TotalBytes)
		if err != nil {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", streamInfo.TotalBytes))
			s.writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "invalid range header", err)
			return
		}
		if len(ranges) > 0 {
			start = ranges[0].start
			end = ranges[0].end
			isRange = true
		}
	}

	if streamInfo.MIMEType != "" {
		w.Header().Set("Content-Type", streamInfo.MIMEType)
	}

	if isRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, streamInfo.TotalBytes))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusPartialContent)
	} else {
		if streamInfo.TotalBytes > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(streamInfo.TotalBytes, 10))
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusOK)
	}

	chunkCount := streamInfo.ChunkCount
	if chunkCount == 0 {
		chunkCount = len(streamInfo.ChunkURLs)
	}
	if chunkCount <= 0 {
		return
	}

	firstChunk := int(start / chunkSize)
	if firstChunk < 0 {
		firstChunk = 0
	}
	lastChunk := int(end / chunkSize)
	if lastChunk >= chunkCount {
		lastChunk = chunkCount - 1
	}

	bufPtr := streamCopyBufPool.Get().(*[]byte)
	defer streamCopyBufPool.Put(bufPtr)

	for i := firstChunk; i <= lastChunk; i++ {
		chunkStart := int64(i) * chunkSize
		chunkEnd := chunkStart + chunkSize - 1
		if chunkEnd >= streamInfo.TotalBytes {
			chunkEnd = streamInfo.TotalBytes - 1
		}
		chunkSizeBytes := chunkEnd - chunkStart + 1

		// Check if this chunk overlaps with the requested range
		if start > chunkEnd || end < chunkStart {
			continue
		}

		// Lookahead prefetch: resolve next chunk's access URL in background while streaming current chunk
		if i+1 <= lastChunk && streamInfo.ResolveChunk != nil {
			go func(nextIdx int) {
				_, _, _ = streamInfo.ResolveChunk(r.Context(), nextIdx)
			}(i + 1)
		}

		var chunkURL string
		var chunkHeaders map[string]string
		if streamInfo.ResolveChunk != nil {
			var resolveErr error
			chunkURL, chunkHeaders, resolveErr = streamInfo.ResolveChunk(r.Context(), i)
			if resolveErr != nil {
				s.logger.Printf("[WARN]  resolve chunk %d URL failed: %v", i, resolveErr)
				return
			}
		} else if i < len(streamInfo.ChunkURLs) {
			chunkURL = streamInfo.ChunkURLs[i]
			chunkHeaders = streamInfo.Headers
		}

		if len(chunkHeaders) == 0 {
			chunkHeaders = streamInfo.Headers
		}

		// Calculate relative start/end within this chunk
		relativeStart := start - chunkStart
		if relativeStart < 0 {
			relativeStart = 0
		}
		relativeEnd := end - chunkStart
		if relativeEnd >= chunkSizeBytes {
			relativeEnd = chunkSizeBytes - 1
		}

		chunkKey := fmt.Sprintf("%s_%d.bin", mediaID, i)
		var chunkPath string
		var err error
		if s.diskCache != nil {
			chunkPath, err = s.diskCache.GetOrFetchChunk(r.Context(), chunkKey, chunkSizeBytes, chunkURL, chunkHeaders)
		} else {
			err = errors.New("disk cache disabled")
		}

		if err != nil {
			s.logger.Printf("[WARN] disk cache miss/fetch failed for chunk %d: %v, falling back to direct stream", i, err)
			if !s.streamChunkDirect(w, r, chunkURL, chunkHeaders, relativeStart, relativeEnd, isRange) {
				return
			}
			continue
		}

		f, err := os.Open(chunkPath)
		if err != nil {
			s.logger.Printf("[WARN] open cached chunk %d failed: %v, falling back to direct stream", i, err)
			if !s.streamChunkDirect(w, r, chunkURL, chunkHeaders, relativeStart, relativeEnd, isRange) {
				return
			}
			continue
		}

		if _, err := f.Seek(relativeStart, io.SeekStart); err != nil {
			f.Close()
			s.logger.Printf("[WARN] seek cached chunk %d failed: %v", i, err)
			return
		}

		neededBytes := relativeEnd - relativeStart + 1
		_, copyErr := io.CopyBuffer(w, io.LimitReader(f, neededBytes), *bufPtr)
		f.Close()
		if copyErr != nil {
			s.logger.Printf("[WARN] write chunk %d failed: %v", i, copyErr)
			return
		}
	}
}

func (s *Server) streamChunkDirect(w http.ResponseWriter, r *http.Request, chunkURL string, headers map[string]string, relativeStart, relativeEnd int64, isRange bool) bool {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, chunkURL, nil)
	if err != nil {
		s.logger.Printf("[WARN] build chunk request failed: %v", err)
		return false
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", relativeStart, relativeEnd))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Printf("[WARN] fetch chunk range failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		s.logger.Printf("[WARN] fetch chunk returned status=%d", resp.StatusCode)
		return false
	}

	var bodyReader io.Reader = resp.Body
	if resp.StatusCode == http.StatusOK && isRange {
		if relativeStart > 0 {
			if _, err := io.CopyN(io.Discard, resp.Body, relativeStart); err != nil {
				s.logger.Printf("[WARN] skip to chunk start failed: %v", err)
				return false
			}
		}
		neededBytes := relativeEnd - relativeStart + 1
		bodyReader = io.LimitReader(resp.Body, neededBytes)
	}

	bufPtr := streamCopyBufPool.Get().(*[]byte)
	defer streamCopyBufPool.Put(bufPtr)
	if _, copyErr := io.CopyBuffer(w, bodyReader, *bufPtr); copyErr != nil {
		s.logger.Printf("[WARN] write chunk failed: %v", copyErr)
		return false
	}
	return true
}

type byteRange struct {
	start int64
	end   int64
}

func parseRangeHeader(header string, size int64) ([]byteRange, error) {
	if header == "" {
		return nil, nil
	}
	if !strings.HasPrefix(header, "bytes=") {
		return nil, fmt.Errorf("invalid range header prefix")
	}
	rangesStr := strings.TrimPrefix(header, "bytes=")
	var ranges []byteRange
	for _, rStr := range strings.Split(rangesStr, ",") {
		rStr = strings.TrimSpace(rStr)
		if rStr == "" {
			continue
		}
		parts := strings.Split(rStr, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format")
		}
		startStr := strings.TrimSpace(parts[0])
		endStr := strings.TrimSpace(parts[1])
		var r byteRange
		if startStr == "" {
			if endStr == "" {
				return nil, fmt.Errorf("invalid range values")
			}
			suffixLen, err := strconv.ParseInt(endStr, 10, 64)
			if err != nil || suffixLen <= 0 {
				return nil, fmt.Errorf("invalid suffix length")
			}
			if suffixLen > size {
				suffixLen = size
			}
			r.start = size - suffixLen
			r.end = size - 1
		} else {
			start, err := strconv.ParseInt(startStr, 10, 64)
			if err != nil || start < 0 {
				return nil, fmt.Errorf("invalid start byte")
			}
			r.start = start
			if endStr == "" {
				r.end = size - 1
			} else {
				end, err := strconv.ParseInt(endStr, 10, 64)
				if err != nil || end < start {
					return nil, fmt.Errorf("invalid end byte")
				}
				r.end = end
				if r.end >= size {
					r.end = size - 1
				}
			}
		}
		if r.start >= size || r.start > r.end {
			continue
		}
		ranges = append(ranges, r)
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("no valid ranges")
	}
	return ranges, nil
}

func copyMediaHeaders(dst, src http.Header) {
	for _, key := range []string{
		"Content-Type",
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"ETag",
		"Last-Modified",
		"Cache-Control",
	} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	mediaID := cleanMediaID(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	overrideProvider := s.resolveWorkerProvider(r)

	asset, err := s.svc.Delete(r.Context(), mediaID, overrideProvider)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"mediaId":   asset.ID,
		"status":    asset.Status,
		"deletedAt": asset.DeletedAt,
	})
}

func (s *Server) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		// Client closed connection (context canceled). No need to return 500.
		s.logger.Printf("[WARN]  %s %s => client closed connection (context canceled)", r.Method, r.URL.RequestURI())
		return
	case errors.Is(err, media.ErrNotFound):
		s.writeError(w, r, http.StatusNotFound, "media not found", err)
	case errors.Is(err, media.ErrInvalidFileType):
		s.writeError(w, r, http.StatusUnsupportedMediaType, "unsupported file type", err)
	case errors.Is(err, media.ErrFileTooLarge):
		s.writeError(w, r, http.StatusRequestEntityTooLarge, "file too large", err)
	case errors.Is(err, media.ErrProviderDisabled):
		s.writeError(w, r, http.StatusServiceUnavailable, "provider unavailable", err)
	default:
		if isBadRequestError(err) {
			s.writeError(w, r, http.StatusBadRequest, err.Error(), err)
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "internal server error", err)
	}
}

func isBadRequestError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "required") || strings.Contains(msg, "must be")
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, fmt.Sprintf("encode json failed: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, statusCode int, message string, err error) {
	resp := map[string]any{"error": message}
	if err != nil && statusCode >= 500 {
		resp["detail"] = "see server logs"
		s.logger.Printf("[ERROR] %s %s => %d %s | cause: %v", r.Method, r.URL.RequestURI(), statusCode, message, err)
	}
	if err != nil && statusCode < 500 {
		resp["detail"] = err.Error()
		s.logger.Printf("[WARN]  %s %s => %d %s | detail: %v", r.Method, r.URL.RequestURI(), statusCode, message, err)
	}
	writeJSON(w, statusCode, resp)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		expected := "Bearer " + s.authToken
		if auth != expected {
			// Log the first 12 chars of the received token for debugging (safe partial reveal)
			got := auth
			if len(got) > 12 {
				got = got[:12] + "..."
			}
			s.logger.Printf("[AUTH]  %s %s => 401 unauthorized | got=%q", r.Method, r.URL.RequestURI(), got)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// withCORS handles CORS preflight requests and injects permissive CORS
// headers on every response. The gateway uses Bearer tokens (not cookies),
// so we mirror the worker's policy (workers/src/index.ts handleOptions) by
// reflecting the request Origin, allowing any front-end domain to call the
// API directly. Without this, browsers block cross-origin requests because
// the method-scoped ServeMux answers OPTIONS with 405.
func (s *Server) withCORS(next http.Handler) http.Handler {
	const allowedHeaders = "Authorization, Content-Type, X-Bot-Name, X-Worker-Base-URL, X-Worker-Auth-Token, X-Storage-Mode, X-Project, X-Usage"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, HEAD")
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.logger.Printf("[HTTP]  method=%s path=%s status=%d duration_ms=%d remote=%s",
			r.Method, r.URL.RequestURI(), rw.statusCode, time.Since(start).Milliseconds(), r.RemoteAddr)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

func (s *Server) handleListMedia(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	expected := "Bearer " + s.authToken
	hasAuth := auth == expected

	limit := 20
	offset := 0

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = l
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil {
			offset = o
		}
	}

	filter := media.ListFilter{
		Limit:        limit,
		Offset:       offset,
		Project:      r.URL.Query().Get("project"),
		Usage:        r.URL.Query().Get("usage"),
		Search:       r.URL.Query().Get("search"),
		MediaType:    r.URL.Query().Get("type"),
		OnlyPublic:   !hasAuth,
		PrivateRules: s.privateRules,
	}

	// Hide dummy (< 100 bytes) images: default to true unless explicitly disabled with hide_dummy=false/0
	hideDummyVal := r.URL.Query().Get("hide_dummy")
	hideDummy := true
	if hideDummyVal == "false" || hideDummyVal == "0" {
		hideDummy = false
	}
	filter.HideDummy = hideDummy

	// Status filtering based on auth and parameter
	status := r.URL.Query().Get("status")
	if !hasAuth {
		filter.Status = media.StatusActive
	} else {
		if status == "deleted" {
			filter.Status = media.StatusDeleted
		} else if status == "all" {
			filter.Status = "" // Empty matches all statuses
		} else {
			filter.Status = media.StatusActive // Default to active for admin too
		}
	}

	assets, totalCount, err := s.svc.List(r.Context(), filter)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	var signedAssets []media.Asset
	for _, asset := range assets {
		signedAssets = append(signedAssets, s.signAssetURL(asset))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":  totalCount,
		"limit":  limit,
		"offset": offset,
		"items":  signedAssets,
	})
}

func (s *Server) handleGetProjects(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	expected := "Bearer " + s.authToken
	hasAuth := auth == expected

	onlyPublic := !hasAuth
	projects, err := s.svc.GetProjects(r.Context(), onlyPublic, s.privateRules)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	if projects == nil {
		projects = []string{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleGetUsages(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	expected := "Bearer " + s.authToken
	hasAuth := auth == expected

	onlyPublic := !hasAuth
	usages, err := s.svc.GetUsages(r.Context(), onlyPublic, s.privateRules)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}
	if usages == nil {
		usages = []string{}
	}
	writeJSON(w, http.StatusOK, usages)
}

func (s *Server) handleGetTelegramChatIDs(w http.ResponseWriter, r *http.Request) {
	if s.telegramBotToken == "" {
		s.writeError(w, r, http.StatusBadRequest, "TELEGRAM_BOT_TOKEN is not configured", nil)
		return
	}

	u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", s.telegramBotToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to call telegram getUpdates", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		s.writeError(w, r, resp.StatusCode, fmt.Sprintf("telegram api returned status %d: %s", resp.StatusCode, string(b)), nil)
		return
	}

	var tgResp struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int `json:"update_id"`
			Message  *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"message"`
			ChannelPost *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"channel_post"`
			MyChatMember *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"my_chat_member"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to decode telegram response", err)
		return
	}

	type ChatInfo struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}

	uniqueChats := make(map[int64]ChatInfo)
	for _, res := range tgResp.Result {
		var chat *struct {
			ID        int64  `json:"id"`
			Title     string `json:"title"`
			Type      string `json:"type"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		}
		if res.Message != nil {
			chat = &res.Message.Chat
		} else if res.ChannelPost != nil {
			chat = &res.ChannelPost.Chat
		} else if res.MyChatMember != nil {
			chat = &res.MyChatMember.Chat
		}

		if chat != nil {
			title := chat.Title
			if title == "" {
				if chat.Username != "" {
					title = "@" + chat.Username
				} else if chat.FirstName != "" {
					title = chat.FirstName
					if chat.LastName != "" {
						title += " " + chat.LastName
					}
				} else {
					title = fmt.Sprintf("Chat %d", chat.ID)
				}
			}
			uniqueChats[chat.ID] = ChatInfo{
				ID:    chat.ID,
				Title: title,
				Type:  chat.Type,
			}
		}
	}

	var list []ChatInfo
	for _, info := range uniqueChats {
		list = append(list, info)
	}

	writeJSON(w, http.StatusOK, list)
}

func (s *Server) generateSignature(mediaID string, expires int64) string {
	h := hmac.New(sha256.New, []byte(s.authToken))
	h.Write([]byte(fmt.Sprintf("%s:%d", mediaID, expires)))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Server) isAssetPublic(asset media.Asset) bool {
	if len(s.privateRules) == 0 {
		return true
	}
	for _, rule := range s.privateRules {
		if rule == "*" || rule == "all" {
			return false
		}
		if asset.Project == rule || asset.Usage == rule {
			return false
		}
	}
	return true
}

func (s *Server) signAssetURL(asset media.Asset) media.Asset {
	// Build full public URL: extract path from legacy full URLs, then prepend current PUBLIC_BASE_URL
	path := asset.PublicURL
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if u, err := url.Parse(path); err == nil {
			path = u.Path
		}
	}
	asset.PublicURL = s.publicBaseURL + path

	if s.isAssetPublic(asset) {
		return asset
	}
	// Generate a signature valid for 24 hours
	expires := time.Now().Add(24 * time.Hour).Unix()
	sig := s.generateSignature(asset.ID, expires)

	sep := "?"
	if strings.Contains(asset.PublicURL, "?") {
		sep = "&"
	}
	asset.PublicURL = fmt.Sprintf("%s%stoken_sig=%s&expires=%d", asset.PublicURL, sep, sig, expires)
	return asset
}

func (s *Server) checkAccess(r *http.Request, asset media.Asset) bool {
	if s.isAssetPublic(asset) {
		return true
	}

	// 1. Check Bearer Token
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	expected := "Bearer " + s.authToken
	if auth == expected {
		return true
	}

	// 2. Check pre-signed URL signature
	sig := r.URL.Query().Get("token_sig")
	expiresStr := r.URL.Query().Get("expires")
	if sig != "" && expiresStr != "" {
		expires, err := strconv.ParseInt(expiresStr, 10, 64)
		if err == nil {
			if time.Now().Unix() <= expires {
				expectedSig := s.generateSignature(asset.ID, expires)
				if hmac.Equal([]byte(sig), []byte(expectedSig)) {
					return true
				}
			}
		}
	}

	return false
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

func (s *Server) resolveWorkerProvider(r *http.Request) provider.StorageProvider {
	if strings.ToLower(r.Header.Get("X-Storage-Mode")) != "proxy" && r.Header.Get("X-Worker-Base-URL") == "" {
		return nil
	}
	wURL := strings.TrimSpace(r.Header.Get("X-Worker-Base-URL"))
	wToken := strings.TrimSpace(r.Header.Get("X-Worker-Auth-Token"))
	if wURL == "" {
		wURL = s.workerBaseURL
	}
	if wToken == "" {
		wToken = s.workerAuthToken
	}
	if wURL == "" {
		return nil
	}

	// Sanitize URL and prepends https:// if missing
	wURL = strings.TrimSpace(wURL)
	if !strings.HasPrefix(wURL, "http://") && !strings.HasPrefix(wURL, "https://") {
		wURL = "https://" + wURL
	}
	wURL = strings.TrimRight(wURL, "/")

	cacheKey := wURL + "|" + wToken
	if cached, ok := s.workerProviders.Load(cacheKey); ok {
		return cached.(provider.StorageProvider)
	}
	wp := provider.NewWorkerProvider(wURL, wToken)
	actual, _ := s.workerProviders.LoadOrStore(cacheKey, wp)
	return actual.(provider.StorageProvider)
}

func cleanMediaID(rawID string) string {
	rawID = strings.TrimSpace(rawID)
	if idx := strings.LastIndex(rawID, "."); idx != -1 {
		ext := strings.ToLower(rawID[idx:])
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".avif", ".heic", ".ico",
			".mp4", ".webm", ".mov", ".mkv",
			".ogg", ".mp3", ".wav", ".aac", ".flac", ".m4a",
			".pdf", ".docx", ".doc", ".xlsx", ".xls", ".pptx", ".ppt", ".csv", ".txt",
			".zip", ".rar", ".7z", ".tar", ".gz", ".json", ".yaml", ".yml":
			return rawID[:idx]
		}
	}
	return rawID
}
