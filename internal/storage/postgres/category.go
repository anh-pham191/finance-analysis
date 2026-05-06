package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type CategoryRepo struct {
	db *sql.DB
}

func NewCategoryRepo(db *sql.DB) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func (r *CategoryRepo) Insert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error) {
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO categories (user_id, name, parent_id, kind)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, userID.Int64(), category.Name, category.ParentID, string(category.Kind)).Scan(&category.ID)
	})
	if err != nil {
		return domain.Category{}, fmt.Errorf("insert category: %w", err)
	}
	return category, nil
}

func (r *CategoryRepo) GetByName(ctx context.Context, userID domain.UserID, name string) (domain.Category, error) {
	var category domain.Category
	var parentID sql.NullInt64
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, name, parent_id, kind
			FROM categories
			WHERE user_id = $1 AND name = $2
		`, userID.Int64(), name).Scan(&category.ID, &category.Name, &parentID, &category.Kind)
	})
	if err != nil {
		return domain.Category{}, err
	}
	if parentID.Valid {
		category.ParentID = &parentID.Int64
	}
	return category, nil
}
