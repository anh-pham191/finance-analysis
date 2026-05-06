package categorise

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestCategoriseFirstRunCreatesRowsAndConverges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(1)
	deps := newCategoriseTestDeps([]domain.Transaction{
		testTxn("txn-groceries", "Synthetic Market", domain.DirectionDebit),
		testTxn("txn-unknown", "Synthetic Mystery", domain.DirectionDebit),
	})
	cfg := testCategoriseConfig([]Rule{
		{
			Name:     "Groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Market"},
			},
			Category: "Food/Groceries",
		},
	})

	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise returned error: %v", err)
	}

	groceries := deps.categories.mustGet(t, "Food/Groceries")
	uncategorised := deps.categories.mustGet(t, "Uncategorised")
	rule := deps.rules.mustGet(t, "Groceries")
	if rule.CategoryID != groceries.ID {
		t.Fatalf("rule category ID = %d, want %d", rule.CategoryID, groceries.ID)
	}
	if !strings.Contains(string(rule.Predicate), `"merchant_in":["Synthetic Market"]`) {
		t.Fatalf("rule predicate JSON = %s, want merchant predicate with config field name", rule.Predicate)
	}

	assertAssignment(t, deps.assignments.mustGet(t, "txn-groceries"), groceries.ID, domain.AssignmentSourceRule, &rule.ID)
	assertAssignment(t, deps.assignments.mustGet(t, "txn-unknown"), uncategorised.ID, domain.AssignmentSourceRule, nil)

	deps.assignments.resetCounts()
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("second Categorise returned error: %v", err)
	}
	if deps.assignments.changed != 0 {
		t.Fatalf("second run changed %d assignments, want 0", deps.assignments.changed)
	}
	if deps.assignments.calls != 0 {
		t.Fatalf("second run made %d assignment write calls, want 0", deps.assignments.calls)
	}
}

func TestCategoriseRuleCategoryEditUpdatesNonManualAssignment(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(1)
	deps := newCategoriseTestDeps([]domain.Transaction{
		testTxn("txn-groceries", "Synthetic Market", domain.DirectionDebit),
	})
	cfg := testCategoriseConfig([]Rule{
		{
			Name:     "Groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Market"},
			},
			Category: "Food/Groceries",
		},
	})

	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise returned error: %v", err)
	}

	cfg.Rules[0].Category = "Food/Eating-out"
	deps.assignments.resetCounts()
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise after rule edit returned error: %v", err)
	}

	eatingOut := deps.categories.mustGet(t, "Food/Eating-out")
	rule := deps.rules.mustGet(t, "Groceries")
	if rule.CategoryID != eatingOut.ID {
		t.Fatalf("rule category ID = %d, want %d", rule.CategoryID, eatingOut.ID)
	}
	assertAssignment(t, deps.assignments.mustGet(t, "txn-groceries"), eatingOut.ID, domain.AssignmentSourceRule, &rule.ID)
	if deps.assignments.changed != 1 {
		t.Fatalf("rule edit changed %d assignments, want 1", deps.assignments.changed)
	}
}

func TestCategoriseRemovedRuleDeletesMissingAndFallsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(1)
	deps := newCategoriseTestDeps([]domain.Transaction{
		testTxn("txn-groceries", "Synthetic Market", domain.DirectionDebit),
		testTxn("txn-legacy", "Synthetic Legacy", domain.DirectionDebit),
	})
	cfg := testCategoriseConfig([]Rule{
		{
			Name:     "Specific groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Market"},
			},
			Category: "Food/Groceries",
		},
		{
			Name:     "Legacy merchant",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Legacy"},
			},
			Category: "Food/Groceries",
		},
		{
			Name:     "Any debit",
			Priority: 20,
			Predicate: Predicate{
				Direction: string(domain.DirectionDebit),
			},
			Category: "Food",
		},
	})

	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise returned error: %v", err)
	}

	cfg.Rules = []Rule{cfg.Rules[2]}
	deps.assignments.resetCounts()
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise after rule removal returned error: %v", err)
	}

	if !reflect.DeepEqual(deps.rules.lastKeepNames, []string{"Any debit"}) {
		t.Fatalf("DeleteMissing keep names = %v, want [Any debit]", deps.rules.lastKeepNames)
	}
	if _, ok := deps.rules.byName["Specific groceries"]; ok {
		t.Fatal("removed rule still exists")
	}
	food := deps.categories.mustGet(t, "Food")
	anyDebit := deps.rules.mustGet(t, "Any debit")
	assertAssignment(t, deps.assignments.mustGet(t, "txn-groceries"), food.ID, domain.AssignmentSourceRule, &anyDebit.ID)
	assertAssignment(t, deps.assignments.mustGet(t, "txn-legacy"), food.ID, domain.AssignmentSourceRule, &anyDebit.ID)
}

