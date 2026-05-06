package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
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
		return domain.Category{}, repoGetError(err)
	}
	if parentID.Valid {
		category.ParentID = &parentID.Int64
	}
	return category, nil
}

func (r *CategoryRepo) Upsert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error) {
	var parentID sql.NullInt64
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO categories (user_id, name, parent_id, kind)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, name) DO UPDATE SET
				parent_id = EXCLUDED.parent_id,
				kind = EXCLUDED.kind
			RETURNING id, name, parent_id, kind
		`, userID.Int64(), category.Name, category.ParentID, string(category.Kind)).Scan(
			&category.ID,
			&category.Name,
			&parentID,
			&category.Kind,
		)
	})
	if err != nil {
		return domain.Category{}, fmt.Errorf("upsert category: %w", err)
	}
	category.ParentID = nil
	if parentID.Valid {
		parent := parentID.Int64
		category.ParentID = &parent
	}
	return category, nil
}

func (r *CategoryRepo) List(ctx context.Context, userID domain.UserID) ([]domain.Category, error) {
	var categories []domain.Category
	err := withUserTx(ctx, r.db, userID, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, name, parent_id, kind
			FROM categories
			WHERE user_id = $1
			ORDER BY name ASC
		`, userID.Int64())
		if err != nil {
			return err
		}
		defer func() {
			_ = rows.Close()
		}()

		for rows.Next() {
			var category domain.Category
			var parentID sql.NullInt64
			if err := rows.Scan(&category.ID, &category.Name, &parentID, &category.Kind); err != nil {
				return err
			}
			if parentID.Valid {
				parent := parentID.Int64
				category.ParentID = &parent
			}
			categories = append(categories, category)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}

func repoGetError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrNotFound
	}
	return err
}
