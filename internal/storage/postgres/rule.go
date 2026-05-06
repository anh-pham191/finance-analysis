package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
		return domain.Rule{}, err
	}
	rule.Predicate = json.RawMessage(predicate)
	return rule, nil
}
