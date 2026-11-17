package httpapi

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
)

type mockRepository struct {
	asset media.Asset
}

func (r *mockRepository) Create(ctx context.Context, input media.CreateAssetInput) (media.Asset, error) {
	return r.asset, nil
}
func (r *mockRepository) GetByID(ctx context.Context, id string) (media.Asset, error) {
	return r.asset, nil
}
func (r *mockRepository) GetActiveBySHA256(ctx context.Context, sha256 string) (media.Asset, error) {
	return r.asset, nil
}
func (r *mockRepository) CountActiveBySHA256(ctx context.Context, sha256 string) (int, error) {
	return 1, nil
}
func (r *mockRepository) MarkDeleted(ctx context.Context, id string) (media.Asset, error) {
	return r.asset, nil
}
func (r *mockRepository) List(ctx context.Context, filter media.ListFilter) ([]media.Asset, error) {
	return []media.Asset{r.asset}, nil
}
func (r *mockRepository) Count(ctx context.Context, filter media.ListFilter) (int, error) {
	return 1, nil
}
func (r *mockRepository) GetStats(ctx context.Context, filter media.ListFilter) (media.MediaStats, error) {
	return media.MediaStats{
		TotalCount: 1,
		TotalBytes: r.asset.SizeBytes,
		Images: media.MediaTypeStats{
			Count:   1,
			Bytes:   r.asset.SizeBytes,
			Percent: 100,
		},
	}, nil
}
func (r *mockRepository) GetDistinctProjects(ctx context.Context, onlyPublic bool, privateRules []string, allowedProjects []string) ([]string, error) {
	return nil, nil
}
func (r *mockRepository) GetDistinctUsages(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
	return nil, nil
}
func (r *mockRepository) SaveChunks(ctx context.Context, assetID string, chunks []media.Chunk) error {
	return nil
}
func (r *mockRepository) GetChunks(ctx context.Context, assetID string) ([]media.Chunk, error) {
	return nil, nil
}
func (r *mockRepository) DeleteChunks(ctx context.Context, assetID string) error {
	return nil
}

type mockProvider struct {
	url string
}

func (p *mockProvider) Name() string {
	return "mock"
}
func (p *mockProvider) Upload(ctx context.Context, in provider.UploadInput) (provider.UploadResult, error) {
	return provider.UploadResult{}, nil
}
func (p *mockProvider) GetAccess(ctx context.Context, providerFileID string, bucketOrChat *string) (provider.AccessResult, error) {
	return provider.AccessResult{
		URL: p.url,
	}, nil
}
func (p *mockProvider) Delete(ctx context.Context, fileID string, bucketOrChat *string) error {
	return nil
}

func TestDynamicWebPConversion(t *testing.T) {
	// 1. Create a dummy PNG image
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 255, 0, 255}}, image.Point{}, draw.Src)

	// 2. Start a test server that serves this PNG
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer ts.Close()

	// 3. Clean up cache dir
	cacheDir := "data/cache"
	_ = os.RemoveAll(cacheDir)
	defer os.RemoveAll(cacheDir)

	// 4. Setup mock repository and provider
	repo := &mockRepository{
		asset: media.Asset{
			ID:        "test-img",
			Provider:  "mock",
			MIMEType:  "image/png",
			SizeBytes: 100,
			Status:    media.StatusActive,
		},
	}
	prov := &mockProvider{url: ts.URL}
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mock": prov}, "mock", "http://localhost:8080")

	logger := log.New(io.Discard, "", 0)
	srv := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", "test-commit", logger)

	// 5. Test Case 1: Request with Accept: image/webp
	req := httptest.NewRequest("GET", "/api/v1/media/test-img", nil)
	req.Header.Set("Accept", "image/webp")
	req.SetPathValue("mediaId", "test-img")

	rr := httptest.NewRecorder()
	srv.serveMediaBinary(rr, req)

	resp := rr.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "image/webp" {
		t.Errorf("expected Content-Type image/webp, got %q", contentType)
	}

	// Verify cache file was created
	cacheFile := filepath.Join(cacheDir, "test-img.webp")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("expected cache file %q to be created, but it was not", cacheFile)
	}

	// Verify Cache-Control header
	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("expected Cache-Control to contain immutable, got %q", cc)
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Errorf("expected ETag header to be set, got empty")
	}

	// Test Case 1b: If-None-Match 304 Not Modified
	req304 := httptest.NewRequest("GET", "/api/v1/media/test-img", nil)
	req304.Header.Set("Accept", "image/webp")
	req304.Header.Set("If-None-Match", etag)
	req304.SetPathValue("mediaId", "test-img")
	rr304 := httptest.NewRecorder()
	srv.serveMediaBinary(rr304, req304)
	if rr304.Code != http.StatusNotModified {
		t.Errorf("expected status 304 Not Modified, got %d", rr304.Code)
	}

	// 6. Test Case 2: Request with ?raw=true
	reqRaw := httptest.NewRequest("GET", "/api/v1/media/test-img?raw=true", nil)
	reqRaw.Header.Set("Accept", "image/webp")
	reqRaw.SetPathValue("mediaId", "test-img")

	rrRaw := httptest.NewRecorder()
	srv.serveMediaBinary(rrRaw, reqRaw)

	respRaw := rrRaw.Result()
	if respRaw.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", respRaw.StatusCode)
	}

	// The Content-Type should NOT be image/webp, it should be parsed from the proxy target (image/png)
	contentTypeRaw := respRaw.Header.Get("Content-Type")
	if strings.Contains(contentTypeRaw, "webp") {
		t.Errorf("expected raw PNG, got %q", contentTypeRaw)
	}
}

