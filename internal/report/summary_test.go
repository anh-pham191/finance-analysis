package report

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestSummaryAggregatesTransactionsInPeriodByCategory(t *testing.T) {
	t.Parallel()

	userID := domain.UserID(42)
	period := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("income", period.From, "5000.00", domain.DirectionCredit),
			summaryTxn("groceries", period.From.AddDate(0, 0, 1), "120.50", domain.DirectionDebit),
			summaryTxn("rent", period.To.Add(-time.Nanosecond), "600.00", domain.DirectionDebit),
			summaryTxn("missing-assignment", period.From.AddDate(0, 0, 2), "25.25", domain.DirectionDebit),
			summaryTxn("before", period.From.Add(-time.Nanosecond), "999.00", domain.DirectionDebit),
			summaryTxn("at-to", period.To, "999.00", domain.DirectionCredit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 10, Name: "Salary", Kind: domain.CategoryKindIncome},
			{ID: 20, Name: "Rent", Kind: domain.CategoryKindExpense},
			{ID: 30, Name: "Groceries", Kind: domain.CategoryKindExpense},
			{ID: 99, Name: "Uncategorised", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"income":    {TxnID: "income", CategoryID: 10},
			"groceries": {TxnID: "groceries", CategoryID: 30},
			"rent":      {TxnID: "rent", CategoryID: 20},
			"before":    {TxnID: "before", CategoryID: 20},
			"at-to":     {TxnID: "at-to", CategoryID: 10},
		}},
	}

	got, err := Summary(context.Background(), userID, deps, period)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}

	if got.Period != period {
		t.Fatalf("period = %#v, want %#v", got.Period, period)
	}
	if got.Income != MoneyAmount("5000.00") {
		t.Fatalf("income = %q, want 5000.00", got.Income)
	}
	if got.Expense != MoneyAmount("745.75") {
		t.Fatalf("expense = %q, want 745.75", got.Expense)
	}
	if got.Net != MoneyAmount("4254.25") {
		t.Fatalf("net = %q, want 4254.25", got.Net)
	}
	if !got.HasUncategorised {
		t.Fatal("HasUncategorised = false, want true")
	}

	wantCategories := []CategoryTotal{
		{CategoryID: 30, Category: "Groceries", Kind: "expense", Total: "120.50"},
		{CategoryID: 20, Category: "Rent", Kind: "expense", Total: "600.00"},
		{CategoryID: 10, Category: "Salary", Kind: "income", Total: "5000.00"},
		{CategoryID: 99, Category: "Uncategorised", Kind: "expense", Total: "25.25"},
	}
	if !slices.Equal(got.Categories, wantCategories) {
		t.Fatalf("categories = %#v, want %#v", got.Categories, wantCategories)
	}
}

func TestSummaryIncludesUncategorisedLineWhenAssignedTxnTotalsZero(t *testing.T) {
	t.Parallel()

	period := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("zero", period.From, "0.00", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 99, Name: "Uncategorised", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"zero": {TxnID: "zero", CategoryID: 99},
		}},
	}

	got, err := Summary(context.Background(), domain.UserID(42), deps, period)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}

	if !got.HasUncategorised {
		t.Fatal("HasUncategorised = false, want true")
	}
	wantCategories := []CategoryTotal{
		{CategoryID: 99, Category: "Uncategorised", Kind: "expense", Total: "0.00"},
	}
	if !slices.Equal(got.Categories, wantCategories) {
		t.Fatalf("categories = %#v, want %#v", got.Categories, wantCategories)
	}
}

func TestSummaryReportsNegativeDebitOutflowsAsPositiveExpenseTotals(t *testing.T) {
	t.Parallel()

	period := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("groceries", period.From, "-12.30", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 30, Name: "Groceries", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"groceries": {TxnID: "groceries", CategoryID: 30},
		}},
	}

	got, err := Summary(context.Background(), domain.UserID(42), deps, period)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}

	if got.Expense != MoneyAmount("12.30") {
		t.Fatalf("expense = %q, want 12.30", got.Expense)
	}
	if got.Net != MoneyAmount("-12.30") {
		t.Fatalf("net = %q, want -12.30", got.Net)
	}
	wantCategories := []CategoryTotal{
		{CategoryID: 30, Category: "Groceries", Kind: "expense", Total: "12.30"},
	}
	if !slices.Equal(got.Categories, wantCategories) {
		t.Fatalf("categories = %#v, want %#v", got.Categories, wantCategories)
	}
}

