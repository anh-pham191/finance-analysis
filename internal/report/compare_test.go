package report

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestCompareAggregatesCategoryTotalsAndDeltas(t *testing.T) {
	t.Parallel()

	a := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	b := domain.Range{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("a-salary", a.From, "1000.00", domain.DirectionCredit),
			summaryTxn("b-salary", b.From, "900.00", domain.DirectionCredit),
			summaryTxn("a-groceries", a.From, "100.00", domain.DirectionDebit),
			summaryTxn("b-groceries", b.From, "160.00", domain.DirectionDebit),
			summaryTxn("b-rent", b.From, "50.00", domain.DirectionDebit),
			summaryTxn("a-transport", a.From, "40.00", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 10, Name: "Salary", Kind: domain.CategoryKindIncome},
			{ID: 20, Name: "Rent", Kind: domain.CategoryKindExpense},
			{ID: 30, Name: "Groceries", Kind: domain.CategoryKindExpense},
			{ID: 40, Name: "Transport", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"a-salary":    {TxnID: "a-salary", CategoryID: 10},
			"b-salary":    {TxnID: "b-salary", CategoryID: 10},
			"a-groceries": {TxnID: "a-groceries", CategoryID: 30},
			"b-groceries": {TxnID: "b-groceries", CategoryID: 30},
			"b-rent":      {TxnID: "b-rent", CategoryID: 20},
			"a-transport": {TxnID: "a-transport", CategoryID: 40},
		}},
	}

	got, err := Compare(context.Background(), domain.UserID(42), deps, a, b, CompareOptions{})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}

	if got.A != a {
		t.Fatalf("A = %#v, want %#v", got.A, a)
	}
	if got.B != b {
		t.Fatalf("B = %#v, want %#v", got.B, b)
	}
	want := []CompareCategory{
		{CategoryID: 10, Category: "Salary", Kind: "income", A: "1000.00", B: "900.00", Delta: "-100.00", DeltaPercent: floatPtr(-10)},
		{CategoryID: 30, Category: "Groceries", Kind: "expense", A: "100.00", B: "160.00", Delta: "60.00", DeltaPercent: floatPtr(60)},
		{CategoryID: 20, Category: "Rent", Kind: "expense", A: "0.00", B: "50.00", Delta: "50.00", DeltaPercent: nil},
		{CategoryID: 40, Category: "Transport", Kind: "expense", A: "40.00", B: "0.00", Delta: "-40.00", DeltaPercent: floatPtr(-100)},
	}
	assertCompareCategories(t, got.Categories, want)
}

func TestCompareSortsByAbsoluteDeltaThenCategoryName(t *testing.T) {
	t.Parallel()

	a := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	b := domain.Range{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("a-zebra", a.From, "50.00", domain.DirectionDebit),
			summaryTxn("b-zebra", b.From, "30.00", domain.DirectionDebit),
			summaryTxn("a-alpha", a.From, "10.00", domain.DirectionDebit),
			summaryTxn("b-alpha", b.From, "30.00", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 1, Name: "Zebra", Kind: domain.CategoryKindExpense},
			{ID: 2, Name: "Alpha", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"a-zebra": {TxnID: "a-zebra", CategoryID: 1},
			"b-zebra": {TxnID: "b-zebra", CategoryID: 1},
			"a-alpha": {TxnID: "a-alpha", CategoryID: 2},
			"b-alpha": {TxnID: "b-alpha", CategoryID: 2},
		}},
	}

	got, err := Compare(context.Background(), domain.UserID(42), deps, a, b, CompareOptions{})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}

	gotNames := []string{got.Categories[0].Category, got.Categories[1].Category}
	wantNames := []string{"Alpha", "Zebra"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("category order = %#v, want %#v", gotNames, wantNames)
	}
}

