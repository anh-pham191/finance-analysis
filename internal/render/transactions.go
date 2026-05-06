package render

import (
	"encoding/csv"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/anh-pham191/finance-analysis/internal/report"
)

func RenderTransactions(w io.Writer, format Format, rows []report.TxnRow) error {
	switch format {
	case FormatTable:
		return renderTransactionsTable(w, rows)
	case FormatCSV:
		return renderTransactionsCSV(w, rows)
	case FormatJSON:
		return renderJSON(w, rows)
	case FormatMarkdown:
		return renderTransactionsMarkdown(w, rows)
	default:
		return unknownFormatError(format)
	}
}

func renderTransactionsCSV(w io.Writer, rows []report.TxnRow) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(transactionHeaders()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			row.TxnID,
			row.PostedAt.Format(timeFormat),
			row.AccountID,
			row.Category,
			row.Direction,
			string(row.Amount),
			row.Merchant,
			row.Description,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func renderTransactionsMarkdown(w io.Writer, rows []report.TxnRow) error {
	if _, err := fmt.Fprintln(w, "| Txn ID | Posted At | Account ID | Category | Direction | Amount | Merchant | Description |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			row.TxnID,
			row.PostedAt.Format(timeFormat),
			row.AccountID,
			row.Category,
			row.Direction,
			row.Amount,
			row.Merchant,
			row.Description,
		); err != nil {
			return err
		}
	}
	return nil
}

func renderTransactionsTable(w io.Writer, rows []report.TxnRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "Transactions"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, "Txn ID\tPosted At\tAccount ID\tCategory\tDirection\tAmount\tMerchant\tDescription"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.TxnID,
			row.PostedAt.Format(timeFormat),
			row.AccountID,
			row.Category,
			row.Direction,
			row.Amount,
			row.Merchant,
			row.Description,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func transactionHeaders() []string {
	return []string{"txn_id", "posted_at", "account_id", "category", "direction", "amount", "merchant", "description"}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"
