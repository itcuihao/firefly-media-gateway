package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// DiskCache manages local disk block/chunk and file caching with LRU eviction.
type DiskCache struct {
	dir      string
	maxBytes int64
	logger   *log.Logger
	sf       singleflight.Group
	evicting atomic.Bool
}

// NewDiskCache creates a new DiskCache instance.
// If dir is empty, it defaults to "data/cache".
// If maxBytes <= 0, it defaults to 20GB.
func NewDiskCache(dir string, maxBytes int64, logger *log.Logger) (*DiskCache, error) {
	if dir == "" {
		dir = "data/cache"
	}
	if maxBytes <= 0 {
		maxBytes = 20 * 1024 * 1024 * 1024 // 20GB default
	}

	for _, sub := range []string{"", "chunks", "files", "webp"} {
		p := filepath.Join(dir, sub)
		if err := os.MkdirAll(p, 0755); err != nil {
			return nil, fmt.Errorf("create cache directory %q: %w", p, err)
		}
	}

	return &DiskCache{
		dir:      dir,
		maxBytes: maxBytes,
		logger:   logger,
	}, nil
}

// Dir returns the root cache directory.
func (d *DiskCache) Dir() string {
	return d.dir
}

// MaxBytes returns the maximum configured cache size in bytes.
func (d *DiskCache) MaxBytes() int64 {
	return d.maxBytes
}