func TestCompareTopTruncatesResults(t *testing.T) {
	t.Parallel()

	a := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	b := domain.Range{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("b-alpha", b.From, "30.00", domain.DirectionDebit),
			summaryTxn("b-bravo", b.From, "20.00", domain.DirectionDebit),
			summaryTxn("b-charlie", b.From, "10.00", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 1, Name: "Alpha", Kind: domain.CategoryKindExpense},
			{ID: 2, Name: "Bravo", Kind: domain.CategoryKindExpense},
			{ID: 3, Name: "Charlie", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"b-alpha":   {TxnID: "b-alpha", CategoryID: 1},
			"b-bravo":   {TxnID: "b-bravo", CategoryID: 2},
			"b-charlie": {TxnID: "b-charlie", CategoryID: 3},
		}},
	}

	got, err := Compare(context.Background(), domain.UserID(42), deps, a, b, CompareOptions{Top: 2})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}

	if len(got.Categories) != 2 {
		t.Fatalf("len(categories) = %d, want 2", len(got.Categories))
	}
	gotNames := []string{got.Categories[0].Category, got.Categories[1].Category}
	wantNames := []string{"Alpha", "Bravo"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("category order = %#v, want %#v", gotNames, wantNames)
	}
}

func TestCompareReportsNegativeDebitOutflowsAsPositiveExpenseTotals(t *testing.T) {
	t.Parallel()

	a := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	b := domain.Range{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	deps := SummaryDeps{
		Txns: fakeTxnRepo{txns: []domain.Transaction{
			summaryTxn("a-groceries", a.From, "-12.30", domain.DirectionDebit),
			summaryTxn("b-groceries", b.From, "-20.00", domain.DirectionDebit),
		}},
		Categories: fakeCategoryRepo{categories: []domain.Category{
			{ID: 30, Name: "Groceries", Kind: domain.CategoryKindExpense},
		}},
		Assignments: fakeAssignmentRepo{assignments: map[string]domain.CategoryAssignment{
			"a-groceries": {TxnID: "a-groceries", CategoryID: 30},
			"b-groceries": {TxnID: "b-groceries", CategoryID: 30},
		}},
	}

	got, err := Compare(context.Background(), domain.UserID(42), deps, a, b, CompareOptions{})
	if err != nil {
		t.Fatalf("Compare returned error: %v", err)
	}

	want := []CompareCategory{
		{CategoryID: 30, Category: "Groceries", Kind: "expense", A: "12.30", B: "20.00", Delta: "7.70", DeltaPercent: floatPtr(62.60162601626016)},
	}
	assertCompareCategories(t, got.Categories, want)
}

func TestCompareDTOJSONTagsUseSnakeCase(t *testing.T) {
	t.Parallel()

	percent := 12.5
	result := CompareResult{
		Categories: []CompareCategory{
			{CategoryID: 99, Category: "Uncategorised", Kind: "expense", A: "1.00", B: "2.00", Delta: "1.00", DeltaPercent: &percent},
		},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(payload)

	for _, want := range []string{`"delta_percent":12.5`, `"category_id":99`} {
		if !strings.Contains(body, want) {
			t.Fatalf("JSON %s does not contain %s", body, want)
		}
	}
	for _, notWant := range []string{"DeltaPercent", "CategoryID"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("JSON %s contains non-snake-case key %s", body, notWant)
		}
	}
}

func assertCompareCategories(t *testing.T, got, want []CompareCategory) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("categories = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].CategoryID != want[i].CategoryID ||
			got[i].Category != want[i].Category ||
			got[i].Kind != want[i].Kind ||
			got[i].A != want[i].A ||
			got[i].B != want[i].B ||
			got[i].Delta != want[i].Delta {
			t.Fatalf("categories[%d] = %#v, want %#v", i, got[i], want[i])
		}
		if got[i].DeltaPercent == nil || want[i].DeltaPercent == nil {
			if got[i].DeltaPercent != want[i].DeltaPercent {
				t.Fatalf("categories[%d].DeltaPercent = %#v, want %#v", i, got[i].DeltaPercent, want[i].DeltaPercent)
			}
			continue
		}
		if math.Abs(*got[i].DeltaPercent-*want[i].DeltaPercent) > 0.0000001 {
			t.Fatalf("categories[%d].DeltaPercent = %v, want %v", i, *got[i].DeltaPercent, *want[i].DeltaPercent)
		}
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