func TestCategoriseRemovedRuleFallsToUncategorised(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(1)
	deps := newCategoriseTestDeps([]domain.Transaction{
		testTxn("txn-legacy", "Synthetic Legacy", domain.DirectionDebit),
	})
	cfg := testCategoriseConfig([]Rule{
		{
			Name:     "Legacy merchant",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Legacy"},
			},
			Category: "Food/Groceries",
		},
	})

	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise returned error: %v", err)
	}

	cfg.Rules = nil
	deps.assignments.resetCounts()
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise after rule removal returned error: %v", err)
	}

	if len(deps.rules.byName) != 0 {
		t.Fatalf("rules after DeleteMissing = %v, want none", deps.rules.byName)
	}
	uncategorised := deps.categories.mustGet(t, "Uncategorised")
	assertAssignment(t, deps.assignments.mustGet(t, "txn-legacy"), uncategorised.ID, domain.AssignmentSourceRule, nil)
}

func TestCategoriseManualAssignmentSurvivesRuleEditAndDeletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	userID := domain.UserID(1)
	deps := newCategoriseTestDeps([]domain.Transaction{
		testTxn("txn-groceries", "Synthetic Market", domain.DirectionDebit),
	})
	cfg := testCategoriseConfig([]Rule{
		{
			Name:     "Groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Market"},
			},
			Category: "Food/Groceries",
		},
	})

	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise returned error: %v", err)
	}
	manualCategory := deps.categories.mustGet(t, "Food")
	deps.assignments.byTxn["txn-groceries"] = domain.CategoryAssignment{
		TxnID:      "txn-groceries",
		CategoryID: manualCategory.ID,
		Source:     domain.AssignmentSourceManual,
		AssignedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	deps.assignments.resetCounts()

	cfg.Rules[0].Category = "Food/Eating-out"
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise after rule edit returned error: %v", err)
	}
	cfg.Rules = nil
	if err := Categorise(ctx, userID, deps.asDeps(), cfg); err != nil {
		t.Fatalf("Categorise after rule deletion returned error: %v", err)
	}

	assignment := deps.assignments.mustGet(t, "txn-groceries")
	assertAssignment(t, assignment, manualCategory.ID, domain.AssignmentSourceManual, nil)
	if !assignment.AssignedAt.Equal(time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("manual assignment time = %s, want unchanged", assignment.AssignedAt)
	}
	if deps.assignments.calls != 0 {
		t.Fatalf("manual assignment caused %d assignment writes, want 0", deps.assignments.calls)
	}
}

func TestCategoriseErrorsWhenUncategorisedMissing(t *testing.T) {
	t.Parallel()

	deps := newCategoriseTestDeps([]domain.Transaction{})
	cfg := Config{
		Categories: []Category{{Name: "Food", Kind: KindExpense}},
	}

	err := Categorise(context.Background(), domain.UserID(1), deps.asDeps(), cfg)
	assertCategoriseErrorContains(t, err, "Uncategorised")
}

func TestCategoriseValidatesDependencies(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		deps Deps
		want string
	}{
		"categories":  {deps: Deps{}, want: "categories"},
		"rules":       {deps: Deps{Categories: &fakeCategoryRepo{}}, want: "rules"},
		"assignments": {deps: Deps{Categories: &fakeCategoryRepo{}, Rules: &fakeRuleRepo{}}, want: "assignments"},
		"transactions": {
			deps: Deps{Categories: &fakeCategoryRepo{}, Rules: &fakeRuleRepo{}, Assignments: &fakeAssignmentRepo{}},
			want: "transactions",
		},
		"clock": {
			deps: Deps{
				Categories:  &fakeCategoryRepo{},
				Rules:       &fakeRuleRepo{},
				Assignments: &fakeAssignmentRepo{},
				Txns:        &fakeTxnRepo{},
			},
			want: "clock",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Categorise(context.Background(), domain.UserID(1), test.deps, Config{})
			assertCategoriseErrorContains(t, err, test.want)
		})
	}
}

