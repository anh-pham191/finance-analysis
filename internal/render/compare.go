package render

import (
	"encoding/csv"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/anh-pham191/finance-analysis/internal/report"
)

type compareJSON struct {
	A          rangeJSON                `json:"a"`
	B          rangeJSON                `json:"b"`
	Categories []report.CompareCategory `json:"categories"`
}

func RenderCompare(w io.Writer, format Format, result report.CompareResult) error {
	switch format {
	case FormatTable:
		return renderCompareTable(w, result)
	case FormatCSV:
		return renderCompareCSV(w, result)
	case FormatJSON:
		return renderJSON(w, compareJSON{
			A:          jsonRange(result.A),
			B:          jsonRange(result.B),
			Categories: result.Categories,
		})
	case FormatMarkdown:
		return renderCompareMarkdown(w, result)
	default:
		return unknownFormatError(format)
	}
}

func renderCompareCSV(w io.Writer, result report.CompareResult) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"category_id", "category", "kind", "a", "b", "delta", "delta_percent"}); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if err := writer.Write([]string{
			fmt.Sprintf("%d", category.CategoryID),
			category.Category,
			category.Kind,
			string(category.A),
			string(category.B),
			string(category.Delta),
			deltaPercentCSV(category.DeltaPercent),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func renderCompareMarkdown(w io.Writer, result report.CompareResult) error {
	if _, err := fmt.Fprintln(w, "| Category ID | Category | Kind | A | B | Delta | Delta % |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---|---|---|---|---|---|---|"); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if _, err := fmt.Fprintf(w, "| %d | %s | %s | %s | %s | %s | %s |\n",
			category.CategoryID,
			category.Category,
			category.Kind,
			category.A,
			category.B,
			category.Delta,
			deltaPercentText(category.DeltaPercent),
		); err != nil {
			return err
		}
	}
	return nil
}

func renderCompareTable(w io.Writer, result report.CompareResult) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "Compare"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, "Category ID\tCategory\tKind\tA\tB\tDelta\tDelta %"); err != nil {
		return err
	}
	for _, category := range result.Categories {
		if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			category.CategoryID,
			category.Category,
			category.Kind,
			category.A,
			category.B,
			category.Delta,
			deltaPercentText(category.DeltaPercent),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func deltaPercentCSV(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *value)
}

func deltaPercentText(value *float64) string {
	if value == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", *value)
}
