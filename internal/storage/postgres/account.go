package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type AccountRepo struct {
	db *sql.DB
}

func NewAccountRepo(db *sql.DB) *AccountRepo {
	return &AccountRepo{db: db}
}

func (r *AccountRepo) Upsert(ctx context.Context, userID domain.UserID, account domain.Account) error {
	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO accounts (user_id, id, name, bank, type, currency)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, id) DO UPDATE SET
				name = EXCLUDED.name,
				bank = EXCLUDED.bank,
				type = EXCLUDED.type,
				currency = EXCLUDED.currency
		`, userID.Int64(), account.ID, account.Name, account.Bank, account.Type, defaultCurrency(account.Currency))
		if err != nil {
			return fmt.Errorf("upsert account: %w", err)
		}
		return nil
	})
}

func (r *AccountRepo) Get(ctx context.Context, userID domain.UserID, id string) (domain.Account, error) {
	var account domain.Account
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, name, bank, type, currency, created_at
			FROM accounts
			WHERE user_id = $1 AND id = $2
		`, userID.Int64(), id).Scan(
			&account.ID,
			&account.Name,
			&account.Bank,
			&account.Type,
			&account.Currency,
			&account.CreatedAt,
		)
	})
	if err != nil {
		return domain.Account{}, err
	}
	return account, nil
}

func defaultCurrency(currency string) string {
	if currency == "" {
		return "NZD"
	}
	return currency
}
