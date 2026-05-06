package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type CategoryRepo interface {
	Insert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error)
	GetByName(ctx context.Context, userID domain.UserID, name string) (domain.Category, error)
	Upsert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error)
	List(ctx context.Context, userID domain.UserID) ([]domain.Category, error)
}
