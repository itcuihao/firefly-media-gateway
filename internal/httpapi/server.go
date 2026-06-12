package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
)

const maxRequestBodyBytes int64 = 2005 * 1024 * 1024 // 2GB + 5MB buffer

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
	logger           *log.Logger
}

func NewServer(svc *media.Service, authToken, telegramBotToken, workerBaseURL, workerAuthToken, publicBaseURL string, privateRules []string, databaseDriver, storageMode string, logger *log.Logger) *Server {
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
		logger:           logger,
	}
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
	mux.HandleFunc("GET /api/v1/media", s.handleListMedia)
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

	return s.withLogging(mux)
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
		"database_driver": s.databaseDriver,
		"storage_driver":  storageMode,
		"worker_url":      workerURL,
	})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			s.writeError(w, r, http.StatusRequestEntityTooLarge, "file too large", err)
			return
		}
		s.writeError(w, r, http.StatusBadRequest, "invalid multipart form", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "file is required", err)
		return
	}
	defer file.Close()

	project := strings.TrimSpace(r.FormValue("project"))
	usage := strings.TrimSpace(r.FormValue("usage"))
	isMember := r.FormValue("member") == "true" || r.FormValue("is_member") == "true"

	overrideProvider := s.resolveWorkerProvider(r)

	asset, err := s.svc.Upload(r.Context(), media.UploadRequest{
		Project:             project,
		Usage:               usage,
		FileName:            header.Filename,
		DeclaredContentType: header.Header.Get("Content-Type"),
		Reader:              file,
		IsMember:            isMember,
		OverrideProvider:    overrideProvider,
	})
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

	if streamInfo.IsChunked {
		s.proxyChunkedMedia(w, r, streamInfo)
		return
	}

	s.proxySingleMedia(w, r, streamInfo)
}

func (s *Server) proxySingleMedia(w http.ResponseWriter, r *http.Request, streamInfo media.StreamInfo) {
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

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Printf("[WARN]  stream media failed: %v", err)
	}
}

func (s *Server) proxyChunkedMedia(w http.ResponseWriter, r *http.Request, streamInfo media.StreamInfo) {
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
		w.WriteHeader(http.StatusPartialContent)
	} else {
		if streamInfo.TotalBytes > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(streamInfo.TotalBytes, 10))
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
	}

	for i, chunkURL := range streamInfo.ChunkURLs {
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

		// Calculate relative start/end within this chunk
		relativeStart := start - chunkStart
		if relativeStart < 0 {
			relativeStart = 0
		}
		relativeEnd := end - chunkStart
		if relativeEnd >= chunkSizeBytes {
			relativeEnd = chunkSizeBytes - 1
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, chunkURL, nil)
		if err != nil {
			s.logger.Printf("[WARN]  build chunk request failed: %v", err)
			return
		}
		for k, v := range streamInfo.Headers {
			req.Header.Set(k, v)
		}
		// Request the specific range from the storage provider
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", relativeStart, relativeEnd))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			s.logger.Printf("[WARN]  fetch chunk range failed: %v", err)
			return
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			s.logger.Printf("[WARN]  fetch chunk returned status=%d", resp.StatusCode)
			return
		}

		_, copyErr := io.Copy(w, resp.Body)
		resp.Body.Close()

		if copyErr != nil {
			s.logger.Printf("[WARN]  write chunk failed: %v", copyErr)
			return
		}
	}
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
		if r.start >= size {
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

	assets, err := s.svc.List(r.Context(), limit, offset)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	var signedAssets []media.Asset
	for _, asset := range assets {
		isPublic := s.isAssetPublic(asset)
		if !isPublic && !hasAuth {
			// Skip private assets for unauthenticated visitors
			continue
		}
		signedAssets = append(signedAssets, s.signAssetURL(asset))
	}

	writeJSON(w, http.StatusOK, signedAssets)
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
	
	return provider.NewWorkerProvider(wURL, wToken)
}

func cleanMediaID(rawID string) string {
	rawID = strings.TrimSpace(rawID)
	if idx := strings.LastIndex(rawID, "."); idx != -1 {
		ext := strings.ToLower(rawID[idx:])
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".mp4", ".webm", ".mov", ".ogg", ".mp3", ".wav":
			return rawID[:idx]
		}
	}
	return rawID
}
