package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type AssignmentRepo interface {
	Upsert(ctx context.Context, userID domain.UserID, assignment domain.CategoryAssignment) error
	Get(ctx context.Context, userID domain.UserID, txnID string) (domain.CategoryAssignment, error)
	UpsertIfChanged(ctx context.Context, userID domain.UserID, assignment domain.CategoryAssignment) (bool, error)
	Delete(ctx context.Context, userID domain.UserID, txnID string) error
	ListByCategory(ctx context.Context, userID domain.UserID, categoryID int64) ([]domain.CategoryAssignment, error)
}
