package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type TxnRepo struct {
	db *sql.DB
}

func NewTxnRepo(db *sql.DB) *TxnRepo {
	return &TxnRepo{db: db}
}

func (r *TxnRepo) Upsert(ctx context.Context, userID domain.UserID, txn domain.Transaction) error {
	raw := txn.RawJSON
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}

	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO transactions (
				user_id, id, account_id, posted_at, amount, direction,
				description, merchant, akahu_category, raw_json
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (user_id, id) DO UPDATE SET
				raw_json = EXCLUDED.raw_json,
				updated_at = now()
		`,
			userID.Int64(),
			txn.ID,
			txn.AccountID,
			txn.PostedAt,
			txn.Amount.String(),
			string(txn.Direction),
			txn.Description,
			txn.Merchant,
			txn.AkahuCategory,
			[]byte(raw),
		)
		if err != nil {
			return fmt.Errorf("upsert transaction: %w", err)
		}
		return nil
	})
}

func (r *TxnRepo) Get(ctx context.Context, userID domain.UserID, id string) (domain.Transaction, error) {
	var txn domain.Transaction
	var amount string
	var direction string
	var raw []byte
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, account_id, posted_at, amount::text, direction,
				description, merchant, akahu_category, raw_json, created_at, updated_at
			FROM transactions
			WHERE user_id = $1 AND id = $2
		`, userID.Int64(), id).Scan(
			&txn.ID,
			&txn.AccountID,
			&txn.PostedAt,
			&amount,
			&direction,
			&txn.Description,
			&txn.Merchant,
			&txn.AkahuCategory,
			&raw,
			&txn.CreatedAt,
			&txn.UpdatedAt,
		)
	})
	if err != nil {
		return domain.Transaction{}, repoGetError(err)
	}

	money, err := domain.NewMoneyFromString(amount)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("parse transaction amount: %w", err)
	}
	parsedDirection, err := domain.ParseDirection(direction)
	if err != nil {
		return domain.Transaction{}, err
	}
	txn.Amount = money
	txn.Direction = parsedDirection
	txn.RawJSON = json.RawMessage(raw)
	return txn, nil
}

func (r *TxnRepo) List(ctx context.Context, userID domain.UserID) ([]domain.Transaction, error) {
	var txns []domain.Transaction
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, account_id, posted_at, amount::text, direction,
				description, merchant, akahu_category, raw_json, created_at, updated_at
			FROM transactions
			WHERE user_id = $1
			ORDER BY posted_at ASC, id ASC
		`, userID.Int64())
		if err != nil {
			return err
		}
		defer func() {
			_ = rows.Close()
		}()

		for rows.Next() {
			var txn domain.Transaction
			var amount string
			var direction string
			var raw []byte
			if err := rows.Scan(
				&txn.ID,
				&txn.AccountID,
				&txn.PostedAt,
				&amount,
				&direction,
				&txn.Description,
				&txn.Merchant,
				&txn.AkahuCategory,
				&raw,
				&txn.CreatedAt,
				&txn.UpdatedAt,
			); err != nil {
				return err
			}
			money, err := domain.NewMoneyFromString(amount)
			if err != nil {
				return fmt.Errorf("parse transaction amount: %w", err)
			}
			parsedDirection, err := domain.ParseDirection(direction)
			if err != nil {
				return err
			}
			txn.Amount = money
			txn.Direction = parsedDirection
			txn.RawJSON = json.RawMessage(raw)
			txns = append(txns, txn)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	return txns, nil
}
