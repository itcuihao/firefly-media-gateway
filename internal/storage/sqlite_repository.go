package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"firefly-media-gateway/internal/media"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func EnsureSQLiteSchema(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS media_assets (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_file_id TEXT NOT NULL,
    provider_bucket_or_chat TEXT,
    public_url TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT,
    project TEXT NOT NULL,
    usage TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'deleted')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    is_chunked INTEGER NOT NULL DEFAULT 0,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    chunk_ids TEXT NOT NULL DEFAULT '[]',
    total_bytes INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_media_assets_status ON media_assets(status);
CREATE INDEX IF NOT EXISTS idx_media_assets_project ON media_assets(project);
CREATE INDEX IF NOT EXISTS idx_media_assets_created_at ON media_assets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_assets_is_chunked ON media_assets(is_chunked);
`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("ensure sqlite schema: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Create(ctx context.Context, input media.CreateAssetInput) (media.Asset, error) {
	const q = `
INSERT INTO media_assets (
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status,
	is_chunked, chunk_count, chunk_ids, total_bytes
)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
`

	chunkIDs, err := json.Marshal(input.ChunkIDs)
	if err != nil {
		return media.Asset{}, fmt.Errorf("marshal chunk ids: %w", err)
	}

	row := r.db.QueryRowContext(ctx, q,
		input.ID,
		input.Provider,
		input.ProviderFileID,
		nullableString(input.ProviderBucketOrChat),
		input.PublicURL,
		input.MIMEType,
		input.SizeBytes,
		nullableString(input.SHA256),
		input.Project,
		input.Usage,
		media.StatusActive,
		input.IsChunked,
		len(input.ChunkIDs),
		string(chunkIDs),
		input.TotalBytes,
	)

	asset, err := scanSQLiteAsset(row)
	if err != nil {
		return media.Asset{}, fmt.Errorf("insert media asset: %w", err)
	}
	return asset, nil
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id string) (media.Asset, error) {
	const q = `
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
FROM media_assets
WHERE id = ?
`

	row := r.db.QueryRowContext(ctx, q, id)
	asset, err := scanSQLiteAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, fmt.Errorf("query media asset: %w", err)
	}
	return asset, nil
}

func (r *SQLiteRepository) MarkDeleted(ctx context.Context, id string) (media.Asset, error) {
	const q = `
UPDATE media_assets
SET status = ?, deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
`

	row := r.db.QueryRowContext(ctx, q, media.StatusDeleted, id)
	asset, err := scanSQLiteAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, fmt.Errorf("mark media deleted: %w", err)
	}
	return asset, nil
}

func (r *SQLiteRepository) List(ctx context.Context, limit, offset int) ([]media.Asset, error) {
	const q = `
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
FROM media_assets
ORDER BY created_at DESC
LIMIT ? OFFSET ?
`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list media assets: %w", err)
	}
	defer rows.Close()

	var assets []media.Asset
	for rows.Next() {
		asset, err := scanSQLiteAsset(rows)
		if err != nil {
			return nil, fmt.Errorf("scan media asset: %w", err)
		}
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return assets, nil
}

func scanSQLiteAsset(s scanner) (media.Asset, error) {
	var asset media.Asset
	var providerBucket sql.NullString
	var sha256 sql.NullString
	var deletedAt any
	var createdAt any
	var updatedAt any
	var chunkIDsJSON string

	err := s.Scan(
		&asset.ID,
		&asset.Provider,
		&asset.ProviderFileID,
		&providerBucket,
		&asset.PublicURL,
		&asset.MIMEType,
		&asset.SizeBytes,
		&sha256,
		&asset.Project,
		&asset.Usage,
		&asset.Status,
		&createdAt,
		&updatedAt,
		&deletedAt,
		&asset.IsChunked,
		&asset.ChunkCount,
		&chunkIDsJSON,
		&asset.TotalBytes,
	)
	if err != nil {
		return media.Asset{}, err
	}

	if providerBucket.Valid {
		asset.ProviderBucketOrChat = &providerBucket.String
	}
	if sha256.Valid {
		asset.SHA256 = &sha256.String
	}
	if chunkIDsJSON != "" {
		if err := json.Unmarshal([]byte(chunkIDsJSON), &asset.ChunkIDs); err != nil {
			return media.Asset{}, fmt.Errorf("unmarshal chunk ids: %w", err)
		}
	}

	var errTime error
	asset.CreatedAt, errTime = parseSQLiteTime(createdAt)
	if errTime != nil {
		return media.Asset{}, fmt.Errorf("parse created_at: %w", errTime)
	}
	asset.UpdatedAt, errTime = parseSQLiteTime(updatedAt)
	if errTime != nil {
		return media.Asset{}, fmt.Errorf("parse updated_at: %w", errTime)
	}
	if deletedAt != nil {
		t, err := parseSQLiteTime(deletedAt)
		if err != nil {
			return media.Asset{}, fmt.Errorf("parse deleted_at: %w", err)
		}
		asset.DeletedAt = &t
	}

	return asset, nil
}

func parseSQLiteTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		return t.UTC(), nil
	case string:
		return parseSQLiteTimeString(t)
	case []byte:
		return parseSQLiteTimeString(string(t))
	default:
		return time.Time{}, fmt.Errorf("unsupported time value %T", v)
	}
}

func parseSQLiteTimeString(v string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		t, err := time.Parse(layout, v)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", v)
}
