package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/report"
)

func TestRenderSummaryJSONUsesStableSnakeCaseFields(t *testing.T) {
	var out bytes.Buffer

	err := RenderSummary(&out, FormatJSON, sampleSummary())

	if err != nil {
		t.Fatalf("RenderSummary returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("summary JSON is invalid: %v", err)
	}
	if _, ok := got["has_uncategorised"]; !ok {
		t.Fatalf("summary JSON missing has_uncategorised: %s", out.String())
	}
	categories, ok := got["categories"].([]any)
	if !ok || len(categories) != 1 {
		t.Fatalf("summary JSON categories = %#v", got["categories"])
	}
	category, ok := categories[0].(map[string]any)
	if !ok {
		t.Fatalf("summary JSON category = %#v", categories[0])
	}
	if _, ok := category["category_id"]; !ok {
		t.Fatalf("summary JSON category missing category_id: %s", out.String())
	}
}

func TestRenderSummaryCSVHasHeaderAndCategoryRow(t *testing.T) {
	var out bytes.Buffer

	err := RenderSummary(&out, FormatCSV, sampleSummary())

	if err != nil {
		t.Fatalf("RenderSummary returned error: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("summary CSV is invalid: %v", err)
	}
	want := [][]string{
		{"category_id", "category", "kind", "total"},
		{"12", "Groceries", "expense", "123.45"},
	}
	if !equalRecords(records, want) {
		t.Fatalf("summary CSV records = %#v, want %#v", records, want)
	}
}

func TestRenderSummaryMarkdownUsesPipeTable(t *testing.T) {
	var out bytes.Buffer

	err := RenderSummary(&out, FormatMarkdown, sampleSummary())

	if err != nil {
		t.Fatalf("RenderSummary returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "| Category ID | Category | Kind | Total |") {
		t.Fatalf("summary markdown missing pipe header: %q", got)
	}
	if !strings.Contains(got, "| 12 | Groceries | expense | 123.45 |") {
		t.Fatalf("summary markdown missing category row: %q", got)
	}
}

func TestRenderSummaryTableIncludesUncategorisedWarning(t *testing.T) {
	var out bytes.Buffer

	err := RenderSummary(&out, FormatTable, sampleSummary())

	if err != nil {
		t.Fatalf("RenderSummary returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Summary") || !strings.Contains(got, "Groceries") {
		t.Fatalf("summary table is not human-readable: %q", got)
	}
	if !strings.Contains(got, "Warning: uncategorised transactions are included") {
		t.Fatalf("summary table missing uncategorised warning: %q", got)
	}
}

func TestRenderCompareTableAndMarkdownUseDashForNilDeltaPercent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format Format
	}{
		{name: "table", format: FormatTable},
		{name: "markdown", format: FormatMarkdown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			err := RenderCompare(&out, tc.format, sampleCompare())

			if err != nil {
				t.Fatalf("RenderCompare returned error: %v", err)
			}
			if !strings.Contains(out.String(), "—") {
				t.Fatalf("compare %s missing dash for nil delta percent: %q", tc.name, out.String())
			}
		})
	}
}

func TestRenderCompareCSVUsesEmptyCellForNilDeltaPercent(t *testing.T) {
	var out bytes.Buffer

	err := RenderCompare(&out, FormatCSV, sampleCompare())

	if err != nil {
		t.Fatalf("RenderCompare returned error: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("compare CSV is invalid: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("compare CSV records = %#v", records)
	}
	if got := records[1][6]; got != "" {
		t.Fatalf("nil delta percent CSV cell = %q, want empty", got)
	}
}

func TestRenderTransactionsJSONUsesStableSnakeCaseFields(t *testing.T) {
	var out bytes.Buffer

	err := RenderTransactions(&out, FormatJSON, sampleTransactions())

	if err != nil {
		t.Fatalf("RenderTransactions returned error: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("transactions JSON is invalid: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("transactions JSON rows = %#v", got)
	}
	if _, ok := got[0]["txn_id"]; !ok {
		t.Fatalf("transactions JSON missing txn_id: %s", out.String())
	}
	if _, ok := got[0]["account_id"]; !ok {
		t.Fatalf("transactions JSON missing account_id: %s", out.String())
	}
}

func TestRenderTransactionsCSVHasHeader(t *testing.T) {
	var out bytes.Buffer

	err := RenderTransactions(&out, FormatCSV, sampleTransactions())

	if err != nil {
		t.Fatalf("RenderTransactions returned error: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("transactions CSV is invalid: %v", err)
	}
	wantHeader := []string{"txn_id", "posted_at", "account_id", "category", "direction", "amount", "merchant", "description"}
	if len(records) == 0 || !equalRecord(records[0], wantHeader) {
		t.Fatalf("transactions CSV header = %#v, want %#v", records, wantHeader)
	}
}

func TestRenderUnknownFormatErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(ioFormat Format) error
	}{
		{name: "summary", run: func(format Format) error { return RenderSummary(&bytes.Buffer{}, format, sampleSummary()) }},
		{name: "compare", run: func(format Format) error { return RenderCompare(&bytes.Buffer{}, format, sampleCompare()) }},
		{name: "transactions", run: func(format Format) error { return RenderTransactions(&bytes.Buffer{}, format, sampleTransactions()) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(Format("xml"))
			if err == nil {
				t.Fatal("expected error for unknown format")
			}
			if !strings.Contains(err.Error(), `unknown render format "xml"`) {
				t.Fatalf("unknown format error = %q", err.Error())
			}
		})
	}
}

func sampleSummary() report.SummaryResult {
	return report.SummaryResult{
		Period: domain.Range{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		Income:           report.MoneyAmount("3000.00"),
		Expense:          report.MoneyAmount("123.45"),
		Net:              report.MoneyAmount("2876.55"),
		HasUncategorised: true,
		Categories: []report.CategoryTotal{
			{CategoryID: 12, Category: "Groceries", Kind: "expense", Total: report.MoneyAmount("123.45")},
		},
	}
}

func sampleCompare() report.CompareResult {
	return report.CompareResult{
		A: domain.Range{
			From: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		B: domain.Range{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		Categories: []report.CompareCategory{
			{
				CategoryID:   12,
				Category:     "Groceries",
				Kind:         "expense",
				A:            report.MoneyAmount("0.00"),
				B:            report.MoneyAmount("123.45"),
				Delta:        report.MoneyAmount("123.45"),
				DeltaPercent: nil,
			},
		},
	}
}

func sampleTransactions() []report.TxnRow {
	return []report.TxnRow{
		{
			TxnID:       "txn_123",
			PostedAt:    time.Date(2026, 4, 2, 12, 30, 0, 0, time.UTC),
			AccountID:   "acc_123",
			Category:    "Groceries",
			Direction:   "debit",
			Amount:      report.MoneyAmount("-123.45"),
			Merchant:    "Market",
			Description: "Weekly shop",
		},
	}
}

func equalRecords(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalRecord(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalRecord(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
