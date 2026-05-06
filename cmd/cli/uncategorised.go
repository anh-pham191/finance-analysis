package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/anh-pham191/finance-analysis/internal/categorise"
	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type uncategorisedOptions struct {
	CategoriesPath string
	Categories     []categorise.Category
}

var uncategorisedRunner = runUncategorised

func newUncategorisedCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := uncategorisedOptions{
		CategoriesPath: defaultCategoriesPath,
	}
	cmd := &cobra.Command{
		Use:   "uncategorised",
		Short: "List transactions assigned to Uncategorised",
		RunE: func(cmd *cobra.Command, args []string) error {
			categories, err := categorise.LoadCategories(opts.CategoriesPath)
			if err != nil {
				return fmt.Errorf("load categories %q: %w", opts.CategoriesPath, err)
			}
			if !categoryDeclared(categories, "Uncategorised") {
				return fmt.Errorf("categories %q must declare Uncategorised", opts.CategoriesPath)
			}
			opts.Categories = categories

			assignments, err := uncategorisedRunner(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if len(assignments) == 0 {
				_, err = fmt.Fprintln(stdout, "No uncategorised transactions.")
				return err
			}
			for _, assignment := range assignments {
				if _, err := fmt.Fprintln(stdout, assignment.TxnID); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&opts.CategoriesPath, "categories", defaultCategoriesPath, "Path to categories YAML")
	return cmd
}

func runUncategorised(ctx context.Context, opts uncategorisedOptions) ([]domain.CategoryAssignment, error) {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL_APP or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	userID := domain.UserID(1)
	category, err := postgres.NewCategoryRepo(db).GetByName(ctx, userID, "Uncategorised")
	if err != nil {
		return nil, fmt.Errorf("get Uncategorised category: %w", err)
	}
	assignments, err := postgres.NewAssignmentRepo(db).ListByCategory(ctx, userID, category.ID)
	if err != nil {
		return nil, fmt.Errorf("list uncategorised assignments: %w", err)
	}
	return assignments, nil
}
