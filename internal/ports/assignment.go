package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type AssignmentRepo interface {
	Upsert(ctx context.Context, userID domain.UserID, assignment domain.CategoryAssignment) error
	Get(ctx context.Context, userID domain.UserID, txnID string) (domain.CategoryAssignment, error)
}
