package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"firefly-media-gateway/internal/media"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, input media.CreateAssetInput) (media.Asset, error) {
	const q = `
INSERT INTO media_assets (
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status,
	is_chunked, chunk_count, chunk_ids, total_bytes
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
`

	chunkCount := len(input.ChunkIDs)
	chunkIDs := input.ChunkIDs
	if chunkIDs == nil {
		chunkIDs = []string{}
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
		chunkCount,
		chunkIDs,
		input.TotalBytes,
	)

	asset, err := scanAsset(row)
	if err != nil {
		return media.Asset{}, fmt.Errorf("insert media asset: %w", err)
	}
	return asset, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (media.Asset, error) {
	const q = `
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
FROM media_assets
WHERE id = $1
`

	row := r.db.QueryRowContext(ctx, q, id)
	asset, err := scanAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, fmt.Errorf("query media asset: %w", err)
	}

	return asset, nil
}

func (r *PostgresRepository) MarkDeleted(ctx context.Context, id string) (media.Asset, error) {
	const q = `
UPDATE media_assets
SET status = $2, deleted_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
`

	row := r.db.QueryRowContext(ctx, q, id, media.StatusDeleted)
	asset, err := scanAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, fmt.Errorf("mark media deleted: %w", err)
	}
	return asset, nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]media.Asset, error) {
	const q = `
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at,
	is_chunked, chunk_count, chunk_ids, total_bytes
FROM media_assets
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list media assets: %w", err)
	}
	defer rows.Close()

	var assets []media.Asset
	for rows.Next() {
		asset, err := scanAsset(rows)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanAsset(s scanner) (media.Asset, error) {
	var asset media.Asset
	var providerBucket sql.NullString
	var sha256 sql.NullString
	var deletedAt sql.NullTime
	var chunkIDs []string

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
		&asset.CreatedAt,
		&asset.UpdatedAt,
		&deletedAt,
		&asset.IsChunked,
		&asset.ChunkCount,
		&chunkIDs,
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
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		asset.DeletedAt = &t
	}
	if chunkIDs != nil {
		asset.ChunkIDs = chunkIDs
	}
	asset.CreatedAt = asset.CreatedAt.UTC()
	asset.UpdatedAt = asset.UpdatedAt.UTC()

	return asset, nil
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := *v
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func Ping(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}
