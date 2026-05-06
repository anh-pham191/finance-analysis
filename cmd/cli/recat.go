package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/anh-pham191/finance-analysis/internal/categorise"
	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type recatOptions struct {
	TxnID          string
	CategoryName   string
	CategoriesPath string
	Categories     []categorise.Category
}

var recatRunner = runRecat

func newRecatCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := recatOptions{
		CategoriesPath: defaultCategoriesPath,
	}
	cmd := &cobra.Command{
		Use:   "recat <txn-id> <category>",
		Short: "Manually assign a transaction category",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			categories, err := categorise.LoadCategories(opts.CategoriesPath)
			if err != nil {
				return fmt.Errorf("load categories %q: %w", opts.CategoriesPath, err)
			}
			if !categoryDeclared(categories, args[1]) {
				return fmt.Errorf("unknown category %q; candidates: %s", args[1], strings.Join(categoryNames(categories), ", "))
			}
			opts.TxnID = args[0]
			opts.CategoryName = args[1]
			opts.Categories = categories
			return recatRunner(cmd.Context(), opts)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&opts.CategoriesPath, "categories", defaultCategoriesPath, "Path to categories YAML")
	return cmd
}

func runRecat(ctx context.Context, opts recatOptions) error {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL_APP or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	userID := domain.UserID(1)
	categoryRepo := postgres.NewCategoryRepo(db)
	category, err := upsertDeclaredCategory(ctx, userID, categoryRepo, opts.Categories, opts.CategoryName)
	if err != nil {
		return err
	}

	_, err = postgres.NewAssignmentRepo(db).UpsertIfChanged(ctx, userID, domain.CategoryAssignment{
		TxnID:      opts.TxnID,
		CategoryID: category.ID,
		Source:     domain.AssignmentSourceManual,
		RuleID:     nil,
	})
	if err != nil {
		return fmt.Errorf("set manual assignment: %w", err)
	}
	return nil
}

func categoryDeclared(categories []categorise.Category, name string) bool {
	for _, category := range categories {
		if category.Name == name {
			return true
		}
	}
	return false
}

func categoryNames(categories []categorise.Category) []string {
	names := make([]string, 0, len(categories))
	for _, category := range categories {
		names = append(names, category.Name)
	}
	sort.Strings(names)
	return names
}

type categoryUpserter interface {
	Upsert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error)
}

func upsertDeclaredCategory(ctx context.Context, userID domain.UserID, repo categoryUpserter, categories []categorise.Category, name string) (domain.Category, error) {
	byName := make(map[string]categorise.Category, len(categories))
	for _, category := range categories {
		byName[category.Name] = category
	}
	declared, ok := byName[name]
	if !ok {
		return domain.Category{}, fmt.Errorf("category %q is not declared", name)
	}
	upserted := make(map[string]domain.Category, len(categories))
	return upsertCategoryWithParents(ctx, userID, repo, byName, upserted, declared)
}

func upsertCategoryWithParents(ctx context.Context, userID domain.UserID, repo categoryUpserter, byName map[string]categorise.Category, upserted map[string]domain.Category, category categorise.Category) (domain.Category, error) {
	if existing, ok := upserted[category.Name]; ok {
		return existing, nil
	}

	var parentID *int64
	if category.Parent != "" {
		parent, ok := byName[category.Parent]
		if !ok {
			return domain.Category{}, fmt.Errorf("category %q references unknown parent %q", category.Name, category.Parent)
		}
		upsertedParent, err := upsertCategoryWithParents(ctx, userID, repo, byName, upserted, parent)
		if err != nil {
			return domain.Category{}, err
		}
		parentID = &upsertedParent.ID
	}

	upsertedCategory, err := repo.Upsert(ctx, userID, domain.Category{
		Name:     category.Name,
		ParentID: parentID,
		Kind:     domain.CategoryKind(category.Kind),
	})
	if err != nil {
		return domain.Category{}, fmt.Errorf("upsert category %q: %w", category.Name, err)
	}
	upserted[category.Name] = upsertedCategory
	return upsertedCategory, nil
}