func TestSingleflightWebPConversion(t *testing.T) {
	cacheDir := "data/cache"
	_ = os.RemoveAll(cacheDir)
	defer os.RemoveAll(cacheDir)

	repo := &mockRepository{
		asset: media.Asset{
			ID:        "concurrent-img",
			Provider:  "mock",
			MIMEType:  "image/png",
			SizeBytes: 100,
			Status:    media.StatusActive,
		},
	}
	prov := &mockProvider{}

	var convertCount int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&convertCount, 1)
		time.Sleep(30 * time.Millisecond) // simulate upstream network latency

		img := image.NewRGBA(image.Rect(0, 0, 50, 50))
		for x := 0; x < 50; x++ {
			for y := 0; y < 50; y++ {
				img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 5), B: 200, A: 255})
			}
		}
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer mockServer.Close()
	prov.url = mockServer.URL

	svc := media.NewService(repo, map[string]provider.StorageProvider{"mock": prov}, "mock", "http://localhost:8080")
	logger := log.New(io.Discard, "", 0)
	srv := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", "test-commit", logger)

	streamInfo := media.StreamInfo{
		StreamURL: mockServer.URL + "/test.png",
	}

	// Send 10 concurrent requests for the exact same uncached mediaID
	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/api/v1/media/concurrent-img", nil)
			req.Header.Set("Accept", "image/webp")
			rr := httptest.NewRecorder()
			success := srv.tryServeWebPCache(rr, req, "concurrent-img", streamInfo, "public, max-age=31536000, immutable")
			if !success {
				t.Errorf("tryServeWebPCache failed")
			}
		}()
	}

	wg.Wait()

	// Singleflight must coalesce the 10 requests into at most 1 upstream call
	if got := atomic.LoadInt32(&convertCount); got != 1 {
		t.Errorf("expected singleflight to coalesce into 1 upstream call, got %d", got)
	}
}

func TestHeadRequestFastPath(t *testing.T) {
	repo := &mockRepository{
		asset: media.Asset{
			ID:        "head-test-media",
			Provider:  "mock",
			MIMEType:  "video/mp4",
			SizeBytes: 10485760, // 10MB
			Status:    media.StatusActive,
		},
	}
	var upstreamCalled int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalled, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	prov := &mockProvider{url: mockServer.URL}
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mock": prov}, "mock", "http://localhost:8080")
	logger := log.New(io.Discard, "", 0)
	srv := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", "test-commit", logger)

	req := httptest.NewRequest("HEAD", "/api/v1/media/head-test-media", nil)
	req.SetPathValue("mediaId", "head-test-media")
	rr := httptest.NewRecorder()

	srv.serveMediaBinary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD request, got %d", rr.Code)
	}
	if cl := rr.Header().Get("Content-Length"); cl != "10485760" {
		t.Errorf("expected Content-Length 10485760, got %q", cl)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("expected Content-Type video/mp4, got %q", ct)
	}
	if ar := rr.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("expected Accept-Ranges bytes, got %q", ar)
	}
	if bodyLen := rr.Body.Len(); bodyLen != 0 {
		t.Errorf("expected empty body for HEAD request, got %d bytes", bodyLen)
	}
	if calls := atomic.LoadInt32(&upstreamCalled); calls != 0 {
		t.Errorf("expected 0 upstream data calls for HEAD request, got %d", calls)
	}
}

