package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type RuleRepo struct {
	db *sql.DB
}

func NewRuleRepo(db *sql.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

func (r *RuleRepo) Insert(ctx context.Context, userID domain.UserID, rule domain.Rule) (domain.Rule, error) {
	predicate := rule.Predicate
	if len(predicate) == 0 {
		predicate = json.RawMessage(`{}`)
	}

	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO rules (user_id, name, priority, predicate, category_id, enabled)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at, updated_at
		`, userID.Int64(), rule.Name, rule.Priority, []byte(predicate), rule.CategoryID, rule.Enabled).
			Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	})
	if err != nil {
		return domain.Rule{}, fmt.Errorf("insert rule: %w", err)
	}
	rule.Predicate = predicate
	return rule, nil
}

func (r *RuleRepo) GetByName(ctx context.Context, userID domain.UserID, name string) (domain.Rule, error) {
	var rule domain.Rule
	var predicate []byte
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, name, priority, predicate, category_id, enabled, created_at, updated_at
			FROM rules
			WHERE user_id = $1 AND name = $2
		`, userID.Int64(), name).Scan(
			&rule.ID,
			&rule.Name,
			&rule.Priority,
			&predicate,
			&rule.CategoryID,
			&rule.Enabled,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
	})
	if err != nil {
		return domain.Rule{}, repoGetError(err)
	}
	rule.Predicate = json.RawMessage(predicate)
	return rule, nil
}

func (r *RuleRepo) Upsert(ctx context.Context, userID domain.UserID, rule domain.Rule) (domain.Rule, error) {
	predicate := rule.Predicate
	if len(predicate) == 0 {
		predicate = json.RawMessage(`{}`)
	}

	var storedPredicate []byte
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO rules (user_id, name, priority, predicate, category_id, enabled)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, name) DO UPDATE SET
				priority = EXCLUDED.priority,
				predicate = EXCLUDED.predicate,
				category_id = EXCLUDED.category_id,
				enabled = EXCLUDED.enabled,
				updated_at = now()
			RETURNING id, name, priority, predicate, category_id, enabled, created_at, updated_at
		`, userID.Int64(), rule.Name, rule.Priority, []byte(predicate), rule.CategoryID, rule.Enabled).Scan(
			&rule.ID,
			&rule.Name,
			&rule.Priority,
			&storedPredicate,
			&rule.CategoryID,
			&rule.Enabled,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
	})
	if err != nil {
		return domain.Rule{}, fmt.Errorf("upsert rule: %w", err)
	}
	rule.Predicate = json.RawMessage(storedPredicate)
	return rule, nil
}

func (r *RuleRepo) List(ctx context.Context, userID domain.UserID) ([]domain.Rule, error) {
	var rules []domain.Rule
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, name, priority, predicate, category_id, enabled, created_at, updated_at
			FROM rules
			WHERE user_id = $1
			ORDER BY priority ASC, name ASC
		`, userID.Int64())
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var rule domain.Rule
			var predicate []byte
			if err := rows.Scan(
				&rule.ID,
				&rule.Name,
				&rule.Priority,
				&predicate,
				&rule.CategoryID,
				&rule.Enabled,
				&rule.CreatedAt,
				&rule.UpdatedAt,
			); err != nil {
				return err
			}
			rule.Predicate = json.RawMessage(predicate)
			rules = append(rules, rule)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	return rules, nil
}

func (r *RuleRepo) DeleteMissing(ctx context.Context, userID domain.UserID, keepNames []string) error {
	return withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		query := `DELETE FROM rules WHERE user_id = $1`
		args := []any{userID.Int64()}
		if len(keepNames) > 0 {
			placeholders := make([]string, len(keepNames))
			for i, name := range keepNames {
				placeholders[i] = fmt.Sprintf("$%d", i+2)
				args = append(args, name)
			}
			query += ` AND name NOT IN (` + strings.Join(placeholders, `, `) + `)`
		}
		_, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("delete missing rules: %w", err)
		}
		return nil
	})
}
