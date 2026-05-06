package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type RuleRepo interface {
	Insert(ctx context.Context, userID domain.UserID, rule domain.Rule) (domain.Rule, error)
	GetByName(ctx context.Context, userID domain.UserID, name string) (domain.Rule, error)
}
