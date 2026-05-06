package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/render"
	"github.com/anh-pham191/finance-analysis/internal/report"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type summaryOptions struct {
	Period domain.Range
	Format render.Format
	Stdout io.Writer
}

var (
	summaryRunner = runSummary
	reportingNow  = time.Now
)

func newSummaryCommand(stdout, stderr io.Writer) *cobra.Command {
	var periodValue string
	var formatValue string
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarise income and spending by category",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseRenderFormat(formatValue)
			if err != nil {
				return err
			}
			period, err := resolveCLIReportPeriod(periodValue)
			if err != nil {
				return err
			}
			return summaryRunner(cmd.Context(), summaryOptions{
				Period: period,
				Format: format,
				Stdout: stdout,
			})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&periodValue, "period", "this-month", "Reporting period")
	cmd.Flags().StringVar(&formatValue, "format", "table", "Output format: table, csv, json, md")
	return cmd
}

func runSummary(ctx context.Context, opts summaryOptions) error {
	db, err := openAppDB()
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	txnRepo := postgres.NewTxnRepo(db)
	deps := report.SummaryDeps{
		Txns:        txnRepo,
		Categories:  postgres.NewCategoryRepo(db),
		Assignments: postgres.NewAssignmentRepo(db),
	}
	result, err := report.Summary(ctx, domain.UserID(1), deps, opts.Period)
	if err != nil {
		return err
	}
	return render.RenderSummary(opts.Stdout, opts.Format, result)
}

func openAppDB() (*sql.DB, error) {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL_APP or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

func parseRenderFormat(value string) (render.Format, error) {
	switch render.Format(value) {
	case render.FormatTable, render.FormatCSV, render.FormatJSON, render.FormatMarkdown:
		return render.Format(value), nil
	default:
		return "", fmt.Errorf("invalid format %q: want table, csv, json, or md", value)
	}
}

func resolveCLIReportPeriod(value string) (domain.Range, error) {
	period, err := domain.ParsePeriod(value)
	if err != nil {
		return domain.Range{}, err
	}
	loc, err := reportingLocation()
	if err != nil {
		return domain.Range{}, err
	}
	resolved, err := period.Resolve(loc, reportingNow())
	if err != nil {
		return domain.Range{}, err
	}
	return resolved, nil
}

func reportingLocation() (*time.Location, error) {
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return nil, fmt.Errorf("load Pacific/Auckland timezone: %w", err)
	}
	return loc, nil
}
