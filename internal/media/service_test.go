package media

import (
	"context"
	"fmt"
	"io"
	"testing"

	"firefly-media-gateway/internal/provider"
)

func TestNormalizeAndValidateMIME(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		declared string
		sniff    []byte
		wantMIME string
		wantKind string
		wantErr  bool
	}{
		{
			name:     "jpeg by sniff",
			fileName: "a.bin",
			sniff:    []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
			wantMIME: "image/jpeg",
			wantKind: "image",
		},
		{
			name:     "mov by extension fallback",
			fileName: "video.mov",
			sniff:    []byte("unknown"),
			wantMIME: "video/quicktime",
			wantKind: "video",
		},
		{
			name:     "invalid",
			fileName: "doc.pdf",
			sniff:    []byte("%PDF"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMIME, gotKind, err := normalizeAndValidateMIME(tt.fileName, tt.declared, tt.sniff)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMIME != tt.wantMIME {
				t.Fatalf("want mime=%q got=%q", tt.wantMIME, gotMIME)
			}
			if gotKind != tt.wantKind {
				t.Fatalf("want kind=%q got=%q", tt.wantKind, gotKind)
			}
		})
	}
}

func TestDeleteChunkedDeletesEveryChunk(t *testing.T) {
	location := "bucket-a"
	repo := &fakeRepository{
		asset: Asset{
			ID:                   "asset-1",
			Provider:             "fake",
			ProviderBucketOrChat: &location,
			Status:               StatusActive,
			IsChunked:            true,
		},
		chunks: []Chunk{
			{AssetID: "asset-1", ChunkIndex: 0, ChunkFileID: "chunk-1"},
			{AssetID: "asset-1", ChunkIndex: 1, ChunkFileID: "chunk-2"},
		},
	}
	p := &fakeProvider{name: "fake"}
	svc := NewService(repo, map[string]provider.StorageProvider{"fake": p}, "fake", "http://example.test")

	asset, err := svc.Delete(context.Background(), "asset-1", nil)
	if err != nil {
		t.Fatalf("delete chunked asset: %v", err)
	}
	if asset.Status != StatusDeleted {
		t.Fatalf("want deleted status, got %q", asset.Status)
	}
	if len(p.deletedIDs) != 2 {
		t.Fatalf("want 2 deleted chunks, got %d", len(p.deletedIDs))
	}
	if p.deletedIDs[0] != "chunk-1" || p.deletedIDs[1] != "chunk-2" {
		t.Fatalf("unexpected deleted chunks: %#v", p.deletedIDs)
	}
}

type fakeRepository struct {
	asset             Asset
	chunks            []Chunk
	activeBySHA256    map[string]Asset
	activeBySHA256Err error
	countBySHA256     map[string]int
}

