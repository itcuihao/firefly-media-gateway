package media

import "context"

type Repository interface {
	Create(ctx context.Context, input CreateAssetInput) (Asset, error)
	GetByID(ctx context.Context, id string) (Asset, error)
	MarkDeleted(ctx context.Context, id string) (Asset, error)
}
