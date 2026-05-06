package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func withUserTx(ctx context.Context, db *sql.DB, userID domain.UserID, fn func(context.Context, *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		return fmt.Errorf("set app user id: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user tx: %w", err)
	}
	return nil
}
