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
	project, usage, status
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at
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
	project, usage, status, created_at, updated_at, deleted_at
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
	project, usage, status, created_at, updated_at, deleted_at
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
