package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type TxnFilter struct {
	Range      domain.Range
	CategoryID *int64
	Merchant   string
	AccountID  string
	Direction  *domain.Direction
	Min        *domain.Money
	Max        *domain.Money
	Sort       string
	Limit      int
	Offset     int
}

type TxnQueryRepo interface {
	ListFiltered(ctx context.Context, userID domain.UserID, filter TxnFilter) ([]domain.Transaction, error)
}
