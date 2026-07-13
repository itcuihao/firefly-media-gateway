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
    is_chunked INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS media_chunks (
    asset_id TEXT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    chunk_file_id TEXT NOT NULL,
    PRIMARY KEY (asset_id, chunk_index)
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
	project, usage, status, is_chunked
)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
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
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
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
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
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

func (r *SQLiteRepository) List(ctx context.Context, filter media.ListFilter) ([]media.Asset, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
FROM media_assets
WHERE 1=1
`)

	if filter.Project != "" {
		sb.WriteString(" AND project = ?")
		args = append(args, filter.Project)
	}
	if filter.Usage != "" {
		sb.WriteString(" AND usage = ?")
		args = append(args, filter.Usage)
	}
	if filter.Status != "" {
		sb.WriteString(" AND status = ?")
		args = append(args, filter.Status)
	}
	if filter.Search != "" {
		sb.WriteString(" AND (id LIKE ? OR project LIKE ? OR usage LIKE ?)")
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.MediaType != "" {
		sb.WriteString(" AND mime_type LIKE ?")
		args = append(args, filter.MediaType+"/%")
	}

	if filter.OnlyPublic && len(filter.PrivateRules) > 0 {
		hasAll := false
		var specificRules []string
		for _, rule := range filter.PrivateRules {
			if rule == "*" || rule == "all" {
				hasAll = true
				break
			}
			specificRules = append(specificRules, rule)
		}
		if hasAll {
			sb.WriteString(" AND 1=0") // No public access allowed
		} else if len(specificRules) > 0 {
			var placeholders []string
			for _, rule := range specificRules {
				placeholders = append(placeholders, "?")
				args = append(args, rule)
			}
			inList := strings.Join(placeholders, ", ")
			sb.WriteString(fmt.Sprintf(" AND project NOT IN (%s) AND usage NOT IN (%s)", inList, inList))
		}
	}

	sb.WriteString(" ORDER BY created_at DESC")

	if filter.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		sb.WriteString(" OFFSET ?")
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
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

func (r *SQLiteRepository) Count(ctx context.Context, filter media.ListFilter) (int, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT COUNT(*) FROM media_assets WHERE 1=1`)

	if filter.Project != "" {
		sb.WriteString(" AND project = ?")
		args = append(args, filter.Project)
	}
	if filter.Usage != "" {
		sb.WriteString(" AND usage = ?")
		args = append(args, filter.Usage)
	}
	if filter.Status != "" {
		sb.WriteString(" AND status = ?")
		args = append(args, filter.Status)
	}
	if filter.Search != "" {
		sb.WriteString(" AND (id LIKE ? OR project LIKE ? OR usage LIKE ?)")
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.MediaType != "" {
		sb.WriteString(" AND mime_type LIKE ?")
		args = append(args, filter.MediaType+"/%")
	}

	if filter.OnlyPublic && len(filter.PrivateRules) > 0 {
		hasAll := false
		var specificRules []string
		for _, rule := range filter.PrivateRules {
			if rule == "*" || rule == "all" {
				hasAll = true
				break
			}
			specificRules = append(specificRules, rule)
		}
		if hasAll {
			return 0, nil
		} else if len(specificRules) > 0 {
			var placeholders []string
			for _, rule := range specificRules {
				placeholders = append(placeholders, "?")
				args = append(args, rule)
			}
			inList := strings.Join(placeholders, ", ")
			sb.WriteString(fmt.Sprintf(" AND project NOT IN (%s) AND usage NOT IN (%s)", inList, inList))
		}
	}

	var count int
	err := r.db.QueryRowContext(ctx, sb.String(), args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count media assets: %w", err)
	}
	return count, nil
}

