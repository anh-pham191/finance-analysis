//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestAccountRepoRoundTripAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "accounts-user1@example.test")
	userTwo := seedUser(t, db.owner, "accounts-user2@example.test")
	repo := NewAccountRepo(db.app)

	account := domain.Account{ID: "acc-shared", Name: "Everyday", Bank: "ANZ", Type: "checking", Currency: "NZD"}
	if err := repo.Upsert(context.Background(), userOne, account); err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	got, err := repo.Get(context.Background(), userOne, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.ID != account.ID || got.Name != account.Name {
		t.Fatalf("account = %+v, want %+v", got, account)
	}
	_, err = repo.Get(context.Background(), userTwo, account.ID)
	assertNotFound(t, err)
}

func TestTxnRepoRoundTripAndTenantIsolation(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "txn-user1@example.test")
	userTwo := seedUser(t, db.owner, "txn-user2@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)

	account := domain.Account{ID: "acc-txn", Name: "Everyday", Bank: "ANZ", Type: "checking", Currency: "NZD"}
	if err := accountRepo.Upsert(context.Background(), userOne, account); err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	txn := domain.Transaction{
		ID:            "txn-1",
		AccountID:     account.ID,
		PostedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		Amount:        domain.MustMoneyFromString("12.30"),
		Direction:     domain.DirectionDebit,
		Description:   "Groceries",
		Merchant:      "Supermarket",
		AkahuCategory: "FOOD",
		RawJSON:       json.RawMessage(`{"id":"txn-1"}`),
	}
	if err := txnRepo.Upsert(context.Background(), userOne, txn); err != nil {
		t.Fatalf("upsert txn: %v", err)
	}
	got, err := txnRepo.Get(context.Background(), userOne, txn.ID)
	if err != nil {
		t.Fatalf("get txn: %v", err)
	}
	if got.ID != txn.ID || got.Amount.String() != "12.30" || got.Direction != domain.DirectionDebit {
		t.Fatalf("txn = %+v, want %+v", got, txn)
	}
	_, err = txnRepo.Get(context.Background(), userTwo, txn.ID)
	assertNotFound(t, err)
}

