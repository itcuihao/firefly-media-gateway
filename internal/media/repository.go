package media

import "context"

type ListFilter struct {
	Limit           int
	Offset          int
	Project         string
	Usage           string
	Search          string
	MediaType       string // 'image' | 'video' | 'audio'
	Status          string // 'active' | 'deleted' | ""
	OnlyPublic      bool
	PrivateRules    []string
	HideDummy       bool
	AllowedProjects []string
}

type MediaTypeStats struct {
	Count   int64   `json:"count"`
	Bytes   int64   `json:"bytes"`
	Percent float64 `json:"percent"`
}

type MediaStats struct {
	TotalCount int64          `json:"total_count"`
	TotalBytes int64          `json:"total_bytes"`
	Images     MediaTypeStats `json:"images"`
	Videos     MediaTypeStats `json:"videos"`
	Others     MediaTypeStats `json:"others"`
}

type Repository interface {
	Create(ctx context.Context, input CreateAssetInput) (Asset, error)
	GetByID(ctx context.Context, id string) (Asset, error)
	GetActiveBySHA256(ctx context.Context, sha256 string) (Asset, error)
	CountActiveBySHA256(ctx context.Context, sha256 string) (int, error)
	MarkDeleted(ctx context.Context, id string) (Asset, error)
	List(ctx context.Context, filter ListFilter) ([]Asset, error)
	Count(ctx context.Context, filter ListFilter) (int, error)
	GetStats(ctx context.Context, filter ListFilter) (MediaStats, error)
	GetDistinctProjects(ctx context.Context, onlyPublic bool, privateRules []string, allowedProjects []string) ([]string, error)
	GetDistinctUsages(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error)

	SaveChunks(ctx context.Context, assetID string, chunks []Chunk) error
	GetChunks(ctx context.Context, assetID string) ([]Chunk, error)
	DeleteChunks(ctx context.Context, assetID string) error
}
