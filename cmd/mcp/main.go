// Command finance-mcp exposes the finance-analysis reporting commands
// as an MCP server over stdio, for use with Claude Desktop.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/render"
	"github.com/anh-pham191/finance-analysis/internal/report"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const userID = domain.UserID(1)

type server struct {
	db  *sql.DB
	loc *time.Location
}

func main() {
	dsn := firstEnv("DATABASE_URL_APP", "DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL_APP or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		log.Fatalf("load timezone: %v", err)
	}

	s := &server{db: db, loc: loc}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "finance-analysis",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "summary",
		Description: "Summarise income and spending by category for a period. " +
			"Period accepts 'this-month', 'last-month', 'this-year', 'last-year', " +
			"'this-week', 'last-week', or explicit 'YYYY', 'YYYY-MM', or 'YYYY-Www'.",
	}, s.summary)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "compare",
		Description: "Compare category totals between two periods (period_a vs period_b). " +
			"Each period uses the same syntax as the summary tool.",
	}, s.compare)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name: "list_txns",
		Description: "List transactions matching filters within a period. " +
			"All filters are optional except period.",
	}, s.listTxns)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_uncategorised",
		Description: "List transaction IDs currently assigned to the Uncategorised category.",
	}, s.listUncategorised)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_categories",
		Description: "List all configured categories with their kinds and parents.",
	}, s.listCategories)

	if err := mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mcp server: %v", err)
	}
}

type periodInput struct {
	Period string `json:"period" jsonschema:"reporting period (e.g. this-month, last-month, this-year, 2025, 2025-03, 2025-W12)"`
}

type textOutput struct {
	JSON string `json:"json" jsonschema:"reporting result rendered as JSON"`
}

func (s *server) summary(ctx context.Context, _ *mcp.CallToolRequest, in periodInput) (*mcp.CallToolResult, textOutput, error) {
	period, err := s.resolvePeriod(in.Period)
	if err != nil {
		return nil, textOutput{}, err
	}
	deps := report.SummaryDeps{
		Txns:        postgres.NewTxnRepo(s.db),
		Categories:  postgres.NewCategoryRepo(s.db),
		Assignments: postgres.NewAssignmentRepo(s.db),
	}
	result, err := report.Summary(ctx, userID, deps, period)
	if err != nil {
		return nil, textOutput{}, err
	}
	return renderJSON(func(w *bytes.Buffer) error {
		return render.RenderSummary(w, render.FormatJSON, result)
	})
}

type compareInput struct {
	PeriodA string `json:"period_a" jsonschema:"earlier period to compare"`
	PeriodB string `json:"period_b" jsonschema:"later period to compare"`
	Top     int    `json:"top,omitempty" jsonschema:"if > 0, limit result to top N categories by absolute delta"`
}

func (s *server) compare(ctx context.Context, _ *mcp.CallToolRequest, in compareInput) (*mcp.CallToolResult, textOutput, error) {
	a, err := s.resolvePeriod(in.PeriodA)
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("period_a: %w", err)
	}
	b, err := s.resolvePeriod(in.PeriodB)
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("period_b: %w", err)
	}
	if in.Top < 0 {
		return nil, textOutput{}, errors.New("top must be >= 0")
	}
	deps := report.SummaryDeps{
		Txns:        postgres.NewTxnRepo(s.db),
		Categories:  postgres.NewCategoryRepo(s.db),
		Assignments: postgres.NewAssignmentRepo(s.db),
	}
	result, err := report.Compare(ctx, userID, deps, a, b, report.CompareOptions{Top: in.Top})
	if err != nil {
		return nil, textOutput{}, err
	}
	return renderJSON(func(w *bytes.Buffer) error {
		return render.RenderCompare(w, render.FormatJSON, result)
	})
}

type listTxnsInput struct {
	Period    string `json:"period" jsonschema:"reporting period"`
	Category  string `json:"category,omitempty" jsonschema:"exact category name (e.g. Food/Groceries)"`
	Merchant  string `json:"merchant,omitempty" jsonschema:"merchant filter"`
	Account   string `json:"account,omitempty" jsonschema:"account ID filter"`
	Direction string `json:"direction,omitempty" jsonschema:"debit or credit"`
	Min       string `json:"min,omitempty" jsonschema:"minimum absolute amount, decimal string"`
	Max       string `json:"max,omitempty" jsonschema:"maximum absolute amount, decimal string"`
	Sort      string `json:"sort,omitempty" jsonschema:"date, amount, or merchant (default date)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"max rows (default 100)"`
	Offset    int    `json:"offset,omitempty" jsonschema:"rows to skip (default 0)"`
}

