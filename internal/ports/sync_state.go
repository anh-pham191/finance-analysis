package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type SyncStateRepo interface {
	Upsert(ctx context.Context, userID domain.UserID, state domain.SyncState) error
	Get(ctx context.Context, userID domain.UserID, accountID string) (domain.SyncState, error)
}
