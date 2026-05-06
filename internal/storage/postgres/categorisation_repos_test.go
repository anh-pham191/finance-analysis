//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestCategoryRepoUpsertListAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "category-task5-user1@example.test")
	userTwo := seedUser(t, db.owner, "category-task5-user2@example.test")
	repo := NewCategoryRepo(db.app)

	parent, err := repo.Upsert(context.Background(), userOne, domain.Category{Name: "Parent", Kind: domain.CategoryKindExpense})
	if err != nil {
		t.Fatalf("upsert parent category: %v", err)
	}
	category, err := repo.Upsert(context.Background(), userOne, domain.Category{Name: "Transport", Kind: domain.CategoryKindExpense})
	if err != nil {
		t.Fatalf("upsert category: %v", err)
	}
	updated, err := repo.Upsert(context.Background(), userOne, domain.Category{
		Name:     category.Name,
		ParentID: &parent.ID,
		Kind:     domain.CategoryKindTransfer,
	})
	if err != nil {
		t.Fatalf("upsert updated category: %v", err)
	}
	if updated.ID != category.ID {
		t.Fatalf("updated category ID = %d, want %d", updated.ID, category.ID)
	}
	if updated.Kind != domain.CategoryKindTransfer || updated.ParentID == nil || *updated.ParentID != parent.ID {
		t.Fatalf("updated category = %+v, want transfer child of %d", updated, parent.ID)
	}

	if _, err := repo.Upsert(context.Background(), userTwo, domain.Category{Name: "OtherTenant", Kind: domain.CategoryKindIncome}); err != nil {
		t.Fatalf("upsert other tenant category: %v", err)
	}
	categories, err := repo.List(context.Background(), userOne)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	assertCategoryNames(t, categories, []string{"Parent", "Transport"})

	got, err := repo.GetByName(context.Background(), userOne, "Transport")
	if err != nil {
		t.Fatalf("get category by name: %v", err)
	}
	if got.ID != category.ID {
		t.Fatalf("category ID = %d, want %d", got.ID, category.ID)
	}
	_, err = repo.GetByName(context.Background(), userOne, "Missing")
	assertPortNotFound(t, err)
	_, err = repo.GetByName(context.Background(), userTwo, "Transport")
	assertPortNotFound(t, err)
}

