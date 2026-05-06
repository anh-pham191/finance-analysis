//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestWithUserTxScopesQueriesToUser(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "withusertx-user1@example.test")
	userTwo := seedUser(t, db.owner, "user2@example.test")

	if err := withUserTx(context.Background(), db.app, userOne, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO accounts (user_id, id, name, bank, type)
			VALUES ($1, 'acc-1', 'Everyday', 'ANZ', 'checking')
		`, userOne.Int64())
		return err
	}); err != nil {
		t.Fatalf("insert account for user one: %v", err)
	}

	var count int
	if err := withUserTx(context.Background(), db.app, userTwo, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT count(*) FROM accounts`).Scan(&count)
	}); err != nil {
		t.Fatalf("count accounts for user two: %v", err)
	}

	if count != 0 {
		t.Fatalf("user two saw %d accounts, want 0", count)
	}
}

func TestAppConnectionWithoutUserTxCannotReadTenantRows(t *testing.T) {
	db := newTestDatabase(t)

	var count int
	err := db.app.QueryRowContext(context.Background(), `SELECT count(*) FROM categories`).Scan(&count)
	if err != nil {
		t.Fatalf("query without app.user_id returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("query without app.user_id returned %d rows, want 0", count)
	}
}

func seedUser(t *testing.T, db *sql.DB, email string) domain.UserID {
	t.Helper()

	uniqueEmail := fmt.Sprintf("%d-%s", time.Now().UnixNano(), email)
	id := time.Now().UnixNano()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin seed user tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, fmt.Sprint(id)); err != nil {
		t.Fatalf("set seed user id: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO users (id, email, display_name)
		OVERRIDING SYSTEM VALUE
		VALUES ($1, $2, $3)
	`, id, uniqueEmail, uniqueEmail); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed user: %v", err)
	}
	userID, err := domain.NewUserID(id)
	if err != nil {
		t.Fatalf("new user id: %v", err)
	}
	return userID
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error = %v, want sql.ErrNoRows or ports.ErrNotFound", err)
	}
}
