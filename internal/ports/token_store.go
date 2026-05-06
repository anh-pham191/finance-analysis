package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type TokenStore interface {
	AkahuTokens(ctx context.Context, userID domain.UserID) (app string, user string, err error)
}