func (r *SQLiteRepository) GetDistinctProjects(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString("SELECT DISTINCT project FROM media_assets WHERE project <> ''")

	if onlyPublic && len(privateRules) > 0 {
		hasAll := false
		var specificRules []string
		for _, rule := range privateRules {
			if rule == "*" || rule == "all" {
				hasAll = true
				break
			}
			specificRules = append(specificRules, rule)
		}
		if hasAll {
			return []string{}, nil
		} else if len(specificRules) > 0 {
			var placeholders []string
			for _, rule := range specificRules {
				placeholders = append(placeholders, "?")
				args = append(args, rule)
			}
			inList := strings.Join(placeholders, ", ")
			sb.WriteString(fmt.Sprintf(" AND project NOT IN (%s)", inList))
		}
	}

	sb.WriteString(" ORDER BY project ASC")

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("get distinct projects: %w", err)
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (r *SQLiteRepository) GetDistinctUsages(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error) {
	var sb strings.Builder
	var args []any
	sb.WriteString("SELECT DISTINCT usage FROM media_assets WHERE usage <> ''")

	if onlyPublic && len(privateRules) > 0 {
		hasAll := false
		var specificRules []string
		for _, rule := range privateRules {
			if rule == "*" || rule == "all" {
				hasAll = true
				break
			}
			specificRules = append(specificRules, rule)
		}
		if hasAll {
			return []string{}, nil
		} else if len(specificRules) > 0 {
			var placeholders []string
			for _, rule := range specificRules {
				placeholders = append(placeholders, "?")
				args = append(args, rule)
			}
			inList := strings.Join(placeholders, ", ")
			sb.WriteString(fmt.Sprintf(" AND usage NOT IN (%s)", inList))
		}
	}

	sb.WriteString(" ORDER BY usage ASC")

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("get distinct usages: %w", err)
	}
	defer rows.Close()

	var usages []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

func (r *SQLiteRepository) SaveChunks(ctx context.Context, assetID string, chunks []media.Chunk) error {
	const q = `INSERT INTO media_chunks (asset_id, chunk_index, chunk_file_id) VALUES (?, ?, ?)`
	for _, c := range chunks {
		if _, err := r.db.ExecContext(ctx, q, assetID, c.ChunkIndex, c.ChunkFileID); err != nil {
			return fmt.Errorf("save chunk %d: %w", c.ChunkIndex, err)
		}
	}
	return nil
}

func (r *SQLiteRepository) GetChunks(ctx context.Context, assetID string) ([]media.Chunk, error) {
	const q = `SELECT asset_id, chunk_index, chunk_file_id FROM media_chunks WHERE asset_id = ? ORDER BY chunk_index`
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

func (r *SQLiteRepository) DeleteChunks(ctx context.Context, assetID string) error {
	const q = `DELETE FROM media_chunks WHERE asset_id = ?`
	_, err := r.db.ExecContext(ctx, q, assetID)
	return err
}

func (r *SQLiteRepository) GetActiveBySHA256(ctx context.Context, sha256 string) (media.Asset, error) {
	const q = `
SELECT
	id, provider, provider_file_id, provider_bucket_or_chat,
	public_url, mime_type, size_bytes, sha256,
	project, usage, status, created_at, updated_at, deleted_at, is_chunked
FROM media_assets
WHERE sha256 = ? AND status = 'active'
ORDER BY created_at DESC
LIMIT 1
`
	row := r.db.QueryRowContext(ctx, q, sha256)
	asset, err := scanSQLiteAsset(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, fmt.Errorf("query media asset by sha256: %w", err)
	}
	return asset, nil
}

func (r *SQLiteRepository) CountActiveBySHA256(ctx context.Context, sha256 string) (int, error) {
	const q = `SELECT COUNT(*) FROM media_assets WHERE sha256 = ? AND status = 'active'`
	var count int
	err := r.db.QueryRowContext(ctx, q, sha256).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active assets by sha256: %w", err)
	}
	return count, nil
}

func scanSQLiteAsset(s scanner) (media.Asset, error) {
	var asset media.Asset
	var providerBucket sql.NullString
	var sha256 sql.NullString
	var deletedAt any
	var createdAt any
	var updatedAt any

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
