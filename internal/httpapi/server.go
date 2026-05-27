package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"firefly-media-gateway/internal/media"
)

const maxRequestBodyBytes int64 = 121 * 1024 * 1024

type Server struct {
	svc       *media.Service
	authToken string
	logger    *log.Logger
}

func NewServer(svc *media.Service, authToken string, logger *log.Logger) *Server {
	return &Server{svc: svc, authToken: authToken, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /debug/ui", s.handleDebugUI)
	mux.HandleFunc("POST /api/v1/media/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("GET /api/v1/media/{mediaId}", s.handleGetMedia)
	mux.HandleFunc("GET /api/v1/media/{mediaId}/meta", s.withAuth(s.handleGetMeta))
	mux.HandleFunc("DELETE /api/v1/media/{mediaId}", s.withAuth(s.handleDelete))

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
			writeError(w, http.StatusRequestEntityTooLarge, "file too large", err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required", err)
		return
	}
	defer file.Close()

	project := strings.TrimSpace(r.FormValue("project"))
	usage := strings.TrimSpace(r.FormValue("usage"))

	asset, err := s.svc.Upload(r.Context(), media.UploadRequest{
		Project:             project,
		Usage:               usage,
		FileName:            header.Filename,
		DeclaredContentType: header.Header.Get("Content-Type"),
		Reader:              file,
	})
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, asset)
}

func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		writeError(w, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	accessURL, err := s.svc.ResolveAccessURL(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	http.Redirect(w, r, accessURL, http.StatusFound)
}

func (s *Server) handleGetMeta(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		writeError(w, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.GetMeta(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, asset)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	mediaID := strings.TrimSpace(r.PathValue("mediaId"))
	if mediaID == "" {
		writeError(w, http.StatusBadRequest, "mediaId is required", nil)
		return
	}

	asset, err := s.svc.Delete(r.Context(), mediaID)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"mediaId":   asset.ID,
		"status":    asset.Status,
		"deletedAt": asset.DeletedAt,
	})
}

func (s *Server) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrNotFound):
		writeError(w, http.StatusNotFound, "media not found", err)
	case errors.Is(err, media.ErrInvalidFileType):
		writeError(w, http.StatusUnsupportedMediaType, "unsupported file type", err)
	case errors.Is(err, media.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "file too large", err)
	case errors.Is(err, media.ErrProviderDisabled):
		writeError(w, http.StatusServiceUnavailable, "provider unavailable", err)
	default:
		if isBadRequestError(err) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error", err)
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

func writeError(w http.ResponseWriter, statusCode int, message string, err error) {
	resp := map[string]any{"error": message}
	if err != nil && statusCode >= 500 {
		resp["detail"] = "see server logs"
	}
	if err != nil && statusCode < 500 {
		resp["detail"] = err.Error()
	}
	writeJSON(w, statusCode, resp)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		expected := "Bearer " + s.authToken
		if auth != expected {
			writeError(w, http.StatusUnauthorized, "unauthorized", nil)
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
		s.logger.Printf("method=%s path=%s status=%d duration_ms=%d remote=%s", r.Method, r.URL.Path, rw.statusCode, time.Since(start).Milliseconds(), r.RemoteAddr)
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