type categoriseTestDeps struct {
	categories  *fakeCategoryRepo
	rules       *fakeRuleRepo
	assignments *fakeAssignmentRepo
	txns        *fakeTxnRepo
	clock       fakeClock
}

func newCategoriseTestDeps(txns []domain.Transaction) categoriseTestDeps {
	return categoriseTestDeps{
		categories:  newFakeCategoryRepo(),
		rules:       newFakeRuleRepo(),
		assignments: newFakeAssignmentRepo(),
		txns:        &fakeTxnRepo{txns: txns},
		clock:       fakeClock{now: time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC)},
	}
}

func (d categoriseTestDeps) asDeps() Deps {
	return Deps{
		Categories:  d.categories,
		Rules:       d.rules,
		Assignments: d.assignments,
		Txns:        d.txns,
		Clock:       d.clock,
	}
}

func testCategoriseConfig(rules []Rule) Config {
	return Config{
		Categories: []Category{
			{Name: "Food", Kind: KindExpense},
			{Name: "Food/Groceries", Kind: KindExpense, Parent: "Food"},
			{Name: "Food/Eating-out", Kind: KindExpense, Parent: "Food"},
			{Name: "Uncategorised", Kind: KindExpense},
		},
		Rules: rules,
	}
}

func testTxn(id, merchant string, direction domain.Direction) domain.Transaction {
	return domain.Transaction{
		ID:        id,
		AccountID: "acct-synthetic",
		Amount:    domain.MustMoneyFromString("12.34"),
		Direction: direction,
		Merchant:  merchant,
	}
}

func assertAssignment(t *testing.T, got domain.CategoryAssignment, wantCategoryID int64, wantSource domain.AssignmentSource, wantRuleID *int64) {
	t.Helper()

	if got.CategoryID != wantCategoryID {
		t.Fatalf("assignment category ID = %d, want %d", got.CategoryID, wantCategoryID)
	}
	if got.Source != wantSource {
		t.Fatalf("assignment source = %q, want %q", got.Source, wantSource)
	}
	if !sameRuleID(got.RuleID, wantRuleID) {
		t.Fatalf("assignment rule ID = %v, want %v", got.RuleID, wantRuleID)
	}
}

func sameRuleID(got, want *int64) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func assertCategoriseErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), want)
	}
}

type fakeCategoryRepo struct {
	nextID int64
	byName map[string]domain.Category
}

func newFakeCategoryRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{nextID: 1, byName: map[string]domain.Category{}}
}

func (r *fakeCategoryRepo) Insert(ctx context.Context, userID domain.UserID, category domain.Category) (domain.Category, error) {
	return r.Upsert(ctx, userID, category)
}

func (r *fakeCategoryRepo) GetByName(_ context.Context, _ domain.UserID, name string) (domain.Category, error) {
	category, ok := r.byName[name]
	if !ok {
		return domain.Category{}, ports.ErrNotFound
	}
	return category, nil
}

func (r *fakeCategoryRepo) Upsert(_ context.Context, _ domain.UserID, category domain.Category) (domain.Category, error) {
	if existing, ok := r.byName[category.Name]; ok {
		category.ID = existing.ID
	} else {
		category.ID = r.nextID
		r.nextID++
	}
	r.byName[category.Name] = category
	return category, nil
}

func (r *fakeCategoryRepo) List(_ context.Context, _ domain.UserID) ([]domain.Category, error) {
	categories := make([]domain.Category, 0, len(r.byName))
	for _, category := range r.byName {
		categories = append(categories, category)
	}
	return categories, nil
}

func (r *fakeCategoryRepo) mustGet(t *testing.T, name string) domain.Category {
	t.Helper()

	category, ok := r.byName[name]
	if !ok {
		t.Fatalf("missing category %q", name)
	}
	return category
}

type fakeRuleRepo struct {
	nextID        int64
	byName        map[string]domain.Rule
	lastKeepNames []string
}

func newFakeRuleRepo() *fakeRuleRepo {
	return &fakeRuleRepo{nextID: 1, byName: map[string]domain.Rule{}}
}

func (r *fakeRuleRepo) Insert(ctx context.Context, userID domain.UserID, rule domain.Rule) (domain.Rule, error) {
	return r.Upsert(ctx, userID, rule)
}

func (r *fakeRuleRepo) GetByName(_ context.Context, _ domain.UserID, name string) (domain.Rule, error) {
	rule, ok := r.byName[name]
	if !ok {
		return domain.Rule{}, ports.ErrNotFound
	}
	return rule, nil
}