// GetOrFetchChunk returns the absolute path of a cached chunk file.
// If the chunk is not cached, it fetches the full chunk from chunkURL using singleflight,
// saves it atomically, and returns the path.
func (d *DiskCache) GetOrFetchChunk(ctx context.Context, chunkKey string, expectedSize int64, chunkURL string, headers map[string]string) (string, error) {
	chunkDir := filepath.Join(d.dir, "chunks")
	targetPath := filepath.Join(chunkDir, chunkKey)

	// 1. Fast path: check if cached file already exists and is intact
	if fi, err := os.Stat(targetPath); err == nil {
		if expectedSize <= 0 || fi.Size() == expectedSize {
			_ = os.Chtimes(targetPath, time.Now(), time.Now())
			return targetPath, nil
		}
		// Size mismatch: incomplete or corrupted file, discard
		_ = os.Remove(targetPath)
	}

	// 2. Coalesce concurrent requests for the same chunk
	res, err, _ := d.sf.Do("chunk:"+chunkKey, func() (any, error) {
		// Double check inside singleflight lock
		if fi, err := os.Stat(targetPath); err == nil {
			if expectedSize <= 0 || fi.Size() == expectedSize {
				_ = os.Chtimes(targetPath, time.Now(), time.Now())
				return targetPath, nil
			}
			_ = os.Remove(targetPath)
		}

		tmpFile, err := os.CreateTemp(chunkDir, "chunk-tmp-*")
		if err != nil {
			return "", fmt.Errorf("create temp chunk file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer func() {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
		}()

		// Download using detached context with timeout to ensure complete chunk download
		reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, chunkURL, nil)
		if err != nil {
			return "", fmt.Errorf("build chunk download request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// Use http.DefaultTransport so test mocks or custom transports are honored
		client := &http.Client{Transport: http.DefaultTransport, Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("download chunk from upstream: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
		}

		written, err := io.Copy(tmpFile, resp.Body)
		if err != nil {
			return "", fmt.Errorf("write chunk temp file: %w", err)
		}
		if expectedSize > 0 && written != expectedSize {
			return "", fmt.Errorf("chunk size mismatch: wrote %d bytes, expected %d", written, expectedSize)
		}

		_ = tmpFile.Close()
		if err := os.Rename(tmpPath, targetPath); err != nil {
			return "", fmt.Errorf("save chunk file: %w", err)
		}

		_ = os.Chtimes(targetPath, time.Now(), time.Now())
		d.TriggerEviction()
		return targetPath, nil
	})

	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// GetOrFetchFile returns the absolute path of a cached single media file.
// If the file is not cached, it fetches the full file from fileURL using singleflight,
// saves it atomically, and returns the path.
func (d *DiskCache) GetOrFetchFile(ctx context.Context, fileKey string, expectedSize int64, fileURL string, headers map[string]string) (string, error) {
	fileDir := filepath.Join(d.dir, "files")
	targetPath := filepath.Join(fileDir, fileKey)

	// 1. Fast path: check if cached file already exists and is intact
	if fi, err := os.Stat(targetPath); err == nil {
		if expectedSize <= 0 || fi.Size() == expectedSize {
			_ = os.Chtimes(targetPath, time.Now(), time.Now())
			return targetPath, nil
		}
		_ = os.Remove(targetPath)
	}

	// 2. Coalesce concurrent downloads
	res, err, _ := d.sf.Do("file:"+fileKey, func() (any, error) {
		if fi, err := os.Stat(targetPath); err == nil {
			if expectedSize <= 0 || fi.Size() == expectedSize {
				_ = os.Chtimes(targetPath, time.Now(), time.Now())
				return targetPath, nil
			}
			_ = os.Remove(targetPath)
		}

		tmpFile, err := os.CreateTemp(fileDir, "file-tmp-*")
		if err != nil {
			return "", fmt.Errorf("create temp media file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer func() {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
		}()

		reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, fileURL, nil)
		if err != nil {
			return "", fmt.Errorf("build file download request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{Transport: http.DefaultTransport, Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("download file from upstream: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
		}

		written, err := io.Copy(tmpFile, resp.Body)
		if err != nil {
			return "", fmt.Errorf("write media temp file: %w", err)
		}
		if expectedSize > 0 && written != expectedSize {
			return "", fmt.Errorf("media size mismatch: wrote %d bytes, expected %d", written, expectedSize)
		}

		_ = tmpFile.Close()
		if err := os.Rename(tmpPath, targetPath); err != nil {
			return "", fmt.Errorf("save media file: %w", err)
		}

		_ = os.Chtimes(targetPath, time.Now(), time.Now())
		d.TriggerEviction()
		return targetPath, nil
	})

	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// TriggerEviction triggers a background check to prune LRU files if cache exceeds maxBytes.
func (d *DiskCache) TriggerEviction() {
	if d.evicting.CompareAndSwap(false, true) {
		go func() {
			defer d.evicting.Store(false)
			d.evict()
		}()
	}
}

type cachedFile struct {
	path    string
	size    int64
	modTime time.Time
}

func (d *DiskCache) evict() {
	var files []cachedFile
	var totalSize int64

	err := filepath.Walk(d.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		// Do not include directories or active temp files
		if !info.IsDir() && !strings.Contains(info.Name(), "-tmp-") {
			files = append(files, cachedFile{
				path:    path,
				size:    info.Size(),
				modTime: info.ModTime(),
			})
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		if d.logger != nil {
			d.logger.Printf("[CACHE] walk cache dir error: %v", err)
		}
		return
	}

	if totalSize <= d.maxBytes {
		return
	}

	// Sort oldest first (LRU based on ModTime, updated via os.Chtimes on every access)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Evict down to 80% of maxBytes to prevent frequent eviction triggers
	targetSize := int64(float64(d.maxBytes) * 0.8)
	var removedCount int
	var freedBytes int64

	for _, f := range files {
		if totalSize <= targetSize {
			break
		}
		if err := os.Remove(f.path); err == nil {
			totalSize -= f.size
			freedBytes += f.size
			removedCount++
		}
	}

	if d.logger != nil && removedCount > 0 {
		d.logger.Printf("[CACHE] LRU eviction: pruned %d files, freed %d MB, remaining %d MB",
			removedCount, freedBytes/(1024*1024), totalSize/(1024*1024))
	}
}

// Clear removes all cached files in the cache directory (used primarily for testing).
func (d *DiskCache) Clear() error {
	for _, sub := range []string{"chunks", "files", "webp"} {
		p := filepath.Join(d.dir, sub)
		entries, err := os.ReadDir(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				_ = os.Remove(filepath.Join(p, e.Name()))
			}
		}
	}
	return nil
}