func TestTxnRepoPreservesStableFieldsOnCorrection(t *testing.T) {
	db := newTestDatabase(t)
	userID := seedUser(t, db.owner, "txn-correction-user@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)

	account := domain.Account{ID: "acc-correction", Name: "Everyday", Bank: "ANZ", Type: "checking", Currency: "NZD"}
	if err := accountRepo.Upsert(context.Background(), userID, account); err != nil {
		t.Fatalf("upsert account: %v", err)
	}

	postedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	txn := domain.Transaction{
		ID:        "txn-correction",
		AccountID: account.ID,
		PostedAt:  postedAt,
		Amount:    domain.MustMoneyFromString("10.00"),
		Direction: domain.DirectionDebit,
		RawJSON:   json.RawMessage(`{"version":1}`),
	}
	if err := txnRepo.Upsert(context.Background(), userID, txn); err != nil {
		t.Fatalf("upsert initial txn: %v", err)
	}

	correctedTxn := domain.Transaction{
		ID:        txn.ID,
		AccountID: account.ID,
		PostedAt:  time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
		Amount:    domain.MustMoneyFromString("99.00"),
		Direction: domain.DirectionDebit,
		RawJSON:   json.RawMessage(`{"version":2}`),
	}
	if err := txnRepo.Upsert(context.Background(), userID, correctedTxn); err != nil {
		t.Fatalf("upsert corrected txn: %v", err)
	}

	got, err := txnRepo.Get(context.Background(), userID, txn.ID)
	if err != nil {
		t.Fatalf("get txn: %v", err)
	}
	if got.Amount.String() != "10.00" {
		t.Fatalf("amount = %s, want 10.00", got.Amount.String())
	}
	if !got.PostedAt.Equal(postedAt) {
		t.Fatalf("posted_at = %s, want %s", got.PostedAt.Format(time.RFC3339), postedAt.Format(time.RFC3339))
	}
	var raw struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(got.RawJSON, &raw); err != nil {
		t.Fatalf("unmarshal raw_json: %v", err)
	}
	if raw.Version != 2 {
		t.Fatalf("raw_json version = %d, want 2", raw.Version)
	}
}

func TestCategoryRuleAssignmentAndSyncStateRepos(t *testing.T) {
	db := newTestDatabase(t)
	userID := seedUser(t, db.owner, "repo-user1@example.test")
	otherUserID := seedUser(t, db.owner, "repo-user2@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)
	categoryRepo := NewCategoryRepo(db.app)
	ruleRepo := NewRuleRepo(db.app)
	assignmentRepo := NewAssignmentRepo(db.app)
	syncRepo := NewSyncStateRepo(db.app)

	category, err := categoryRepo.Insert(context.Background(), userID, domain.Category{Name: "Food/Groceries", Kind: domain.CategoryKindExpense})
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}
	if category.ID == 0 {
		t.Fatal("category ID was not populated")
	}
	gotCategory, err := categoryRepo.GetByName(context.Background(), userID, category.Name)
	if err != nil {
		t.Fatalf("get category: %v", err)
	}
	if gotCategory.ID != category.ID {
		t.Fatalf("category ID = %d, want %d", gotCategory.ID, category.ID)
	}
	_, err = categoryRepo.GetByName(context.Background(), otherUserID, category.Name)
	assertNotFound(t, err)

	rule, err := ruleRepo.Insert(context.Background(), userID, domain.Rule{
		Name:       "Groceries",
		Priority:   10,
		Predicate:  json.RawMessage(`{"merchant_in":["Supermarket"]}`),
		CategoryID: category.ID,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("rule ID was not populated")
	}
	_, err = ruleRepo.GetByName(context.Background(), otherUserID, rule.Name)
	assertNotFound(t, err)

	account := domain.Account{ID: "acc-assignment", Name: "Everyday", Bank: "ANZ", Type: "checking", Currency: "NZD"}
	if err := accountRepo.Upsert(context.Background(), userID, account); err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	txn := domain.Transaction{
		ID:        "txn-assignment",
		AccountID: account.ID,
		PostedAt:  time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		Amount:    domain.MustMoneyFromString("10.00"),
		Direction: domain.DirectionDebit,
	}
	if err := txnRepo.Upsert(context.Background(), userID, txn); err != nil {
		t.Fatalf("upsert txn: %v", err)
	}

	ruleID := rule.ID
	assignment := domain.CategoryAssignment{
		TxnID:      txn.ID,
		CategoryID: category.ID,
		Source:     domain.AssignmentSourceRule,
		RuleID:     &ruleID,
	}
	if err := assignmentRepo.Upsert(context.Background(), userID, assignment); err != nil {
		t.Fatalf("upsert assignment: %v", err)
	}
	gotAssignment, err := assignmentRepo.Get(context.Background(), userID, txn.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if gotAssignment.CategoryID != category.ID || gotAssignment.RuleID == nil || *gotAssignment.RuleID != rule.ID {
		t.Fatalf("assignment = %+v, want category %d rule %d", gotAssignment, category.ID, rule.ID)
	}
	_, err = assignmentRepo.Get(context.Background(), otherUserID, txn.ID)
	assertNotFound(t, err)

	cursor := "abc"
	lastSynced := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	state := domain.SyncState{AccountID: account.ID, LastSyncedAt: &lastSynced, LastCursor: &cursor}
	if err := syncRepo.Upsert(context.Background(), userID, state); err != nil {
		t.Fatalf("upsert sync state: %v", err)
	}
	gotState, err := syncRepo.Get(context.Background(), userID, account.ID)
	if err != nil {
		t.Fatalf("get sync state: %v", err)
	}
	if gotState.LastCursor == nil || *gotState.LastCursor != cursor {
		t.Fatalf("sync state cursor = %v, want %q", gotState.LastCursor, cursor)
	}
	_, err = syncRepo.Get(context.Background(), otherUserID, account.ID)
	assertNotFound(t, err)
}

func TestCascadeDeleteRemovesUserOwnedRows(t *testing.T) {
	db := newTestDatabase(t)
	userID := seedUser(t, db.owner, "cascade-user1@example.test")
	accountRepo := NewAccountRepo(db.app)

	if err := accountRepo.Upsert(context.Background(), userID, domain.Account{ID: "acc-cascade", Name: "Everyday", Currency: "NZD"}); err != nil {
		t.Fatalf("upsert account: %v", err)
	}

	if err := deleteUser(t, db.owner, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	if err := withUserTx(context.Background(), db.owner, userID, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT count(*) FROM accounts WHERE user_id = $1`, userID.Int64()).Scan(&count)
	}); err != nil {
		t.Fatalf("count accounts after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("accounts after user delete = %d, want 0", count)
	}
}

func TestCrossTenantForeignKeysRejectCategoryReferences(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "category-fk-user1@example.test")
	userTwo := seedUser(t, db.owner, "category-fk-user2@example.test")
	categoryRepo := NewCategoryRepo(db.app)
	ruleRepo := NewRuleRepo(db.app)

	category, err := categoryRepo.Insert(context.Background(), userOne, domain.Category{Name: "TenantOne", Kind: domain.CategoryKindExpense})
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}

	_, err = categoryRepo.Insert(context.Background(), userTwo, domain.Category{
		Name:     "Bad cross tenant child",
		ParentID: &category.ID,
		Kind:     domain.CategoryKindExpense,
	})
	if err == nil {
		t.Fatal("cross-tenant category parent reference was accepted")
	}

	_, err = ruleRepo.Insert(context.Background(), userTwo, domain.Rule{
		Name:       "Bad cross tenant rule",
		Priority:   1,
		Predicate:  json.RawMessage(`{}`),
		CategoryID: category.ID,
		Enabled:    true,
	})
	if err == nil {
		t.Fatal("cross-tenant category reference was accepted")
	}
}

func TestCrossTenantForeignKeysRejectAssignmentAndSyncStateReferences(t *testing.T) {
	db := newTestDatabase(t)
	userOne := seedUser(t, db.owner, "assignment-fk-user1@example.test")
	userTwo := seedUser(t, db.owner, "assignment-fk-user2@example.test")
	accountRepo := NewAccountRepo(db.app)
	txnRepo := NewTxnRepo(db.app)
	categoryRepo := NewCategoryRepo(db.app)
	assignmentRepo := NewAssignmentRepo(db.app)
	syncRepo := NewSyncStateRepo(db.app)

	account := domain.Account{ID: "acc-cross-tenant", Name: "Everyday", Currency: "NZD"}
	if err := accountRepo.Upsert(context.Background(), userOne, account); err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	txn := domain.Transaction{
		ID:        "txn-cross-tenant",
		AccountID: account.ID,
		PostedAt:  time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		Amount:    domain.MustMoneyFromString("1.00"),
		Direction: domain.DirectionDebit,
	}
	if err := txnRepo.Upsert(context.Background(), userOne, txn); err != nil {
		t.Fatalf("upsert txn: %v", err)
	}
	category, err := categoryRepo.Insert(context.Background(), userOne, domain.Category{Name: "CrossTenantCategory", Kind: domain.CategoryKindExpense})
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}

	err = assignmentRepo.Upsert(context.Background(), userTwo, domain.CategoryAssignment{
		TxnID:      txn.ID,
		CategoryID: category.ID,
		Source:     domain.AssignmentSourceManual,
	})
	if err == nil {
		t.Fatal("cross-tenant assignment reference was accepted")
	}

	err = syncRepo.Upsert(context.Background(), userTwo, domain.SyncState{AccountID: account.ID})
	if err == nil {
		t.Fatal("cross-tenant sync_state reference was accepted")
	}
}

var _ = sql.ErrNoRows

func deleteUser(t *testing.T, db *sql.DB, userID domain.UserID) error {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID.Int64()); err != nil {
		return err
	}
	return tx.Commit()
}
