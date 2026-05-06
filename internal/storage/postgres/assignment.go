package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type AssignmentRepo struct {
	db *sql.DB
}

func NewAssignmentRepo(db *sql.DB) *AssignmentRepo {
	return &AssignmentRepo{db: db}
}

func (r *AssignmentRepo) Upsert(ctx context.Context, userID domain.UserID, assignment domain.CategoryAssignment) error {
	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO category_assignments (user_id, txn_id, category_id, source, rule_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, txn_id) DO UPDATE SET
				category_id = EXCLUDED.category_id,
				source = EXCLUDED.source,
				rule_id = EXCLUDED.rule_id,
				assigned_at = now()
		`, userID.Int64(), assignment.TxnID, assignment.CategoryID, string(assignment.Source), assignment.RuleID)
		if err != nil {
			return fmt.Errorf("upsert category assignment: %w", err)
		}
		return nil
	})
}

func (r *AssignmentRepo) Get(ctx context.Context, userID domain.UserID, txnID string) (domain.CategoryAssignment, error) {
	var assignment domain.CategoryAssignment
	var ruleID sql.NullInt64
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT txn_id, category_id, source, rule_id, assigned_at
			FROM category_assignments
			WHERE user_id = $1 AND txn_id = $2
		`, userID.Int64(), txnID).Scan(
			&assignment.TxnID,
			&assignment.CategoryID,
			&assignment.Source,
			&ruleID,
			&assignment.AssignedAt,
		)
	})
	if err != nil {
		return domain.CategoryAssignment{}, err
	}
	if ruleID.Valid {
		assignment.RuleID = &ruleID.Int64
	}
	return assignment, nil
}
