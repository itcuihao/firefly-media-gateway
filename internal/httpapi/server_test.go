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
func (r *mockRepository) GetDistinctProjects(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
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
			success := srv.tryServeWebPCache(rr, req, "concurrent-img", streamInfo)
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
