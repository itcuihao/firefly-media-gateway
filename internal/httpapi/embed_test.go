package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendHandler(t *testing.T) {
	srv := &Server{}
	handler := srv.frontendHandler()

	t.Run("root path returns index.html with no-cache", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		cc := rr.Header().Get("Cache-Control")
		if !strings.Contains(cc, "no-cache") || !strings.Contains(cc, "no-store") {
			t.Errorf("expected no-cache Cache-Control, got %q", cc)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "<!doctype html>") && !strings.Contains(body, "<html") {
			t.Errorf("expected HTML body, got %q", body)
		}
	})

	t.Run("index.html redirects to canonical root or returns with no-cache", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/index.html", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// http.FileServer redirects /index.html to ./ (301)
		if rr.Code != http.StatusOK && rr.Code != http.StatusMovedPermanently {
			t.Fatalf("expected 200 or 301, got %d", rr.Code)
		}
		cc := rr.Header().Get("Cache-Control")
		if !strings.Contains(cc, "no-cache") {
			t.Errorf("expected no-cache Cache-Control, got %q", cc)
		}
	})

	t.Run("spa route fallbacks to index.html with no-cache", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboard", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		cc := rr.Header().Get("Cache-Control")
		if !strings.Contains(cc, "no-cache") {
			t.Errorf("expected no-cache Cache-Control for SPA route, got %q", cc)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "<!doctype html>") && !strings.Contains(body, "<html") {
			t.Errorf("expected HTML body, got %q", body)
		}
	})

	t.Run("missing static asset returns 404 instead of index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/assets/non-existent-file.js", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for missing JS asset, got %d", rr.Code)
		}
	})

	t.Run("missing file with extension returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/non-existent.png", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for missing file with extension, got %d", rr.Code)
		}
	})
}
