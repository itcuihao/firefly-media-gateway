package media

import (
	"context"
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
	asset  Asset
	chunks []Chunk
}

func (r *fakeRepository) Create(context.Context, CreateAssetInput) (Asset, error) {
	return Asset{}, nil
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
