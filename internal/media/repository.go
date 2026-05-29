package media

import "context"

type Repository interface {
	Create(ctx context.Context, input CreateAssetInput) (Asset, error)
	GetByID(ctx context.Context, id string) (Asset, error)
	MarkDeleted(ctx context.Context, id string) (Asset, error)
	List(ctx context.Context, limit, offset int) ([]Asset, error)

	SaveChunks(ctx context.Context, assetID string, chunks []Chunk) error
	GetChunks(ctx context.Context, assetID string) ([]Chunk, error)
	DeleteChunks(ctx context.Context, assetID string) error
}
