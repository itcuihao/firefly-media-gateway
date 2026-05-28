-- Add chunked upload support for large files (> 50MB)
-- Chunks are uploaded as separate files to Telegram and linked via asset_id

-- Add chunk-related columns
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS is_chunked BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS chunk_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS chunk_ids TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS total_bytes BIGINT NOT NULL DEFAULT 0;

-- Create index for chunked assets
CREATE INDEX IF NOT EXISTS idx_media_assets_is_chunked ON media_assets(is_chunked);

COMMENT ON COLUMN media_assets.is_chunked IS 'Whether this asset is split into chunks';
COMMENT ON COLUMN media_assets.chunk_count IS 'Number of chunks (0 for single file)';
COMMENT ON COLUMN media_assets.chunk_ids IS 'Array of Telegram file_ids for each chunk';
COMMENT ON COLUMN media_assets.total_bytes IS 'Total size of original file before chunking';
