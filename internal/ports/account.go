package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type AccountRepo interface {
	Upsert(ctx context.Context, userID domain.UserID, account domain.Account) error
	Get(ctx context.Context, userID domain.UserID, id string) (domain.Account, error)
}
