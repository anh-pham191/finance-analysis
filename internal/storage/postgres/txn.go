package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
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

func (r *TxnRepo) ListFiltered(ctx context.Context, userID domain.UserID, filter ports.TxnFilter) ([]ports.TxnReportRow, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var reportRows []ports.TxnReportRow
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		orderBy, err := txnFilterOrderBy(filter.Sort)
		if err != nil {
			return err
		}

		joins := `
			LEFT JOIN category_assignments ca ON ca.user_id = t.user_id AND ca.txn_id = t.id
			LEFT JOIN categories c ON c.user_id = t.user_id AND c.id = ca.category_id
		`
		where := []string{"t.user_id = $1"}
		args := []any{userID.Int64()}
		nextArg := 2

		if !filter.Range.From.IsZero() {
			where = append(where, fmt.Sprintf("t.posted_at >= $%d", nextArg))
			args = append(args, filter.Range.From)
			nextArg++
		}
		if !filter.Range.To.IsZero() {
			where = append(where, fmt.Sprintf("t.posted_at < $%d", nextArg))
			args = append(args, filter.Range.To)
			nextArg++
		}
		if filter.CategoryID != nil {
			where = append(where, fmt.Sprintf("ca.category_id = $%d", nextArg))
			args = append(args, *filter.CategoryID)
			nextArg++
		}
		if filter.Merchant != "" {
			where = append(where, fmt.Sprintf("t.merchant = $%d", nextArg))
			args = append(args, filter.Merchant)
			nextArg++
		}
		if filter.AccountID != "" {
			where = append(where, fmt.Sprintf("t.account_id = $%d", nextArg))
			args = append(args, filter.AccountID)
			nextArg++
		}
		if filter.Direction != nil {
			where = append(where, fmt.Sprintf("t.direction = $%d", nextArg))
			args = append(args, string(*filter.Direction))
			nextArg++
		}
		if filter.Min != nil {
			where = append(where, fmt.Sprintf("t.amount >= $%d", nextArg))
			args = append(args, filter.Min.String())
			nextArg++
		}
		if filter.Max != nil {
			where = append(where, fmt.Sprintf("t.amount <= $%d", nextArg))
			args = append(args, filter.Max.String())
			nextArg++
		}

		limitArg := nextArg
		args = append(args, limit)
		nextArg++
		offsetArg := nextArg
		args = append(args, offset)

		query := fmt.Sprintf(`
			SELECT t.id, t.account_id, t.posted_at, t.amount::text, t.direction,
				t.description, t.merchant, t.akahu_category, t.raw_json, t.created_at, t.updated_at,
				COALESCE(c.name, '')
			FROM transactions t
			%s
			WHERE %s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, joins, strings.Join(where, " AND "), orderBy, limitArg, offsetArg)

		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer func() {
			_ = rows.Close()
		}()

		for rows.Next() {
			row, err := scanTxnReportRow(rows)
			if err != nil {
				return err
			}
			reportRows = append(reportRows, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list filtered transactions: %w", err)
	}
	return reportRows, nil
}

type transactionScanner interface {
	Scan(dest ...any) error
}

func scanTransaction(scanner transactionScanner) (domain.Transaction, error) {
	var txn domain.Transaction
	var amount string
	var direction string
	var raw []byte
	if err := scanner.Scan(
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
		return domain.Transaction{}, err
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

func scanTxnReportRow(scanner transactionScanner) (ports.TxnReportRow, error) {
	var row ports.TxnReportRow
	var amount string
	var direction string
	var raw []byte
	if err := scanner.Scan(
		&row.Transaction.ID,
		&row.Transaction.AccountID,
		&row.Transaction.PostedAt,
		&amount,
		&direction,
		&row.Transaction.Description,
		&row.Transaction.Merchant,
		&row.Transaction.AkahuCategory,
		&raw,
		&row.Transaction.CreatedAt,
		&row.Transaction.UpdatedAt,
		&row.Category,
	); err != nil {
		return ports.TxnReportRow{}, err
	}
	money, err := domain.NewMoneyFromString(amount)
	if err != nil {
		return ports.TxnReportRow{}, fmt.Errorf("parse transaction amount: %w", err)
	}
	parsedDirection, err := domain.ParseDirection(direction)
	if err != nil {
		return ports.TxnReportRow{}, err
	}
	row.Transaction.Amount = money
	row.Transaction.Direction = parsedDirection
	row.Transaction.RawJSON = json.RawMessage(raw)
	return row, nil
}

func txnFilterOrderBy(sort string) (string, error) {
	switch sort {
	case "", "date":
		return "t.posted_at ASC, t.id ASC", nil
	case "amount":
		return "t.amount DESC, t.id ASC", nil
	case "merchant":
		return "t.merchant ASC, t.id ASC", nil
	default:
		return "", fmt.Errorf("invalid transaction sort %q", sort)
	}
}