func TestRuleRepoUpsertListDeleteMissingAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "rule-task5-user1@example.test")
	userTwo := seedUser(t, db.owner, "rule-task5-user2@example.test")
	categoryRepo := NewCategoryRepo(db.app)
	ruleRepo := NewRuleRepo(db.app)

	categoryOne := mustUpsertCategory(t, categoryRepo, userOne, "Food", domain.CategoryKindExpense)
	categoryTwo := mustUpsertCategory(t, categoryRepo, userOne, "Income", domain.CategoryKindIncome)
	otherCategory := mustUpsertCategory(t, categoryRepo, userTwo, "Food", domain.CategoryKindExpense)

	rule, err := ruleRepo.Upsert(context.Background(), userOne, domain.Rule{
		Name:       "Groceries",
		Priority:   30,
		Predicate:  json.RawMessage(`{"merchant_in":["Old"]}`),
		CategoryID: categoryOne.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("upsert rule: %v", err)
	}
	updated, err := ruleRepo.Upsert(context.Background(), userOne, domain.Rule{
		Name:       rule.Name,
		Priority:   20,
		Predicate:  json.RawMessage(`{"merchant_in":["New"]}`),
		CategoryID: categoryTwo.ID,
		Enabled:    false,
	})
	if err != nil {
		t.Fatalf("upsert updated rule: %v", err)
	}
	if updated.ID != rule.ID {
		t.Fatalf("updated rule ID = %d, want %d", updated.ID, rule.ID)
	}
	if updated.Priority != 20 || updated.CategoryID != categoryTwo.ID || updated.Enabled {
		t.Fatalf("updated rule = %+v, want updated fields", updated)
	}
	assertJSONEqual(t, updated.Predicate, json.RawMessage(`{"merchant_in":["New"]}`))

	if _, err := ruleRepo.Upsert(context.Background(), userOne, domain.Rule{Name: "Beta", Priority: 10, Predicate: json.RawMessage(`{}`), CategoryID: categoryOne.ID, Enabled: true}); err != nil {
		t.Fatalf("upsert beta rule: %v", err)
	}
	if _, err := ruleRepo.Upsert(context.Background(), userOne, domain.Rule{Name: "Alpha", Priority: 10, Predicate: json.RawMessage(`{}`), CategoryID: categoryOne.ID, Enabled: true}); err != nil {
		t.Fatalf("upsert alpha rule: %v", err)
	}
	if _, err := ruleRepo.Upsert(context.Background(), userTwo, domain.Rule{Name: "OtherTenant", Priority: 1, Predicate: json.RawMessage(`{}`), CategoryID: otherCategory.ID, Enabled: true}); err != nil {
		t.Fatalf("upsert other tenant rule: %v", err)
	}

	rules, err := ruleRepo.List(context.Background(), userOne)
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	assertRuleNames(t, rules, []string{"Alpha", "Beta", "Groceries"})

	got, err := ruleRepo.GetByName(context.Background(), userOne, "Groceries")
	if err != nil {
		t.Fatalf("get rule by name: %v", err)
	}
	if got.ID != rule.ID {
		t.Fatalf("rule ID = %d, want %d", got.ID, rule.ID)
	}
	_, err = ruleRepo.GetByName(context.Background(), userOne, "Missing")
	assertPortNotFound(t, err)

	if err := ruleRepo.DeleteMissing(context.Background(), userOne, []string{"Beta"}); err != nil {
		t.Fatalf("delete missing rules: %v", err)
	}
	rules, err = ruleRepo.List(context.Background(), userOne)
	if err != nil {
		t.Fatalf("list rules after delete missing: %v", err)
	}
	assertRuleNames(t, rules, []string{"Beta"})
	otherRules, err := ruleRepo.List(context.Background(), userTwo)
	if err != nil {
		t.Fatalf("list other tenant rules: %v", err)
	}
	assertRuleNames(t, otherRules, []string{"OtherTenant"})

	if err := ruleRepo.DeleteMissing(context.Background(), userOne, nil); err != nil {
		t.Fatalf("delete all missing rules: %v", err)
	}
	rules, err = ruleRepo.List(context.Background(), userOne)
	if err != nil {
		t.Fatalf("list rules after delete all: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("rules after delete all = %v, want none", rules)
	}
	otherRules, err = ruleRepo.List(context.Background(), userTwo)
	if err != nil {
		t.Fatalf("list other tenant rules after delete all: %v", err)
	}
	assertRuleNames(t, otherRules, []string{"OtherTenant"})
}

func TestAssignmentRepoUpsertIfChangedDeleteListAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "assignment-task5-user1@example.test")
	userTwo := seedUser(t, db.owner, "assignment-task5-user2@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)
	categoryRepo := NewCategoryRepo(db.app)
	ruleRepo := NewRuleRepo(db.app)
	assignmentRepo := NewAssignmentRepo(db.app)

	mustUpsertAccount(t, accountRepo, userOne, "acc-assignment-task5")
	categoryOne := mustUpsertCategory(t, categoryRepo, userOne, "Uncategorised", domain.CategoryKindExpense)
	categoryTwo := mustUpsertCategory(t, categoryRepo, userOne, "Groceries", domain.CategoryKindExpense)
	rule := mustUpsertRule(t, ruleRepo, userOne, "Rule", categoryOne.ID)
	ruleID := rule.ID

	mustUpsertTxn(t, txnRepo, userOne, "txn-b", "acc-assignment-task5", time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC))
	assignment := domain.CategoryAssignment{
		TxnID:      "txn-b",
		CategoryID: categoryOne.ID,
		Source:     domain.AssignmentSourceRule,
		RuleID:     &ruleID,
	}
	changed, err := assignmentRepo.UpsertIfChanged(context.Background(), userOne, assignment)
	if err != nil {
		t.Fatalf("upsert assignment: %v", err)
	}
	if !changed {
		t.Fatal("insert changed = false, want true")
	}
	got, err := assignmentRepo.Get(context.Background(), userOne, assignment.TxnID)
	if err != nil {
		t.Fatalf("get inserted assignment: %v", err)
	}
	firstAssignedAt := got.AssignedAt

	changed, err = assignmentRepo.UpsertIfChanged(context.Background(), userOne, assignment)
	if err != nil {
		t.Fatalf("upsert unchanged assignment: %v", err)
	}
	if changed {
		t.Fatal("unchanged assignment changed = true, want false")
	}
	got, err = assignmentRepo.Get(context.Background(), userOne, assignment.TxnID)
	if err != nil {
		t.Fatalf("get unchanged assignment: %v", err)
	}
	if !got.AssignedAt.Equal(firstAssignedAt) {
		t.Fatalf("assigned_at changed from %s to %s for unchanged assignment", firstAssignedAt, got.AssignedAt)
	}

	changed, err = assignmentRepo.UpsertIfChanged(context.Background(), userOne, domain.CategoryAssignment{
		TxnID:      assignment.TxnID,
		CategoryID: categoryTwo.ID,
		Source:     domain.AssignmentSourceManual,
	})
	if err != nil {
		t.Fatalf("upsert changed assignment: %v", err)
	}
	if !changed {
		t.Fatal("changed assignment changed = false, want true")
	}
	got, err = assignmentRepo.Get(context.Background(), userOne, assignment.TxnID)
	if err != nil {
		t.Fatalf("get changed assignment: %v", err)
	}
	if !got.AssignedAt.After(firstAssignedAt) {
		t.Fatalf("assigned_at = %s, want after %s", got.AssignedAt, firstAssignedAt)
	}
	if got.CategoryID != categoryTwo.ID || got.Source != domain.AssignmentSourceManual || got.RuleID != nil {
		t.Fatalf("changed assignment = %+v, want manual category %d with nil rule", got, categoryTwo.ID)
	}

	if err := assignmentRepo.Delete(context.Background(), userOne, assignment.TxnID); err != nil {
		t.Fatalf("delete assignment: %v", err)
	}
	_, err = assignmentRepo.Get(context.Background(), userOne, assignment.TxnID)
	assertPortNotFound(t, err)

	mustUpsertTxn(t, txnRepo, userOne, "txn-c", "acc-assignment-task5", time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC))
	mustUpsertTxn(t, txnRepo, userOne, "txn-a", "acc-assignment-task5", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	mustUpsertTxn(t, txnRepo, userOne, "txn-d", "acc-assignment-task5", time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC))
	mustUpsertAssignment(t, assignmentRepo, userOne, "txn-c", categoryOne.ID, domain.AssignmentSourceRule, &ruleID)
	mustUpsertAssignment(t, assignmentRepo, userOne, "txn-a", categoryOne.ID, domain.AssignmentSourceRule, &ruleID)
	mustUpsertAssignment(t, assignmentRepo, userOne, "txn-d", categoryTwo.ID, domain.AssignmentSourceManual, nil)

	assignments, err := assignmentRepo.ListByCategory(context.Background(), userOne, categoryOne.ID)
	if err != nil {
		t.Fatalf("list assignments by category: %v", err)
	}
	assertAssignmentTxnIDs(t, assignments, []string{"txn-a", "txn-c"})

	_, err = assignmentRepo.Get(context.Background(), userTwo, "txn-a")
	assertPortNotFound(t, err)
	if err := assignmentRepo.Delete(context.Background(), userTwo, "txn-a"); err != nil {
		t.Fatalf("delete assignment as other tenant: %v", err)
	}
	if _, err := assignmentRepo.Get(context.Background(), userOne, "txn-a"); err != nil {
		t.Fatalf("other tenant delete removed assignment: %v", err)
	}
	_, err = assignmentRepo.UpsertIfChanged(context.Background(), userTwo, domain.CategoryAssignment{
		TxnID:      "txn-a",
		CategoryID: categoryOne.ID,
		Source:     domain.AssignmentSourceRule,
		RuleID:     &ruleID,
	})
	if err == nil {
		t.Fatal("cross-tenant assignment mutation succeeded")
	}
}

func TestTxnRepoListAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "txn-list-task5-user1@example.test")
	userTwo := seedUser(t, db.owner, "txn-list-task5-user2@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)

	mustUpsertAccount(t, accountRepo, userOne, "acc-txn-list-user1")
	mustUpsertAccount(t, accountRepo, userTwo, "acc-txn-list-user2")
	mustUpsertTxn(t, txnRepo, userOne, "txn-b", "acc-txn-list-user1", time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	mustUpsertTxn(t, txnRepo, userOne, "txn-c", "acc-txn-list-user1", time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC))
	mustUpsertTxn(t, txnRepo, userOne, "txn-a", "acc-txn-list-user1", time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	mustUpsertTxn(t, txnRepo, userTwo, "txn-other", "acc-txn-list-user2", time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))

	txns, err := txnRepo.List(context.Background(), userOne)
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	assertTxnIDs(t, txns, []string{"txn-a", "txn-b", "txn-c"})

	otherTxns, err := txnRepo.List(context.Background(), userTwo)
	if err != nil {
		t.Fatalf("list other tenant transactions: %v", err)
	}
	assertTxnIDs(t, otherTxns, []string{"txn-other"})
}

func mustUpsertCategory(t *testing.T, repo *CategoryRepo, userID domain.UserID, name string, kind domain.CategoryKind) domain.Category {
	t.Helper()
	category, err := repo.Upsert(context.Background(), userID, domain.Category{Name: name, Kind: kind})
	if err != nil {
		t.Fatalf("upsert category %s: %v", name, err)
	}
	return category
}

