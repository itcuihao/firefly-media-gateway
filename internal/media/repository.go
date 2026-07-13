package media

import "context"

type ListFilter struct {
	Limit        int
	Offset       int
	Project      string
	Usage        string
	Search       string
	MediaType    string // 'image' | 'video' | 'audio'
	Status       string // 'active' | 'deleted' | ""
	OnlyPublic   bool
	PrivateRules []string
}

type Repository interface {
	Create(ctx context.Context, input CreateAssetInput) (Asset, error)
	GetByID(ctx context.Context, id string) (Asset, error)
	GetActiveBySHA256(ctx context.Context, sha256 string) (Asset, error)
	CountActiveBySHA256(ctx context.Context, sha256 string) (int, error)
	MarkDeleted(ctx context.Context, id string) (Asset, error)
	List(ctx context.Context, filter ListFilter) ([]Asset, error)
	Count(ctx context.Context, filter ListFilter) (int, error)
	GetDistinctProjects(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error)
	GetDistinctUsages(ctx context.Context, onlyPublic bool, privateRules []string) ([]string, error)

	SaveChunks(ctx context.Context, assetID string, chunks []Chunk) error
	GetChunks(ctx context.Context, assetID string) ([]Chunk, error)
	DeleteChunks(ctx context.Context, assetID string) error
}