func (r *fakeRuleRepo) Upsert(_ context.Context, _ domain.UserID, rule domain.Rule) (domain.Rule, error) {
	if existing, ok := r.byName[rule.Name]; ok {
		rule.ID = existing.ID
		rule.CreatedAt = existing.CreatedAt
	} else {
		rule.ID = r.nextID
		r.nextID++
	}
	r.byName[rule.Name] = rule
	return rule, nil
}

func (r *fakeRuleRepo) List(_ context.Context, _ domain.UserID) ([]domain.Rule, error) {
	rules := make([]domain.Rule, 0, len(r.byName))
	for _, rule := range r.byName {
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *fakeRuleRepo) DeleteMissing(_ context.Context, _ domain.UserID, keepNames []string) error {
	r.lastKeepNames = append([]string(nil), keepNames...)
	slices.Sort(r.lastKeepNames)
	keep := map[string]struct{}{}
	for _, name := range keepNames {
		keep[name] = struct{}{}
	}
	for name := range r.byName {
		if _, ok := keep[name]; !ok {
			delete(r.byName, name)
		}
	}
	return nil
}

func (r *fakeRuleRepo) mustGet(t *testing.T, name string) domain.Rule {
	t.Helper()

	rule, ok := r.byName[name]
	if !ok {
		t.Fatalf("missing rule %q", name)
	}
	return rule
}

type fakeAssignmentRepo struct {
	byTxn   map[string]domain.CategoryAssignment
	calls   int
	changed int
}

func newFakeAssignmentRepo() *fakeAssignmentRepo {
	return &fakeAssignmentRepo{byTxn: map[string]domain.CategoryAssignment{}}
}

func (r *fakeAssignmentRepo) Upsert(_ context.Context, _ domain.UserID, assignment domain.CategoryAssignment) error {
	r.byTxn[assignment.TxnID] = assignment
	return nil
}

func (r *fakeAssignmentRepo) Get(_ context.Context, _ domain.UserID, txnID string) (domain.CategoryAssignment, error) {
	assignment, ok := r.byTxn[txnID]
	if !ok {
		return domain.CategoryAssignment{}, ports.ErrNotFound
	}
	return assignment, nil
}

func (r *fakeAssignmentRepo) UpsertIfChanged(_ context.Context, _ domain.UserID, assignment domain.CategoryAssignment) (bool, error) {
	r.calls++
	existing, ok := r.byTxn[assignment.TxnID]
	if ok && existing.CategoryID == assignment.CategoryID && sameRuleID(existing.RuleID, assignment.RuleID) && existing.Source == assignment.Source {
		return false, nil
	}
	r.byTxn[assignment.TxnID] = assignment
	r.changed++
	return true, nil
}

func (r *fakeAssignmentRepo) Delete(_ context.Context, _ domain.UserID, txnID string) error {
	delete(r.byTxn, txnID)
	return nil
}

func (r *fakeAssignmentRepo) ListByCategory(_ context.Context, _ domain.UserID, categoryID int64) ([]domain.CategoryAssignment, error) {
	assignments := []domain.CategoryAssignment{}
	for _, assignment := range r.byTxn {
		if assignment.CategoryID == categoryID {
			assignments = append(assignments, assignment)
		}
	}
	return assignments, nil
}

func (r *fakeAssignmentRepo) resetCounts() {
	r.calls = 0
	r.changed = 0
}

func (r *fakeAssignmentRepo) mustGet(t *testing.T, txnID string) domain.CategoryAssignment {
	t.Helper()

	assignment, ok := r.byTxn[txnID]
	if !ok {
		t.Fatalf("missing assignment for %q", txnID)
	}
	return assignment
}

type fakeTxnRepo struct {
	txns []domain.Transaction
}

func (r *fakeTxnRepo) Upsert(_ context.Context, _ domain.UserID, _ domain.Transaction) error {
	return nil
}

func (r *fakeTxnRepo) Get(_ context.Context, _ domain.UserID, id string) (domain.Transaction, error) {
	for _, txn := range r.txns {
		if txn.ID == id {
			return txn, nil
		}
	}
	return domain.Transaction{}, ports.ErrNotFound
}

func (r *fakeTxnRepo) List(_ context.Context, _ domain.UserID) ([]domain.Transaction, error) {
	return append([]domain.Transaction(nil), r.txns...), nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}