func mustUpsertRule(t *testing.T, repo *RuleRepo, userID domain.UserID, name string, categoryID int64) domain.Rule {
	t.Helper()
	rule, err := repo.Upsert(context.Background(), userID, domain.Rule{
		Name:       name,
		Priority:   1,
		Predicate:  json.RawMessage(`{}`),
		CategoryID: categoryID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("upsert rule %s: %v", name, err)
	}
	return rule
}

func mustUpsertAccount(t *testing.T, repo *AccountRepo, userID domain.UserID, id string) {
	t.Helper()
	if err := repo.Upsert(context.Background(), userID, domain.Account{ID: id, Name: id, Currency: "NZD"}); err != nil {
		t.Fatalf("upsert account %s: %v", id, err)
	}
}

func mustUpsertTxn(t *testing.T, repo *TxnRepo, userID domain.UserID, id string, accountID string, postedAt time.Time) {
	t.Helper()
	err := repo.Upsert(context.Background(), userID, domain.Transaction{
		ID:        id,
		AccountID: accountID,
		PostedAt:  postedAt,
		Amount:    domain.MustMoneyFromString("1.00"),
		Direction: domain.DirectionDebit,
		RawJSON:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("upsert txn %s: %v", id, err)
	}
}

func mustUpsertAssignment(t *testing.T, repo *AssignmentRepo, userID domain.UserID, txnID string, categoryID int64, source domain.AssignmentSource, ruleID *int64) {
	t.Helper()
	changed, err := repo.UpsertIfChanged(context.Background(), userID, domain.CategoryAssignment{
		TxnID:      txnID,
		CategoryID: categoryID,
		Source:     source,
		RuleID:     ruleID,
	})
	if err != nil {
		t.Fatalf("upsert assignment %s: %v", txnID, err)
	}
	if !changed {
		t.Fatalf("assignment %s changed = false, want true", txnID)
	}
}

func assertPortNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error = %v, want ports.ErrNotFound", err)
	}
}

func assertJSONEqual(t *testing.T, got json.RawMessage, want json.RawMessage) {
	t.Helper()
	var gotValue any
	var wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got JSON: %v", err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal want JSON: %v", err)
	}
	if !jsonEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

func jsonEqual(a any, b any) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aBytes) == string(bBytes)
}

func assertCategoryNames(t *testing.T, categories []domain.Category, want []string) {
	t.Helper()
	if len(categories) != len(want) {
		t.Fatalf("categories = %+v, want names %v", categories, want)
	}
	for i, category := range categories {
		if category.Name != want[i] {
			t.Fatalf("category names = %+v, want %v", categories, want)
		}
	}
}

func assertRuleNames(t *testing.T, rules []domain.Rule, want []string) {
	t.Helper()
	if len(rules) != len(want) {
		t.Fatalf("rules = %+v, want names %v", rules, want)
	}
	for i, rule := range rules {
		if rule.Name != want[i] {
			t.Fatalf("rule names = %+v, want %v", rules, want)
		}
	}
}

func assertAssignmentTxnIDs(t *testing.T, assignments []domain.CategoryAssignment, want []string) {
	t.Helper()
	if len(assignments) != len(want) {
		t.Fatalf("assignments = %+v, want txn IDs %v", assignments, want)
	}
	for i, assignment := range assignments {
		if assignment.TxnID != want[i] {
			t.Fatalf("assignment txn IDs = %+v, want %v", assignments, want)
		}
	}
}

func assertTxnIDs(t *testing.T, txns []domain.Transaction, want []string) {
	t.Helper()
	if len(txns) != len(want) {
		t.Fatalf("transactions = %+v, want IDs %v", txns, want)
	}
	for i, txn := range txns {
		if txn.ID != want[i] {
			t.Fatalf("transaction IDs = %+v, want %v", txns, want)
		}
	}
}
