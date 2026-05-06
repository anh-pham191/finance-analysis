package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/akahu"
	"github.com/anh-pham191/finance-analysis/internal/observability"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

var (
	pingHealthDB     = defaultPingHealthDB
	checkAkahuHealth = defaultCheckAkahuHealth
)

func newHealthCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check database connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()

			if err := pingHealthDB(ctx); err != nil {
				return err
			}
			if akahuHealthConfigured() {
				if err := checkAkahuHealth(ctx); err != nil {
					return errors.New(redactHealthError(fmt.Sprintf("check Akahu: %v", err)))
				}
			}
			_, _ = fmt.Fprintln(stdout, "ok")
			return nil
		},
	}
}

func defaultPingHealthDB(ctx context.Context) error {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL_APP or DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func akahuHealthConfigured() bool {
	return os.Getenv("AKAHU_APP_TOKEN") != "" && os.Getenv("AKAHU_USER_TOKEN") != ""
}

func defaultCheckAkahuHealth(ctx context.Context) error {
	baseURL, err := syncAkahuBaseURL(os.Getenv("AKAHU_BASE_URL"))
	if err != nil {
		return err
	}

	client := akahu.NewClient(akahu.Config{
		AppToken:  os.Getenv("AKAHU_APP_TOKEN"),
		UserToken: os.Getenv("AKAHU_USER_TOKEN"),
		BaseURL:   baseURL,
	})
	if _, err := client.ListAccounts(ctx); err != nil {
		return errors.New(observability.RedactString(err.Error()))
	}
	return nil
}

func redactHealthError(message string) string {
	for _, token := range []string{os.Getenv("AKAHU_APP_TOKEN"), os.Getenv("AKAHU_USER_TOKEN")} {
		if token != "" {
			message = strings.ReplaceAll(message, token, "***")
		}
	}
	return observability.RedactString(message)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
