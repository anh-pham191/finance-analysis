package render

import (
	"encoding/csv"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/anh-pham191/finance-analysis/internal/report"
)

type summaryJSON struct {
	Period           rangeJSON              `json:"period"`
	Income           report.MoneyAmount     `json:"income"`
	Expense          report.MoneyAmount     `json:"expense"`
	Net              report.MoneyAmount     `json:"net"`
	Categories       []report.CategoryTotal `json:"categories"`
	HasUncategorised bool                   `json:"has_uncategorised"`
}

func RenderSummary(w io.Writer, format Format, result report.SummaryResult) error {
	switch format {
	case FormatTable:
		return renderSummaryTable(w, result)
	case FormatCSV:
		return renderSummaryCSV(w, result)
	case FormatJSON:
		return renderJSON(w, summaryJSON{
			Period:           jsonRange(result.Period),
			Income:           result.Income,
			Expense:          result.Expense,
			Net:              result.Net,
			Categories:       result.Categories,
			HasUncategorised: result.HasUncategorised,
		})
	case FormatMarkdown:
		return renderSummaryMarkdown(w, result)
	default:
		return unknownFormatError(format)
	}
}

func renderSummaryCSV(w io.Writer, result report.SummaryResult) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"category_id", "category", "kind", "total"}); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if err := writer.Write([]string{
			fmt.Sprintf("%d", category.CategoryID),
			category.Category,
			category.Kind,
			string(category.Total),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func renderSummaryMarkdown(w io.Writer, result report.SummaryResult) error {
	if _, err := fmt.Fprintf(w, "Income: %s\nExpense: %s\nNet: %s\n\n", result.Income, result.Expense, result.Net); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Category ID | Category | Kind | Total |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---|---|---|---|"); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if _, err := fmt.Fprintf(w, "| %d | %s | %s | %s |\n", category.CategoryID, category.Category, category.Kind, category.Total); err != nil {
			return err
		}
	}
	return nil
}

func renderSummaryTable(w io.Writer, result report.SummaryResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "Summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "Income\t%s\nExpense\t%s\nNet\t%s\n\n", result.Income, result.Expense, result.Net); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, "Category ID\tCategory\tKind\tTotal"); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", category.CategoryID, category.Category, category.Kind, category.Total); err != nil {
			return err
		}
	}
	if result.HasUncategorised {
		if _, err := fmt.Fprintln(tw, "\nWarning: uncategorised transactions are included"); err != nil {
			return err
		}
	}
	return tw.Flush()
}
