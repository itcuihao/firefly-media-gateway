package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"firefly-media-gateway/internal/media"
	"firefly-media-gateway/internal/provider"
	"firefly-media-gateway/internal/s3"
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

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestVideoAutoChunkingAndStreamingIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Setup in-memory SQLite repo
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := storage.EnsureSQLiteSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	repo := storage.NewSQLiteRepository(db)

	// 2. Setup mock provider & Service
	memProv := newMemoryProvider()
	svc := media.NewService(repo, map[string]provider.StorageProvider{"mem": memProv}, "mem", "http://localhost:8080")
	svc.SetUploadConcurrency(3)

	logger := log.New(io.Discard, "", 0)
	apiServer := NewServer(svc, "test-token", "", "", "", "http://localhost:8080", nil, "sqlite", "direct", "test-commit", logger)
	s3Gateway := s3.NewGateway(svc, "http://localhost:8080")

	// Intercept outgoing HTTP calls from Server.proxyChunkedMedia
	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasPrefix(req.URL.Host, "mock-storage.internal") {
			fileID := strings.TrimPrefix(req.URL.Path, "/")
			memProv.mu.Lock()
			data, ok := memProv.files[fileID]
			memProv.mu.Unlock()
			if !ok {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}

			rangeHeader := req.Header.Get("Range")
			if strings.HasPrefix(rangeHeader, "bytes=") {
				rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
				parts := strings.Split(rangeStr, "-")
				start, _ := strconv.ParseInt(parts[0], 10, 64)
				end, _ := strconv.ParseInt(parts[1], 10, 64)
				if end >= int64(len(data)) {
					end = int64(len(data)) - 1
				}
				slice := data[start : end+1]

				h := make(http.Header)
				h.Set("Content-Type", "application/octet-stream")
				h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				h.Set("Content-Length", strconv.Itoa(len(slice)))
				return &http.Response{
					StatusCode:    http.StatusPartialContent,
					Body:          io.NopCloser(bytes.NewReader(slice)),
					Header:        h,
					ContentLength: int64(len(slice)),
				}, nil
			}

			h := make(http.Header)
			h.Set("Content-Type", "application/octet-stream")
			h.Set("Content-Length", strconv.Itoa(len(data)))
			return &http.Response{
				StatusCode:    http.StatusOK,
				Body:          io.NopCloser(bytes.NewReader(data)),
				Header:        h,
				ContentLength: int64(len(data)),
			}, nil
		}
		return nil, fmt.Errorf("unexpected host: %s", req.URL.Host)
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/s3/") {
			s3Gateway.Handler().ServeHTTP(w, r)
		} else {
			apiServer.Handler().ServeHTTP(w, r)
		}
	})

	// ==========================================
	// Test Case 1: Upload small video (1MB) -> single upload, isChunked == false
	// ==========================================
	t.Run("Small video upload (1MB) without member flag", func(t *testing.T) {
		smallVideoData := make([]byte, 1024*1024)
		for i := range smallVideoData {
			smallVideoData[i] = byte(i % 251)
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("project", "test-proj")
		_ = writer.WriteField("usage", "scene")
		// No member or is_member field!
		part, _ := writer.CreateFormFile("file", "small.mov")
		_, _ = part.Write(smallVideoData)
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer test-token")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var created media.Asset
		if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if created.IsChunked {
			t.Errorf("expected 1MB video to be single upload (isChunked=false), got true")
		}
	})

	// ==========================================
	// Test Case 2: Upload large video (16MB) -> auto-chunked into 2 chunks, isChunked == true
	// ==========================================
	var largeAssetID string
	var largeVideoData []byte

	t.Run("Large video upload (16MB) auto-chunking without member flag", func(t *testing.T) {
		const totalSize = 16 * 1024 * 1024
		largeVideoData = make([]byte, totalSize)
		for i := range largeVideoData {
			largeVideoData[i] = byte((i*37 + 11) % 256)
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("project", "test-proj")
		_ = writer.WriteField("usage", "scene")
		// No member or is_member field passed!
		part, _ := writer.CreateFormFile("file", "large.mov")
		_, _ = part.Write(largeVideoData)
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/media/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer test-token")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var created media.Asset
		if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !created.IsChunked {
			t.Fatalf("expected 16MB video to be auto-chunked (isChunked=true), got false")
		}

		largeAssetID = created.ID

		// Verify chunks in DB
		chunks, err := repo.GetChunks(ctx, created.ID)
		if err != nil {
			t.Fatalf("get chunks: %v", err)
		}
		if len(chunks) != 2 {
			t.Fatalf("expected 2 chunks, got %d", len(chunks))
		}
	})

	// ==========================================
	// Test Case 3: HTTP Range streaming on chunked video across chunk boundaries
	// ==========================================
	t.Run("HTTP Range streaming on chunked video", func(t *testing.T) {
		if largeAssetID == "" {
			t.Fatal("largeAssetID is empty, skip streaming test")
		}

		// 3a. Range request within first chunk: bytes 100-199
		req1 := httptest.NewRequest("GET", "/api/v1/media/"+largeAssetID+"/stream", nil)
		req1.Header.Set("Range", "bytes=100-199")
		req1.Header.Set("Authorization", "Bearer test-token")
		rr1 := httptest.NewRecorder()
		handler.ServeHTTP(rr1, req1)

		if rr1.Code != http.StatusPartialContent {
			t.Fatalf("expected 206 Partial Content, got %d", rr1.Code)
		}
		if !bytes.Equal(rr1.Body.Bytes(), largeVideoData[100:200]) {
			t.Fatalf("streamed bytes do not match expected slice")
		}

		// 3b. Range request SPANNING CHUNK BOUNDARY (chunkSize is 15MB = 15728640 bytes)
		// Request 20 bytes from 15728630 to 15728649 (10 bytes in chunk 0, 10 bytes in chunk 1)
		const start = int64(15*1024*1024 - 10)
		const end = int64(15*1024*1024 + 9)
		req2 := httptest.NewRequest("GET", "/api/v1/media/"+largeAssetID+"/stream", nil)
		req2.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
		req2.Header.Set("Authorization", "Bearer test-token")
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)

		if rr2.Code != http.StatusPartialContent {
			t.Fatalf("expected 206 Partial Content across boundary, got %d", rr2.Code)
		}
		expectedSlice := largeVideoData[start : end+1]
		if !bytes.Equal(rr2.Body.Bytes(), expectedSlice) {
			t.Fatalf("cross-chunk boundary streamed bytes do not match original video slice!")
		}
	})

	// ==========================================
	// Test Case 4: S3 Gateway PUT with large video (>15MB) auto-chunking
	// ==========================================
	t.Run("S3 Gateway PutObject with 16MB video", func(t *testing.T) {
		const s3Size = 16 * 1024 * 1024
		s3Data := make([]byte, s3Size)
		for i := range s3Data {
			s3Data[i] = byte((i + 7) % 256)
		}

		s3Req := httptest.NewRequest("PUT", "/s3/testbucket/testproj/scene/movie.mov", bytes.NewReader(s3Data))
		s3Req.Header.Set("Content-Type", "video/quicktime")
		s3RR := httptest.NewRecorder()
		handler.ServeHTTP(s3RR, s3Req)

		if s3RR.Code != http.StatusOK {
			t.Fatalf("expected S3 PUT 200 OK, got %d, body: %s", s3RR.Code, s3RR.Body.String())
		}

		// Verify asset created by S3
		assets, err := repo.List(ctx, media.ListFilter{Project: "testproj", Limit: 10})
		if err != nil {
			t.Fatalf("list assets: %v", err)
		}

		var s3Asset *media.Asset
		for _, a := range assets {
			if a.SizeBytes == s3Size {
				s3Asset = &a
				break
			}
		}
		if s3Asset == nil {
			t.Fatalf("expected to find S3 uploaded asset with size %d", s3Size)
		}
		if !s3Asset.IsChunked {
			t.Errorf("expected S3 uploaded 16MB video to be auto-chunked (isChunked=true)")
		}

		s3Chunks, err := repo.GetChunks(ctx, s3Asset.ID)
		if err != nil {
			t.Fatalf("get S3 chunks: %v", err)
		}
		if len(s3Chunks) != 2 {
			t.Errorf("expected 2 chunks for S3 16MB video, got %d", len(s3Chunks))
		}
	})
}
