package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	project, usage, status, is_chunked
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
`

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
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
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
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
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
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
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

func (r *PostgresRepository) SaveChunks(ctx context.Context, assetID string, chunks []media.Chunk) error {
	const q = `INSERT INTO media_chunks (asset_id, chunk_index, chunk_file_id) VALUES ($1, $2, $3)`
	for _, c := range chunks {
		if _, err := r.db.ExecContext(ctx, q, assetID, c.ChunkIndex, c.ChunkFileID); err != nil {
			return fmt.Errorf("save chunk %d: %w", c.ChunkIndex, err)
		}
	}
	return nil
}

func (r *PostgresRepository) GetChunks(ctx context.Context, assetID string) ([]media.Chunk, error) {
	const q = `SELECT asset_id, chunk_index, chunk_file_id FROM media_chunks WHERE asset_id = $1 ORDER BY chunk_index`
	rows, err := r.db.QueryContext(ctx, q, assetID)
	if err != nil {
		return nil, fmt.Errorf("get chunks: %w", err)
	}
	defer rows.Close()

	var chunks []media.Chunk
	for rows.Next() {
		var c media.Chunk
		if err := rows.Scan(&c.AssetID, &c.ChunkIndex, &c.ChunkFileID); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		if len(c.AssetID) == 36 {
			c.AssetID = strings.ReplaceAll(c.AssetID, "-", "")
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (r *PostgresRepository) DeleteChunks(ctx context.Context, assetID string) error {
	const q = `DELETE FROM media_chunks WHERE asset_id = $1`
	_, err := r.db.ExecContext(ctx, q, assetID)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAsset(s scanner) (media.Asset, error) {
	var asset media.Asset
	var providerBucket sql.NullString
	var sha256 sql.NullString
	var deletedAt sql.NullTime

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
	)
	if err != nil {
		return media.Asset{}, err
	}

	if len(asset.ID) == 36 {
		asset.ID = strings.ReplaceAll(asset.ID, "-", "")
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
