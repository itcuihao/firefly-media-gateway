package storage

import (
	"context"
	"database/sql"
	"testing"

	"firefly-media-gateway/internal/media"

	_ "modernc.org/sqlite"
)

func TestSQLiteRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := EnsureSQLiteSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	repo := NewSQLiteRepository(db)
	location := "chat-1"
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	created, err := repo.Create(ctx, media.CreateAssetInput{
		ID:                   "asset-1",
		Provider:             "tg",
		ProviderFileID:       "file-1",
		ProviderBucketOrChat: &location,
		PublicURL:            "http://example.test/api/v1/media/asset-1",
		MIMEType:             "image/jpeg",
		SizeBytes:            123,
		SHA256:               &sha,
		Project:              "project-a",
		Usage:                "cover",
		IsChunked:            true,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if created.ID != "asset-1" || created.Provider != "tg" || !created.IsChunked {
		t.Fatalf("unexpected created asset: %#v", created)
	}

	// Save chunks
	chunks := []media.Chunk{
		{AssetID: "asset-1", ChunkIndex: 0, ChunkFileID: "chunk-file-1"},
		{AssetID: "asset-1", ChunkIndex: 1, ChunkFileID: "chunk-file-2"},
	}
	if err := repo.SaveChunks(ctx, "asset-1", chunks); err != nil {
		t.Fatalf("save chunks: %v", err)
	}

	// Get chunks
	gotChunks, err := repo.GetChunks(ctx, "asset-1")
	if err != nil {
		t.Fatalf("get chunks: %v", err)
	}
	if len(gotChunks) != 2 || gotChunks[0].ChunkFileID != "chunk-file-1" || gotChunks[1].ChunkFileID != "chunk-file-2" {
		t.Fatalf("unexpected chunks: %#v", gotChunks)
	}

	got, err := repo.GetByID(ctx, "asset-1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if got.ProviderBucketOrChat == nil || *got.ProviderBucketOrChat != location {
		t.Fatalf("unexpected provider location: %#v", got.ProviderBucketOrChat)
	}

	list, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 asset, got %d", len(list))
	}

	deleted, err := repo.MarkDeleted(ctx, "asset-1")
	if err != nil {
		t.Fatalf("mark deleted: %v", err)
	}
	if deleted.Status != media.StatusDeleted || deleted.DeletedAt == nil {
		t.Fatalf("asset was not marked deleted: %#v", deleted)
	}

	// Delete chunks
	if err := repo.DeleteChunks(ctx, "asset-1"); err != nil {
		t.Fatalf("delete chunks: %v", err)
	}
	gotChunks, err = repo.GetChunks(ctx, "asset-1")
	if err != nil {
		t.Fatalf("get chunks after delete: %v", err)
	}
	if len(gotChunks) != 0 {
		t.Fatalf("want 0 chunks after delete, got %d", len(gotChunks))
	}
}