func (r *fakeRepository) Create(_ context.Context, input CreateAssetInput) (Asset, error) {
	return Asset{
		ID:                   input.ID,
		Provider:             input.Provider,
		ProviderFileID:       input.ProviderFileID,
		ProviderBucketOrChat: input.ProviderBucketOrChat,
		PublicURL:            input.PublicURL,
		MIMEType:             input.MIMEType,
		SizeBytes:            input.SizeBytes,
		SHA256:               input.SHA256,
		Project:              input.Project,
		Usage:                input.Usage,
		Status:               StatusActive,
		IsChunked:            input.IsChunked,
	}, nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (Asset, error) {
	if r.asset.ID != id {
		return Asset{}, ErrNotFound
	}
	return r.asset, nil
}

func (r *fakeRepository) MarkDeleted(_ context.Context, id string) (Asset, error) {
	if r.asset.ID != id {
		return Asset{}, ErrNotFound
	}
	r.asset.Status = StatusDeleted
	return r.asset, nil
}

func (r *fakeRepository) List(context.Context, int, int) ([]Asset, error) {
	return nil, nil
}

func (r *fakeRepository) SaveChunks(_ context.Context, assetID string, chunks []Chunk) error {
	r.chunks = chunks
	return nil
}

func (r *fakeRepository) GetChunks(_ context.Context, assetID string) ([]Chunk, error) {
	return r.chunks, nil
}

func (r *fakeRepository) DeleteChunks(_ context.Context, assetID string) error {
	r.chunks = nil
	return nil
}

func (r *fakeRepository) GetActiveBySHA256(_ context.Context, sha string) (Asset, error) {
	if r.activeBySHA256Err != nil {
		return Asset{}, r.activeBySHA256Err
	}
	if a, ok := r.activeBySHA256[sha]; ok {
		return a, nil
	}
	return Asset{}, ErrNotFound
}

func (r *fakeRepository) CountActiveBySHA256(_ context.Context, sha string) (int, error) {
	if c, ok := r.countBySHA256[sha]; ok {
		return c, nil
	}
	return 0, nil
}

type fakeProvider struct {
	name       string
	deletedIDs []string
}

func (p *fakeProvider) Name() string {
	return p.name
}

func (p *fakeProvider) Upload(context.Context, provider.UploadInput) (provider.UploadResult, error) {
	return provider.UploadResult{}, nil
}

func (p *fakeProvider) Delete(_ context.Context, providerFileID string, _ *string) error {
	p.deletedIDs = append(p.deletedIDs, providerFileID)
	return nil
}

func (p *fakeProvider) GetAccess(context.Context, string, *string) (provider.AccessResult, error) {
	return provider.AccessResult{}, nil
}

func TestUploadDeduplication(t *testing.T) {
	location := "bucket-a"
	existingAsset := Asset{
		ID:                   "asset-old",
		Provider:             "fake",
		ProviderFileID:       "file-remote-123",
		ProviderBucketOrChat: &location,
		Status:               StatusActive,
		IsChunked:            false,
	}

	repo := &fakeRepository{
		activeBySHA256: map[string]Asset{
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855": existingAsset, // SHA256 of empty reader
		},
	}

	p := &errorProvider{name: "fake"}
	svc := NewService(repo, map[string]provider.StorageProvider{"fake": p}, "fake", "http://example.test")

	var r emptyReader
	result, err := svc.Upload(context.Background(), UploadRequest{
		Project:             "proj",
		Usage:               "cover",
		FileName:            "test.jpg",
		DeclaredContentType: "image/jpeg",
		Reader:              r,
	})
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	if result.ID == existingAsset.ID {
		t.Fatalf("expected new asset ID, got duplicate ID %s", result.ID)
	}
	if result.ProviderFileID != existingAsset.ProviderFileID {
		t.Fatalf("expected reused ProviderFileID %s, got %s", existingAsset.ProviderFileID, result.ProviderFileID)
	}
}

func TestDeleteDeduplication(t *testing.T) {
	location := "bucket-a"
	sha := "hash123"
	repo := &fakeRepository{
		asset: Asset{
			ID:                   "asset-1",
			Provider:             "fake",
			ProviderFileID:       "file-1",
			ProviderBucketOrChat: &location,
			Status:               StatusActive,
			SHA256:               &sha,
			IsChunked:            false,
		},
		countBySHA256: map[string]int{
			"hash123": 2, // 2 active records share this hash
		},
	}
	p := &fakeProvider{name: "fake"}
	svc := NewService(repo, map[string]provider.StorageProvider{"fake": p}, "fake", "http://example.test")

	asset, err := svc.Delete(context.Background(), "asset-1", nil)
	if err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if asset.Status != StatusDeleted {
		t.Fatalf("expected deleted status, got %q", asset.Status)
	}

	if len(p.deletedIDs) != 0 {
		t.Fatalf("expected no physical deletes, but got %d deletes: %#v", len(p.deletedIDs), p.deletedIDs)
	}
}

type emptyReader struct{}
func (emptyReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

type errorProvider struct {
	name string
}
func (p *errorProvider) Name() string { return p.name }
func (p *errorProvider) Upload(context.Context, provider.UploadInput) (provider.UploadResult, error) {
	return provider.UploadResult{}, fmt.Errorf("should not be called")
}
func (p *errorProvider) Delete(context.Context, string, *string) error { return nil }
func (p *errorProvider) GetAccess(context.Context, string, *string) (provider.AccessResult, error) {
	return provider.AccessResult{}, nil
}
