package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type TxnRepo interface {
	Upsert(ctx context.Context, userID domain.UserID, txn domain.Transaction) error
	Get(ctx context.Context, userID domain.UserID, id string) (domain.Transaction, error)
}