func TestPrivateAssetCacheControl(t *testing.T) {
	repo := &mockRepository{
		asset: media.Asset{
			ID:        "secret-doc",
			Provider:  "mock",
			MIMEType:  "application/pdf",
			SizeBytes: 2048,
			Project:   "internal-secret",
			Status:    media.StatusActive,
		},
	}
	prov := &mockProvider{url: "http://mock.url"}
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mock": prov}, "mock", "http://localhost:8080")
	logger := log.New(io.Discard, "", 0)

	// Configure private rule matching project "internal-secret"
	srv := NewServer(svc, "secret-token", "", "", "", "http://localhost:8080", []string{"internal-secret"}, "sqlite", "direct", "test-commit", logger)

	// Authorized HEAD request to verify private Cache-Control
	req := httptest.NewRequest("HEAD", "/api/v1/media/secret-doc", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.SetPathValue("mediaId", "secret-doc")
	rr := httptest.NewRecorder()

	srv.serveMediaBinary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authorized access, got %d", rr.Code)
	}
	cc := rr.Header().Get("Cache-Control")
	if !strings.Contains(cc, "private") || strings.Contains(cc, "public") {
		t.Errorf("expected private Cache-Control for private asset, got %q", cc)
	}
	if xab := rr.Header().Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("expected X-Accel-Buffering: no, got %q", xab)
	}
}

func TestCORSAndStreamingHeaders(t *testing.T) {
	repo := &mockRepository{
		asset: media.Asset{
			ID:        "sample-media",
			Provider:  "mock",
			MIMEType:  "image/jpeg",
			SizeBytes: 1024,
			Status:    media.StatusActive,
		},
	}
	prov := &mockProvider{url: "http://mock.url"}
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mock": prov}, "mock", "http://localhost:8080")
	logger := log.New(io.Discard, "", 0)
	srv := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", "test-commit", logger)
	handler := srv.Handler()

	// 1. OPTIONS preflight
	optReq := httptest.NewRequest(http.MethodOptions, "/api/v1/media/sample-media", nil)
	optReq.Header.Set("Origin", "https://example.com")
	optReq.Header.Set("Access-Control-Request-Method", "GET")
	optRR := httptest.NewRecorder()
	handler.ServeHTTP(optRR, optReq)

	if optRR.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for OPTIONS preflight, got %d", optRR.Code)
	}
	if origin := optRR.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %q", origin)
	}
	if maxAge := optRR.Header().Get("Access-Control-Max-Age"); maxAge != "86400" {
		t.Errorf("expected Access-Control-Max-Age: 86400, got %q", maxAge)
	}

	// 2. GET health check (non-media route) should also have wildcard CORS and exposed headers
	healthReq := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	healthRR := httptest.NewRecorder()
	handler.ServeHTTP(healthRR, healthReq)

	if origin := healthRR.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: * on health route, got %q", origin)
	}
	expose := healthRR.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(expose, "Content-Range") || !strings.Contains(expose, "ETag") {
		t.Errorf("expected Access-Control-Expose-Headers to include Content-Range and ETag, got %q", expose)
	}
	if vary := healthRR.Header().Get("Vary"); strings.Contains(vary, "Origin") {
		t.Errorf("expected no Vary: Origin header with wildcard CORS, got %q", vary)
	}

	// 3. GET /api/v1/media/stats
	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/media/stats", nil)
	statsRR := httptest.NewRecorder()
	handler.ServeHTTP(statsRR, statsReq)

	if statsRR.Code != http.StatusOK {
		t.Errorf("expected status 200 for /api/v1/media/stats, got %d", statsRR.Code)
	}
	if !strings.Contains(statsRR.Body.String(), `"total_count":1`) {
		t.Errorf("expected total_count: 1 in stats response, got %s", statsRR.Body.String())
	}

	// 4. GET /api/v1/auth/verify without token -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify", nil)
	unauthRR := httptest.NewRecorder()
	handler.ServeHTTP(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for unauthenticated /api/v1/auth/verify, got %d", unauthRR.Code)
	}

	// 5. GET /api/v1/auth/verify with correct Bearer token -> 200
	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/verify", nil)
	authReq.Header.Set("Authorization", "Bearer test-token")
	authRR := httptest.NewRecorder()
	handler.ServeHTTP(authRR, authReq)
	if authRR.Code != http.StatusOK {
		t.Errorf("expected status 200 for authenticated /api/v1/auth/verify, got %d", authRR.Code)
	}
	if !strings.Contains(authRR.Body.String(), `"authenticated":true`) {
		t.Errorf("expected authenticated: true in response, got %s", authRR.Body.String())
	}
}
