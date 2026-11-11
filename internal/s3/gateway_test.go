package s3

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
	"firefly-media-gateway/internal/storage"

	_ "modernc.org/sqlite"
)

type memoryProvider struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMemoryProvider() *memoryProvider {
	return &memoryProvider{files: make(map[string][]byte)}
}

func (p *memoryProvider) Name() string { return "mem" }

func (p *memoryProvider) Upload(ctx context.Context, in provider.UploadInput) (provider.UploadResult, error) {
	data, err := io.ReadAll(in.Reader)
	if err != nil {
		return provider.UploadResult{}, err
	}
	p.mu.Lock()
	fileID := fmt.Sprintf("file-%d", len(p.files)+1)
	p.files[fileID] = data
	p.mu.Unlock()

	loc := "mem-bucket"
	return provider.UploadResult{
		ProviderFileID:       fileID,
		ProviderBucketOrChat: &loc,
	}, nil
}

func (p *memoryProvider) Delete(ctx context.Context, providerFileID string, bucketOrChat *string) error {
	p.mu.Lock()
	delete(p.files, providerFileID)
	p.mu.Unlock()
	return nil
}

func (p *memoryProvider) GetAccess(ctx context.Context, providerFileID string, bucketOrChat *string) (provider.AccessResult, error) {
	return provider.AccessResult{
		URL: fmt.Sprintf("http://mock-storage.internal/%s", providerFileID),
	}, nil
}

func setupTestGateway(t *testing.T, authToken string) (*Gateway, *media.Service) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := storage.EnsureSQLiteSchema(context.Background(), db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	repo := storage.NewSQLiteRepository(db)
	memProv := newMemoryProvider()
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mem": memProv}, "mem", "http://localhost:8080")

	gw := NewGateway(svc, "http://localhost:8080", authToken)
	return gw, svc
}

func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for x := 0; x < 40; x++ {
		for y := 0; y < 40; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 5), B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestS3ListBuckets(t *testing.T) {
	gw, _ := setupTestGateway(t, "")
	handler := gw.Handler()

	for _, path := range []string{"/s3", "/s3/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s expected 200, got %d", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<ListAllMyBucketsResult") {
			t.Errorf("GET %s missing <ListAllMyBucketsResult>: %s", path, body)
		}
	}
}

func TestS3BucketOperations(t *testing.T) {
	gw, _ := setupTestGateway(t, "secret-token")
	handler := gw.Handler()

	// HEAD /s3/mybucket (unauthenticated is fine for HEAD)
	req := httptest.NewRequest(http.MethodHead, "/s3/mybucket", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /s3/mybucket expected 200, got %d", rec.Code)
	}

	// PUT /s3/mybucket without auth -> 403
	req = httptest.NewRequest(http.MethodPut, "/s3/mybucket", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("PUT /s3/mybucket without auth expected 403, got %d", rec.Code)
	}

	// PUT /s3/mybucket with auth (CreateBucket) -> 200
	req = httptest.NewRequest(http.MethodPut, "/s3/mybucket", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("PUT /s3/mybucket with auth expected 200, got %d", rec.Code)
	}
}

func TestS3Auth(t *testing.T) {
	gw, _ := setupTestGateway(t, "secret-token")
	handler := gw.Handler()
	pngData := createTestPNG()

	// 1. Unauthenticated PUT should fail with 403
	req := httptest.NewRequest(http.MethodPut, "/s3/default/test.png", bytes.NewBuffer(pngData))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unauth PUT, got %d", rec.Code)
	}

	// 2. Bearer token in Authorization header
	req = httptest.NewRequest(http.MethodPut, "/s3/default/test.png", bytes.NewBuffer(pngData))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with Bearer auth, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. SigV4 format in Authorization header
	req = httptest.NewRequest(http.MethodPut, "/s3/default/test2.png", bytes.NewBuffer(pngData))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=secret-token/20261111/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=mock")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with SigV4 auth, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Query param auth_token
	req = httptest.NewRequest(http.MethodPut, "/s3/default/test3.png?auth_token=secret-token", bytes.NewBuffer(pngData))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with ?auth_token=, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestS3PutGetHeadDeleteObject(t *testing.T) {
	gw, _ := setupTestGateway(t, "")
	handler := gw.Handler()
	pngData := createTestPNG()

	// 1. PUT 1-part key: sample.png
	req := httptest.NewRequest(http.MethodPut, "/s3/mybucket/sample.png", bytes.NewBuffer(pngData))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /s3/mybucket/sample.png failed with %d: %s", rec.Code, rec.Body.String())
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Errorf("expected ETag in PUT response")
	}

	// 2. PUT 3-part key: project/usage/avatar.jpg
	req = httptest.NewRequest(http.MethodPut, "/s3/mybucket/sheep/avatar/user1.png", bytes.NewBuffer(pngData))
	req.Header.Set("Content-Type", "image/png")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT 3-part key failed with %d: %s", rec.Code, rec.Body.String())
	}

	// 3. HEAD Object by key without ?asset_id=
	req = httptest.NewRequest(http.MethodHead, "/s3/mybucket/sample.png", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /s3/mybucket/sample.png expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", rec.Header().Get("Content-Type"))
	}

	// 4. GET Object by key without ?asset_id= (should redirect to access URL)
	req = httptest.NewRequest(http.MethodGet, "/s3/mybucket/sample.png", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect && rec.Code != http.StatusFound {
		t.Errorf("GET /s3/mybucket/sample.png expected 307 or 302 redirect, got %d", rec.Code)
	}

	// 5. GET /s3/mybucket (ListObjects)
	req = httptest.NewRequest(http.MethodGet, "/s3/mybucket", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ListObjects expected 200, got %d", rec.Code)
	}
	listXml := rec.Body.String()
	if !strings.Contains(listXml, "<ListBucketResult") {
		t.Errorf("missing <ListBucketResult>: %s", listXml)
	}

	// 6. DELETE Object by key without ?asset_id=
	req = httptest.NewRequest(http.MethodDelete, "/s3/mybucket/sample.png", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("DELETE expected 204 No Content, got %d", rec.Code)
	}

	// 7. DELETE non-existent object should also return 204 per S3 spec
	req = httptest.NewRequest(http.MethodDelete, "/s3/mybucket/non-existent-key.png", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("DELETE non-existent expected 204, got %d", rec.Code)
	}
}

func TestS3MultipartUpload(t *testing.T) {
	gw, _ := setupTestGateway(t, "")
	handler := gw.Handler()

	// 1. POST ?uploads (InitiateMultipartUpload)
	req := httptest.NewRequest(http.MethodPost, "/s3/mybucket/bigfile.mp4?uploads", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST ?uploads expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<InitiateMultipartUploadResult") || !strings.Contains(body, "<UploadId>") {
		t.Errorf("unexpected InitiateMultipartUpload response: %s", body)
	}

	// 2. POST ?uploadId=xxx (CompleteMultipartUpload)
	req = httptest.NewRequest(http.MethodPost, "/s3/mybucket/bigfile.mp4?uploadId=upload-12345", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST ?uploadId expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	if !strings.Contains(body, "<CompleteMultipartUploadResult") {
		t.Errorf("unexpected CompleteMultipartUpload response: %s", body)
	}
}
