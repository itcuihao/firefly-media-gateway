package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"firefly-media-gateway/internal/media"
)

const maxRequestBodyBytes int64 = 201 * 1024 * 1024 // 200MB + 1MB buffer

type Server struct {
	svc              *media.Service
	authToken        string
	telegramBotToken string
	logger           *log.Logger
}

func NewServer(svc *media.Service, authToken, telegramBotToken string, logger *log.Logger) *Server {
	return &Server{
		svc:              svc,
		authToken:        authToken,
		telegramBotToken: telegramBotToken,
		logger:           logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /debug/ui", s.handleDebugUI)
	mux.HandleFunc("POST /api/v1/media/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("GET /api/v1/media", s.withAuth(s.handleListMedia))
	mux.HandleFunc("GET /api/v1/media/{mediaId}", s.handleGetMedia)
	mux.HandleFunc("GET /api/v1/media/{mediaId}/meta", s.withAuth(s.handleGetMeta))
	mux.HandleFunc("GET /api/v1/media/{mediaId}/stream", s.withAuth(s.handleStream))
	mux.HandleFunc("DELETE /api/v1/media/{mediaId}", s.withAuth(s.handleDelete))
	mux.HandleFunc("GET /api/v1/telegram/chat-ids", s.withAuth(s.handleGetTelegramChatIDs))

	return s.withLogging(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
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

	asset, err := s.svc.Upload(r.Context(), media.UploadRequest{
		Project:             project,
		Usage:               usage,
		FileName:            header.Filename,
		DeclaredContentType: header.Header.Get("Content-Type"),
		Reader:              file,
		IsMember:            isMember,
	})
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, asset)
}

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	accessURL, err := s.svc.ResolveAccessURL(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	http.Redirect(w, r, accessURL, http.StatusFound)
}

func (s *Server) handleGetMeta(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.GetMeta(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	streamInfo, err := s.svc.StreamAsset(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, streamInfo)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		s.writeError(w, r, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.Delete(r.Context(), mediaID)
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

	writeJSON(w, http.StatusOK, assets)
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
