package httpapi

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiskCacheGetOrFetchChunk(t *testing.T) {
	tempDir := t.TempDir()
	logger := log.New(io.Discard, "", 0)
	dc, err := NewDiskCache(tempDir, 10*1024*1024, logger)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	mockChunkData := []byte("hello-chunk-data-1234567890")
	var fetchCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(mockChunkData)
	}))
	defer server.Close()

	ctx := context.Background()
	chunkKey := "test_chunk_0.bin"

	// 1. First fetch: cache miss -> downloads from mock server
	path1, err := dc.GetOrFetchChunk(ctx, chunkKey, int64(len(mockChunkData)), server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error on first fetch: %v", err)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected 1 fetch, got %d", fetchCount)
	}

	content, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("failed to read cached chunk file: %v", err)
	}
	if string(content) != string(mockChunkData) {
		t.Fatalf("expected chunk content %q, got %q", mockChunkData, content)
	}

	// 2. Second fetch: cache hit -> must not call mock server again
	path2, err := dc.GetOrFetchChunk(ctx, chunkKey, int64(len(mockChunkData)), server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error on second fetch: %v", err)
	}
	if path1 != path2 {
		t.Fatalf("expected path %q, got %q", path1, path2)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Fatalf("expected still 1 fetch (cache hit), got %d", fetchCount)
	}
}

func TestDiskCacheSingleflightConcurrentFetches(t *testing.T) {
	tempDir := t.TempDir()
	logger := log.New(io.Discard, "", 0)
	dc, err := NewDiskCache(tempDir, 10*1024*1024, logger)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	mockData := make([]byte, 1024)
	for i := range mockData {
		mockData[i] = byte(i % 256)
	}

	var fetchCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		time.Sleep(50 * time.Millisecond) // artificially delay upstream response
		_, _ = w.Write(mockData)
	}))
	defer server.Close()

	ctx := context.Background()
	chunkKey := "concurrent_chunk.bin"

	var wg sync.WaitGroup
	const concurrency = 10
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := dc.GetOrFetchChunk(ctx, chunkKey, int64(len(mockData)), server.URL, nil)
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrency client %d failed: %v", i, err)
		}
	}

	// With singleflight, only 1 HTTP request should have been made to upstream
	if count := atomic.LoadInt32(&fetchCount); count != 1 {
		t.Fatalf("expected exactly 1 fetch due to singleflight coalescing, got %d", count)
	}
}

func TestDiskCacheLRUEviction(t *testing.T) {
	tempDir := t.TempDir()
	logger := log.New(io.Discard, "", 0)

	// Set max cache size to 1000 bytes
	const maxBytes int64 = 1000
	dc, err := NewDiskCache(tempDir, maxBytes, logger)
	if err != nil {
		t.Fatalf("failed to create disk cache: %v", err)
	}

	// Write 3 files of 400 bytes each in chunks/ -> total 1200 bytes > 1000 bytes max
	chunksDir := filepath.Join(tempDir, "chunks")
	f1 := filepath.Join(chunksDir, "chunk_1.bin")
	f2 := filepath.Join(chunksDir, "chunk_2.bin")
	f3 := filepath.Join(chunksDir, "chunk_3.bin")

	buf := make([]byte, 400)
	_ = os.WriteFile(f1, buf, 0644)
	_ = os.WriteFile(f2, buf, 0644)
	_ = os.WriteFile(f3, buf, 0644)

	// Set distinct mod times: f1 oldest, f2 middle, f3 newest
	now := time.Now()
	_ = os.Chtimes(f1, now.Add(-30*time.Minute), now.Add(-30*time.Minute))
	_ = os.Chtimes(f2, now.Add(-15*time.Minute), now.Add(-15*time.Minute))
	_ = os.Chtimes(f3, now, now)

	// Run eviction
	dc.evict()

	// Target size is 80% of 1000 = 800 bytes.
	// With 3 files of 400 bytes (1200 bytes), deleting f1 leaves 800 bytes (<= targetSize).
	// So f1 must be deleted, f2 and f3 must remain.
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Errorf("expected oldest file f1 to be evicted, but it still exists")
	}
	if _, err := os.Stat(f2); os.IsNotExist(err) {
		t.Errorf("expected f2 to remain in cache, but it was deleted")
	}
	if _, err := os.Stat(f3); os.IsNotExist(err) {
		t.Errorf("expected f3 to remain in cache, but it was deleted")
	}
}
