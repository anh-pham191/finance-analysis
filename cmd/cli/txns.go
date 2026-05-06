package main

import (
	"context"
	"fmt"
	"io"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/render"
	"github.com/anh-pham191/finance-analysis/internal/report"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type txnsOptions struct {
	Period       domain.Range
	CategoryName string
	Merchant     string
	AccountID    string
	Direction    *domain.Direction
	Min          *domain.Money
	Max          *domain.Money
	Sort         string
	Limit        int
	Offset       int
	Format       render.Format
	Stdout       io.Writer
}

var txnsRunner = runTxns

func newTxnsCommand(stdout, stderr io.Writer) *cobra.Command {
	var periodValue string
	var formatValue string
	var directionValue string
	var minValue string
	var maxValue string
	opts := txnsOptions{
		Sort:   "date",
		Limit:  100,
		Stdout: stdout,
	}
	cmd := &cobra.Command{
		Use:   "txns",
		Short: "List transactions matching reporting filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseRenderFormat(formatValue)
			if err != nil {
				return err
			}
			if opts.Limit < 0 {
				return fmt.Errorf("--limit must be >= 0")
			}
			if opts.Offset < 0 {
				return fmt.Errorf("--offset must be >= 0")
			}
			period, err := resolveCLIReportPeriod(periodValue)
			if err != nil {
				return err
			}
			direction, err := parseOptionalDirection(directionValue)
			if err != nil {
				return err
			}
			min, err := parseOptionalMoney("--min", minValue)
			if err != nil {
				return err
			}
			max, err := parseOptionalMoney("--max", maxValue)
			if err != nil {
				return err
			}
			opts.Period = period
			opts.Format = format
			opts.Direction = direction
			opts.Min = min
			opts.Max = max
			return txnsRunner(cmd.Context(), opts)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&periodValue, "period", "this-month", "Reporting period")
	cmd.Flags().StringVar(&opts.CategoryName, "category", "", "Category name")
	cmd.Flags().StringVar(&opts.Merchant, "merchant", "", "Merchant name")
	cmd.Flags().StringVar(&opts.AccountID, "account", "", "Account ID")
	cmd.Flags().StringVar(&directionValue, "direction", "", "Direction: debit or credit")
	cmd.Flags().StringVar(&minValue, "min", "", "Minimum amount")
	cmd.Flags().StringVar(&maxValue, "max", "", "Maximum amount")
	cmd.Flags().StringVar(&opts.Sort, "sort", "date", "Sort: date, amount, or merchant")
	cmd.Flags().IntVar(&opts.Limit, "limit", 100, "Maximum rows to return")
	cmd.Flags().IntVar(&opts.Offset, "offset", 0, "Rows to skip")
	cmd.Flags().StringVar(&formatValue, "format", "table", "Output format: table, csv, json, md")
	return cmd
}

func runTxns(ctx context.Context, opts txnsOptions) error {
	db, err := openAppDB()
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	userID := domain.UserID(1)
	categoryRepo := postgres.NewCategoryRepo(db)
	var categoryID *int64
	if opts.CategoryName != "" {
		category, err := categoryRepo.GetByName(ctx, userID, opts.CategoryName)
		if err != nil {
			return fmt.Errorf("get category %q: %w", opts.CategoryName, err)
		}
		categoryID = &category.ID
	}

	txnRepo := postgres.NewTxnRepo(db)
	rows, err := report.Transactions(ctx, userID, report.TransactionsDeps{Txns: txnRepo}, report.TxnFilter{
		Period:     opts.Period,
		CategoryID: categoryID,
		Merchant:   opts.Merchant,
		AccountID:  opts.AccountID,
		Direction:  opts.Direction,
		Min:        opts.Min,
		Max:        opts.Max,
		Sort:       opts.Sort,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
	})
	if err != nil {
		return err
	}
	return render.RenderTransactions(opts.Stdout, opts.Format, rows)
}

func parseOptionalDirection(value string) (*domain.Direction, error) {
	if value == "" {
		return nil, nil
	}
	direction, err := domain.ParseDirection(value)
	if err != nil {
		return nil, err
	}
	return &direction, nil
}

func parseOptionalMoney(flagName, value string) (*domain.Money, error) {
	if value == "" {
		return nil, nil
	}
	money, err := domain.NewMoneyFromString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be a decimal amount: %w", flagName, err)
	}
	return &money, nil
}
