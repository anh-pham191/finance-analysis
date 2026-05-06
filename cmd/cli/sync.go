package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/akahu"
	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ingest"
	"github.com/anh-pham191/finance-analysis/internal/ports"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

const (
	syncFromLayout      = "2006-01-02"
	defaultAkahuBaseURL = "https://api.akahu.io/v1"
)

type syncOptions struct {
	From *time.Time
}

type ingestResult = ingest.Result

var syncRunner = runSync

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func newSyncCommand(stdout, stderr io.Writer) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync accounts and transactions from Akahu",
		RunE: func(cmd *cobra.Command, args []string) error {
			var parsed *time.Time
			if from != "" {
				fromDate, err := time.Parse(syncFromLayout, from)
				if err != nil {
					return errors.New("--from must be YYYY-MM-DD")
				}
				parsed = &fromDate
			}
			result, err := syncRunner(cmd.Context(), syncOptions{From: parsed})
			if err != nil {
				return err
			}
			printSyncResult(stdout, result)
			return nil
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&from, "from", "", "Sync transactions from YYYY-MM-DD")
	return cmd
}

func runSync(ctx context.Context, opts syncOptions) (ingest.Result, error) {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		return ingest.Result{}, errors.New("DATABASE_URL_APP or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return ingest.Result{}, fmt.Errorf("open database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	baseURL, err := syncAkahuBaseURL(os.Getenv("AKAHU_BASE_URL"))
	if err != nil {
		return ingest.Result{}, err
	}

	deps := ingest.Deps{
		Accounts:   postgres.NewAccountRepo(db),
		Txns:       postgres.NewTxnRepo(db),
		SyncStates: postgres.NewSyncStateRepo(db),
		Tokens:     akahu.EnvTokenStore{},
		NewAkahuClient: func(appToken, userToken string) ports.AkahuClient {
			return akahu.NewClient(akahu.Config{
				AppToken:  appToken,
				UserToken: userToken,
				BaseURL:   baseURL,
			})
		},
		Clock: systemClock{},
	}
	return ingest.Sync(ctx, domain.UserID(1), deps, ingest.Options{From: opts.From})
}

func printSyncResult(stdout io.Writer, result ingest.Result) {
	if result.Accounts == 0 {
		_, _ = fmt.Fprintln(stdout, "Sync complete: 0 accounts found. Check Akahu dashboard connections and token permissions.")
		return
	}
	_, _ = fmt.Fprintf(stdout, "Sync complete: %d accounts, %d transactions fetched.\n", result.Accounts, result.Transactions)
}

func syncAkahuBaseURL(raw string) (string, error) {
	if raw == "" {
		raw = defaultAkahuBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("AKAHU_BASE_URL is invalid")
	}
	return raw, nil
}
