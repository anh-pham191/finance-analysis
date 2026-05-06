package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/render"
	"github.com/anh-pham191/finance-analysis/internal/report"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	"github.com/spf13/cobra"
)

type compareOptions struct {
	A      domain.Range
	B      domain.Range
	Format render.Format
	Top    int
	Stdout io.Writer
}

var compareRunner = runCompare

func newCompareCommand(stdout, stderr io.Writer) *cobra.Command {
	var formatValue string
	opts := compareOptions{Stdout: stdout}
	var mom bool
	var wow bool
	var yoy bool
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare category spending between two periods",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseRenderFormat(formatValue)
			if err != nil {
				return err
			}
			if opts.Top < 0 {
				return fmt.Errorf("--top must be >= 0")
			}
			a, b, err := resolveCompareRanges(mom, wow, yoy)
			if err != nil {
				return err
			}
			opts.A = a
			opts.B = b
			opts.Format = format
			return compareRunner(cmd.Context(), opts)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().BoolVar(&mom, "mom", false, "Compare previous complete month with the month before it")
	cmd.Flags().BoolVar(&wow, "wow", false, "Compare previous complete ISO week with the week before it")
	cmd.Flags().BoolVar(&yoy, "yoy", false, "Compare this calendar year with last calendar year")
	cmd.Flags().StringVar(&formatValue, "format", "table", "Output format: table, csv, json, md")
	cmd.Flags().IntVar(&opts.Top, "top", 0, "Limit categories by largest absolute delta")
	return cmd
}

func runCompare(ctx context.Context, opts compareOptions) error {
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
	result, err := report.Compare(ctx, domain.UserID(1), deps, opts.A, opts.B, report.CompareOptions{Top: opts.Top})
	if err != nil {
		return err
	}
	return render.RenderCompare(opts.Stdout, opts.Format, result)
}

func resolveCompareRanges(mom, wow, yoy bool) (domain.Range, domain.Range, error) {
	selected := 0
	for _, value := range []bool{mom, wow, yoy} {
		if value {
			selected++
		}
	}
	if selected != 1 {
		return domain.Range{}, domain.Range{}, fmt.Errorf("choose exactly one of --mom, --wow, or --yoy")
	}

	loc, err := reportingLocation()
	if err != nil {
		return domain.Range{}, domain.Range{}, err
	}
	now := reportingNow().In(loc)
	switch {
	case mom:
		bFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -1, 0)
		b := domain.Range{From: bFrom, To: bFrom.AddDate(0, 1, 0)}
		aFrom := bFrom.AddDate(0, -1, 0)
		return domain.Range{From: aFrom, To: bFrom}, b, nil
	case wow:
		thisWeek := startOfCLIISOWeek(now, loc)
		b := domain.Range{From: thisWeek.AddDate(0, 0, -7), To: thisWeek}
		return domain.Range{From: thisWeek.AddDate(0, 0, -14), To: b.From}, b, nil
	case yoy:
		bFrom := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		b := domain.Range{From: bFrom, To: bFrom.AddDate(1, 0, 0)}
		aFrom := bFrom.AddDate(-1, 0, 0)
		return domain.Range{From: aFrom, To: bFrom}, b, nil
	default:
		return domain.Range{}, domain.Range{}, fmt.Errorf("choose exactly one of --mom, --wow, or --yoy")
	}
}

func startOfCLIISOWeek(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1-weekday)
}
