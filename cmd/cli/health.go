package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

func newHealthCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check database connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
			if dsn == "" {
				return errors.New("DATABASE_URL_APP or DATABASE_URL is required")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			db, err := sql.Open("pgx", dsn)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer db.Close()

			if err := db.PingContext(ctx); err != nil {
				return fmt.Errorf("ping database: %w", err)
			}
			_, _ = fmt.Fprintln(stdout, "ok")
			return nil
		},
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
