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
	"testing"

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
	srv := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", logger)

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
