package akahu

import (
	"context"
	"errors"
	"os"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type EnvTokenStore struct{}

func (EnvTokenStore) AkahuTokens(ctx context.Context, userID domain.UserID) (string, string, error) {
	// M2 intentionally returns the same local tokens for every user. M8a
	// replaces this with encrypted per-user token storage.
	app := os.Getenv("AKAHU_APP_TOKEN")
	user := os.Getenv("AKAHU_USER_TOKEN")
	if app == "" || user == "" {
		return "", "", errors.New("Akahu tokens are not configured")
	}
	return app, user, nil
}
