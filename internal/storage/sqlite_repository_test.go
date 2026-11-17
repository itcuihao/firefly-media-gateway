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

	list, err := repo.List(ctx, media.ListFilter{Limit: 10, Offset: 0})
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

func TestSQLiteRepositoryHideDummy(t *testing.T) {
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
	sha1 := "dummy-sha-67"
	sha2 := "real-sha-2k"

	// Insert a 67-byte dummy image and a 2048-byte real image
	_, err = repo.Create(ctx, media.CreateAssetInput{
		ID:        "dummy-67b",
		Provider:  "tg",
		PublicURL: "https://example.com/dummy.png",
		MIMEType:  "image/png",
		SizeBytes: 67,
		SHA256:    &sha1,
	})
	if err != nil {
		t.Fatalf("create dummy: %v", err)
	}

	_, err = repo.Create(ctx, media.CreateAssetInput{
		ID:        "real-2kb",
		Provider:  "tg",
		PublicURL: "https://example.com/real.png",
		MIMEType:  "image/png",
		SizeBytes: 2048,
		SHA256:    &sha2,
	})
	if err != nil {
		t.Fatalf("create real: %v", err)
	}

	// List with HideDummy = true
	list, err := repo.List(ctx, media.ListFilter{Limit: 10, HideDummy: true})
	if err != nil {
		t.Fatalf("list with HideDummy: %v", err)
	}
	if len(list) != 1 || list[0].ID != "real-2kb" {
		t.Fatalf("expected only real-2kb, got %v", list)
	}

	count, err := repo.Count(ctx, media.ListFilter{HideDummy: true})
	if err != nil {
		t.Fatalf("count with HideDummy: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 with HideDummy, got %d", count)
	}

	// List with HideDummy = false
	listAll, err := repo.List(ctx, media.ListFilter{Limit: 10, HideDummy: false})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 items with HideDummy=false, got %d", len(listAll))
	}
}

func TestSQLiteRepositoryGetStatsAndAllowedProjects(t *testing.T) {
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

	// Insert 1 image in default, 1 video in test-project, 1 file in secret-project
	_, _ = repo.Create(ctx, media.CreateAssetInput{
		ID:        "img-1",
		Provider:  "tg",
		PublicURL: "/media/img-1.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 1000,
		Project:   "default",
		Usage:     "cover",
	})
	_, _ = repo.Create(ctx, media.CreateAssetInput{
		ID:        "vid-1",
		Provider:  "tg",
		PublicURL: "/media/vid-1.mp4",
		MIMEType:  "video/mp4",
		SizeBytes: 5000,
		Project:   "test-project",
		Usage:     "scene",
	})
	_, _ = repo.Create(ctx, media.CreateAssetInput{
		ID:        "doc-1",
		Provider:  "tg",
		PublicURL: "/media/doc-1.pdf",
		MIMEType:  "application/pdf",
		SizeBytes: 2000,
		Project:   "secret-project",
		Usage:     "file",
	})

	// 1. Full Stats without allowedProjects filter
	allStats, err := repo.GetStats(ctx, media.ListFilter{Status: "active"})
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if allStats.TotalCount != 3 || allStats.TotalBytes != 8000 {
		t.Errorf("expected 3 items and 8000 bytes, got %d items and %d bytes", allStats.TotalCount, allStats.TotalBytes)
	}
	if allStats.Images.Count != 1 || allStats.Images.Bytes != 1000 {
		t.Errorf("expected 1 image (1000B), got %d (%dB)", allStats.Images.Count, allStats.Images.Bytes)
	}
	if allStats.Videos.Count != 1 || allStats.Videos.Bytes != 5000 {
		t.Errorf("expected 1 video (5000B), got %d (%dB)", allStats.Videos.Count, allStats.Videos.Bytes)
	}
	if allStats.Others.Count != 1 || allStats.Others.Bytes != 2000 {
		t.Errorf("expected 1 other (2000B), got %d (%dB)", allStats.Others.Count, allStats.Others.Bytes)
	}

	// 2. Stats with guest allowedProjects filter: ["default", "test-project"]
	guestStats, err := repo.GetStats(ctx, media.ListFilter{
		Status:          "active",
		AllowedProjects: []string{"default", "test-project"},
	})
	if err != nil {
		t.Fatalf("GetStats with AllowedProjects failed: %v", err)
	}
	if guestStats.TotalCount != 2 || guestStats.TotalBytes != 6000 {
		t.Errorf("expected 2 guest items and 6000 bytes, got %d items and %d bytes", guestStats.TotalCount, guestStats.TotalBytes)
	}
	if guestStats.Others.Count != 0 {
		t.Errorf("expected 0 others for guest, got %d", guestStats.Others.Count)
	}

	// 3. GetDistinctProjects with allowedProjects
	projs, err := repo.GetDistinctProjects(ctx, true, nil, []string{"default", "test-project"})
	if err != nil {
		t.Fatalf("GetDistinctProjects failed: %v", err)
	}
	if len(projs) != 2 || projs[0] != "default" || projs[1] != "test-project" {
		t.Errorf("expected [default, test-project], got %v", projs)
	}
}