func TestSummaryDTOJSONTagsUseSnakeCase(t *testing.T) {
	t.Parallel()

	result := SummaryResult{
		Categories: []CategoryTotal{
			{CategoryID: 99, Category: "Uncategorised", Kind: "expense", Total: "1.00"},
		},
		HasUncategorised: true,
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(payload)

	for _, want := range []string{`"has_uncategorised":true`, `"category_id":99`} {
		if !strings.Contains(body, want) {
			t.Fatalf("JSON %s does not contain %s", body, want)
		}
	}
	for _, notWant := range []string{"HasUncategorised", "CategoryID"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("JSON %s contains non-snake-case key %s", body, notWant)
		}
	}
}

func TestSummaryValidatesDependencies(t *testing.T) {
	t.Parallel()

	_, err := Summary(context.Background(), domain.UserID(42), SummaryDeps{}, domain.Range{})
	if err == nil {
		t.Fatal("Summary returned nil error, want dependency validation error")
	}
	if strings.Contains(err.Error(), "Synthetic") {
		t.Fatalf("error %q includes transaction data", err)
	}
}

type fakeTxnRepo struct {
	txns []domain.Transaction
	err  error
}

func (r fakeTxnRepo) Upsert(context.Context, domain.UserID, domain.Transaction) error {
	panic("not implemented")
}

func (r fakeTxnRepo) Get(context.Context, domain.UserID, string) (domain.Transaction, error) {
	panic("not implemented")
}

func (r fakeTxnRepo) List(context.Context, domain.UserID) ([]domain.Transaction, error) {
	if r.err != nil {
		return nil, r.err
	}
	return slices.Clone(r.txns), nil
}

type fakeCategoryRepo struct {
	categories []domain.Category
	err        error
}

func (r fakeCategoryRepo) Insert(context.Context, domain.UserID, domain.Category) (domain.Category, error) {
	panic("not implemented")
}

func (r fakeCategoryRepo) GetByName(context.Context, domain.UserID, string) (domain.Category, error) {
	panic("not implemented")
}

func (r fakeCategoryRepo) Upsert(context.Context, domain.UserID, domain.Category) (domain.Category, error) {
	panic("not implemented")
}

func (r fakeCategoryRepo) List(context.Context, domain.UserID) ([]domain.Category, error) {
	if r.err != nil {
		return nil, r.err
	}
	return slices.Clone(r.categories), nil
}

type fakeAssignmentRepo struct {
	assignments map[string]domain.CategoryAssignment
	err         error
}

func (r fakeAssignmentRepo) Upsert(context.Context, domain.UserID, domain.CategoryAssignment) error {
	panic("not implemented")
}

func (r fakeAssignmentRepo) Get(_ context.Context, _ domain.UserID, txnID string) (domain.CategoryAssignment, error) {
	if r.err != nil {
		return domain.CategoryAssignment{}, r.err
	}
	assignment, ok := r.assignments[txnID]
	if !ok {
		return domain.CategoryAssignment{}, ports.ErrNotFound
	}
	return assignment, nil
}

func (r fakeAssignmentRepo) UpsertIfChanged(context.Context, domain.UserID, domain.CategoryAssignment) (bool, error) {
	panic("not implemented")
}

func (r fakeAssignmentRepo) Delete(context.Context, domain.UserID, string) error {
	panic("not implemented")
}

func (r fakeAssignmentRepo) ListByCategory(context.Context, domain.UserID, int64) ([]domain.CategoryAssignment, error) {
	panic("not implemented")
}

func summaryTxn(id string, postedAt time.Time, amount string, direction domain.Direction) domain.Transaction {
	return domain.Transaction{
		ID:          id,
		PostedAt:    postedAt,
		Amount:      domain.MustMoneyFromString(amount),
		Direction:   direction,
		Description: "Synthetic description",
		Merchant:    "Synthetic merchant",
	}
}

var _ ports.TxnRepo = fakeTxnRepo{}
var _ ports.CategoryRepo = fakeCategoryRepo{}
var _ ports.AssignmentRepo = fakeAssignmentRepo{}
