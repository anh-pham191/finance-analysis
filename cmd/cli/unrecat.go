package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type unrecatOptions struct {
	TxnID string
}

var unrecatRunner = runUnrecat

func newUnrecatCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unrecat <txn-id>",
		Short: "Clear a transaction category assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return unrecatRunner(cmd.Context(), unrecatOptions{TxnID: args[0]})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

func runUnrecat(ctx context.Context, opts unrecatOptions) error {
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

	if err := postgres.NewAssignmentRepo(db).Delete(ctx, domain.UserID(1), opts.TxnID); err != nil {
		return fmt.Errorf("clear assignment: %w", err)
	}
	return nil
}
