package postgres

import (
	"context"
	"database/sql"
	"errors"
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
		return domain.CategoryAssignment{}, repoGetError(err)
	}
	if ruleID.Valid {
		assignment.RuleID = &ruleID.Int64
	}
	return assignment, nil
}

func (r *AssignmentRepo) UpsertIfChanged(ctx context.Context, userID domain.UserID, assignment domain.CategoryAssignment) (bool, error) {
	changed := true
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			INSERT INTO category_assignments (user_id, txn_id, category_id, source, rule_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, txn_id) DO UPDATE SET
				category_id = EXCLUDED.category_id,
				source = EXCLUDED.source,
				rule_id = EXCLUDED.rule_id,
				assigned_at = now()
			WHERE category_assignments.category_id IS DISTINCT FROM EXCLUDED.category_id
				OR category_assignments.source IS DISTINCT FROM EXCLUDED.source
				OR category_assignments.rule_id IS DISTINCT FROM EXCLUDED.rule_id
			RETURNING assigned_at
		`, userID.Int64(), assignment.TxnID, assignment.CategoryID, string(assignment.Source), assignment.RuleID).
			Scan(&assignment.AssignedAt)
		if errors.Is(err, sql.ErrNoRows) {
			changed = false
			return nil
		}
		return err
	})
	if err != nil {
		return false, fmt.Errorf("upsert category assignment if changed: %w", err)
	}
	return changed, nil
}

func (r *AssignmentRepo) Delete(ctx context.Context, userID domain.UserID, txnID string) error {
	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM category_assignments
			WHERE user_id = $1 AND txn_id = $2
		`, userID.Int64(), txnID)
		if err != nil {
			return fmt.Errorf("delete category assignment: %w", err)
		}
		return nil
	})
}

func (r *AssignmentRepo) ListByCategory(ctx context.Context, userID domain.UserID, categoryID int64) ([]domain.CategoryAssignment, error) {
	var assignments []domain.CategoryAssignment
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT txn_id, category_id, source, rule_id, assigned_at
			FROM category_assignments
			WHERE user_id = $1 AND category_id = $2
			ORDER BY txn_id ASC
		`, userID.Int64(), categoryID)
		if err != nil {
			return err
		}
		defer func() {
			_ = rows.Close()
		}()

		for rows.Next() {
			var assignment domain.CategoryAssignment
			var ruleID sql.NullInt64
			if err := rows.Scan(
				&assignment.TxnID,
				&assignment.CategoryID,
				&assignment.Source,
				&ruleID,
				&assignment.AssignedAt,
			); err != nil {
				return err
			}
			if ruleID.Valid {
				rule := ruleID.Int64
				assignment.RuleID = &rule
			}
			assignments = append(assignments, assignment)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list category assignments: %w", err)
	}
	return assignments, nil
}
