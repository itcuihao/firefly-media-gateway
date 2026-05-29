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
		ChunkIDs:             []string{"chunk-1", "chunk-2"},
		TotalBytes:           246,
	})
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if created.ID != "asset-1" || created.Provider != "tg" {
		t.Fatalf("unexpected created asset: %#v", created)
	}
	if len(created.ChunkIDs) != 2 || created.ChunkIDs[1] != "chunk-2" {
		t.Fatalf("unexpected chunk ids: %#v", created.ChunkIDs)
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
}
