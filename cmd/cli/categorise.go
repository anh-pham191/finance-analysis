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

const (
	defaultCategoriesPath = "config/categories.yaml"
	defaultRulesPath      = "config/rules.yaml"
)

type categoriseOptions struct {
	CategoriesPath string
	RulesPath      string
	Config         categorise.Config
}

var categoriseRunner = runCategorise

func newCategoriseCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := categoriseOptions{
		CategoriesPath: defaultCategoriesPath,
		RulesPath:      defaultRulesPath,
	}
	cmd := &cobra.Command{
		Use:   "categorise",
		Short: "Categorise transactions from YAML rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			categories, err := categorise.LoadCategories(opts.CategoriesPath)
			if err != nil {
				return fmt.Errorf("load categories %q: %w", opts.CategoriesPath, err)
			}
			rules, err := categorise.LoadRules(opts.RulesPath, categories)
			if err != nil {
				return fmt.Errorf("load rules %q: %w", opts.RulesPath, err)
			}
			opts.Config = categorise.Config{
				Categories: categories,
				Rules:      rules,
			}
			return categoriseRunner(cmd.Context(), opts)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&opts.CategoriesPath, "categories", defaultCategoriesPath, "Path to categories YAML")
	cmd.Flags().StringVar(&opts.RulesPath, "rules", defaultRulesPath, "Path to rules YAML")
	return cmd
}

func runCategorise(ctx context.Context, opts categoriseOptions) error {
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

	deps := categorise.Deps{
		Categories:  postgres.NewCategoryRepo(db),
		Rules:       postgres.NewRuleRepo(db),
		Assignments: postgres.NewAssignmentRepo(db),
		Txns:        postgres.NewTxnRepo(db),
		Clock:       systemClock{},
	}
	return categorise.Categorise(ctx, domain.UserID(1), deps, opts.Config)
}