func (s *server) listTxns(ctx context.Context, _ *mcp.CallToolRequest, in listTxnsInput) (*mcp.CallToolResult, textOutput, error) {
	period, err := s.resolvePeriod(in.Period)
	if err != nil {
		return nil, textOutput{}, err
	}
	if in.Limit < 0 || in.Offset < 0 {
		return nil, textOutput{}, errors.New("limit and offset must be >= 0")
	}
	limit := in.Limit
	if limit == 0 {
		limit = 100
	}
	sort := in.Sort
	if sort == "" {
		sort = "date"
	}

	categoryRepo := postgres.NewCategoryRepo(s.db)
	var categoryID *int64
	if in.Category != "" {
		category, err := categoryRepo.GetByName(ctx, userID, in.Category)
		if err != nil {
			return nil, textOutput{}, fmt.Errorf("get category %q: %w", in.Category, err)
		}
		categoryID = &category.ID
	}

	var direction *domain.Direction
	if in.Direction != "" {
		d, err := domain.ParseDirection(in.Direction)
		if err != nil {
			return nil, textOutput{}, err
		}
		direction = &d
	}
	min, err := optionalMoney(in.Min)
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("min: %w", err)
	}
	max, err := optionalMoney(in.Max)
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("max: %w", err)
	}

	rows, err := report.Transactions(ctx, userID, report.TransactionsDeps{Txns: postgres.NewTxnRepo(s.db)}, report.TxnFilter{
		Period:     period,
		CategoryID: categoryID,
		Merchant:   in.Merchant,
		AccountID:  in.Account,
		Direction:  direction,
		Min:        min,
		Max:        max,
		Sort:       sort,
		Limit:      limit,
		Offset:     in.Offset,
	})
	if err != nil {
		return nil, textOutput{}, err
	}
	return renderJSON(func(w *bytes.Buffer) error {
		return render.RenderTransactions(w, render.FormatJSON, rows)
	})
}

type emptyInput struct{}

func (s *server) listUncategorised(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, textOutput, error) {
	categoryRepo := postgres.NewCategoryRepo(s.db)
	cat, err := categoryRepo.GetByName(ctx, userID, "Uncategorised")
	if err != nil {
		return nil, textOutput{}, fmt.Errorf("get Uncategorised category: %w", err)
	}
	assignments, err := postgres.NewAssignmentRepo(s.db).ListByCategory(ctx, userID, cat.ID)
	if err != nil {
		return nil, textOutput{}, err
	}
	ids := make([]string, 0, len(assignments))
	for _, a := range assignments {
		ids = append(ids, a.TxnID)
	}
	return jsonResult(map[string]any{"count": len(ids), "txn_ids": ids})
}

func (s *server) listCategories(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, textOutput, error) {
	cats, err := postgres.NewCategoryRepo(s.db).List(ctx, userID)
	if err != nil {
		return nil, textOutput{}, err
	}
	type catRow struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Parent *int64 `json:"parent_id,omitempty"`
	}
	rows := make([]catRow, 0, len(cats))
	for _, c := range cats {
		rows = append(rows, catRow{ID: c.ID, Name: c.Name, Kind: string(c.Kind), Parent: c.ParentID})
	}
	return jsonResult(rows)
}

func (s *server) resolvePeriod(value string) (domain.Range, error) {
	if value == "" {
		value = "this-month"
	}
	period, err := domain.ParsePeriod(value)
	if err != nil {
		return domain.Range{}, err
	}
	return period.Resolve(s.loc, time.Now())
}

func optionalMoney(value string) (*domain.Money, error) {
	if value == "" {
		return nil, nil
	}
	m, err := domain.NewMoneyFromString(value)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func renderJSON(fn func(*bytes.Buffer) error) (*mcp.CallToolResult, textOutput, error) {
	var buf bytes.Buffer
	if err := fn(&buf); err != nil {
		return nil, textOutput{}, err
	}
	body := buf.String()
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: body}},
	}, textOutput{JSON: body}, nil
}

func jsonResult(v any) (*mcp.CallToolResult, textOutput, error) {
	return renderJSON(func(buf *bytes.Buffer) error {
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	})
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
