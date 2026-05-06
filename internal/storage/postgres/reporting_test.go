//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestReportingTxnQuery(t *testing.T) {
	db := newTestDatabase(t)
	fixture := seedReportingTxnFixture(t, db)
	repo := NewTxnRepo(db.app)

	aprilRange := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}

	t.Run("date range is half open", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Range: aprilRange})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-02", "rpt-a-06", "rpt-a-05", "rpt-a-03"})
	})

	t.Run("category filter uses assignments", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{CategoryID: &fixture.groceriesID})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-02"})
	})

	t.Run("merchant filter is exact", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Merchant: "Alpha Market"})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-06"})
	})

	t.Run("account filter is exact", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{AccountID: fixture.savingsAccountID})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-03"})
	})

	t.Run("direction filter is exact", func(t *testing.T) {
		direction := domain.DirectionCredit
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Direction: &direction})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-04"})
	})

	t.Run("min and max amount are inclusive", func(t *testing.T) {
		min := domain.MustMoneyFromString("10.00")
		max := domain.MustMoneyFromString("25.00")
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Min: &min, Max: &max})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-02", "rpt-a-05"})
	})

	t.Run("sorts by date with id tiebreaker by default", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-02", "rpt-a-06", "rpt-a-05", "rpt-a-03", "rpt-a-04"})
	})

	t.Run("sorts by amount descending with id tiebreaker", func(t *testing.T) {
		direction := domain.DirectionDebit
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Direction: &direction, Sort: "amount"})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-02", "rpt-a-05", "rpt-a-01", "rpt-a-06", "rpt-a-03"})
	})

	t.Run("sorts by merchant with id tiebreaker", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Sort: "merchant"})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01", "rpt-a-06", "rpt-a-05", "rpt-a-03", "rpt-a-02", "rpt-a-04"})
	})

	t.Run("rejects invalid sort", func(t *testing.T) {
		_, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Sort: "description"})
		if err == nil {
			t.Fatal("invalid sort returned nil error")
		}
	})

	t.Run("limit and offset page stable order", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-06", "rpt-a-05"})
	})

	t.Run("offset past end returns empty", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{Offset: 99})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, nil)
	})

	t.Run("tenant isolation excludes matching other tenant rows", func(t *testing.T) {
		txns, err := repo.ListFiltered(context.Background(), fixture.userA, ports.TxnFilter{
			CategoryID: &fixture.groceriesID,
			Merchant:   "Alpha Market",
			Sort:       "merchant",
		})
		if err != nil {
			t.Fatalf("list filtered transactions: %v", err)
		}
		assertTxnIDs(t, txns, []string{"rpt-a-01"})

		otherTxns, err := repo.ListFiltered(context.Background(), fixture.userB, ports.TxnFilter{Merchant: "Alpha Market"})
		if err != nil {
			t.Fatalf("list other tenant filtered transactions: %v", err)
		}
		assertTxnIDs(t, otherTxns, []string{"rpt-b-01"})
	})

}

type reportingTxnFixture struct {
	userA            domain.UserID
	userB            domain.UserID
	savingsAccountID string
	groceriesID      int64
}

func seedReportingTxnFixture(t *testing.T, db testDatabase) reportingTxnFixture {
	t.Helper()

	userA := seedUser(t, db.owner, "reporting-query-a@example.test")
	userB := seedUser(t, db.owner, "reporting-query-b@example.test")
	accountRepo := NewAccountRepo(db.app)
	categoryRepo := NewCategoryRepo(db.app)
	txnRepo := NewTxnRepo(db.app)

	mainAccountID := "rpt-a-main"
	savingsAccountID := "rpt-a-savings"
	mustUpsertAccount(t, accountRepo, userA, mainAccountID)
	mustUpsertAccount(t, accountRepo, userA, savingsAccountID)
	mustUpsertAccount(t, accountRepo, userB, "rpt-b-main")

	groceries := mustUpsertCategory(t, categoryRepo, userA, "Reporting/Groceries", domain.CategoryKindExpense)
	transport := mustUpsertCategory(t, categoryRepo, userA, "Reporting/Transport", domain.CategoryKindExpense)
	income := mustUpsertCategory(t, categoryRepo, userA, "Reporting/Income", domain.CategoryKindIncome)
	otherGroceries := mustUpsertCategory(t, categoryRepo, userB, "Reporting/Groceries", domain.CategoryKindExpense)

	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-01",
		accountID: mainAccountID,
		postedAt:  time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		amount:    "10.00",
		direction: domain.DirectionDebit,
		merchant:  "Alpha Market",
		category:  &groceries.ID,
	})
	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-02",
		accountID: mainAccountID,
		postedAt:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		amount:    "25.00",
		direction: domain.DirectionDebit,
		merchant:  "Coffee Shop",
		category:  &groceries.ID,
	})
	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-03",
		accountID: savingsAccountID,
		postedAt:  time.Date(2026, 4, 30, 23, 59, 0, 0, time.UTC),
		amount:    "5.50",
		direction: domain.DirectionDebit,
		merchant:  "Bus Co",
		category:  &transport.ID,
	})
	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-04",
		accountID: mainAccountID,
		postedAt:  time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		amount:    "1000.00",
		direction: domain.DirectionCredit,
		merchant:  "Employer",
		category:  &income.ID,
	})
	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-05",
		accountID: mainAccountID,
		postedAt:  time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
		amount:    "25.00",
		direction: domain.DirectionDebit,
		merchant:  "Bakery",
	})
	mustUpsertReportingTxn(t, txnRepo, userA, reportingTxn{
		id:        "rpt-a-06",
		accountID: mainAccountID,
		postedAt:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		amount:    "7.00",
		direction: domain.DirectionDebit,
		merchant:  "Alpha Market",
		category:  &transport.ID,
	})
	mustUpsertReportingTxn(t, txnRepo, userB, reportingTxn{
		id:        "rpt-b-01",
		accountID: "rpt-b-main",
		postedAt:  time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		amount:    "10.00",
		direction: domain.DirectionDebit,
		merchant:  "Alpha Market",
		category:  &otherGroceries.ID,
	})

	return reportingTxnFixture{
		userA:            userA,
		userB:            userB,
		savingsAccountID: savingsAccountID,
		groceriesID:      groceries.ID,
	}
}

type reportingTxn struct {
	id        string
	accountID string
	postedAt  time.Time
	amount    string
	direction domain.Direction
	merchant  string
	category  *int64
}

func mustUpsertReportingTxn(t *testing.T, repo *TxnRepo, userID domain.UserID, txn reportingTxn) {
	t.Helper()

	if err := repo.Upsert(context.Background(), userID, domain.Transaction{
		ID:          txn.id,
		AccountID:   txn.accountID,
		PostedAt:    txn.postedAt,
		Amount:      domain.MustMoneyFromString(txn.amount),
		Direction:   txn.direction,
		Description: "Synthetic reporting fixture",
		Merchant:    txn.merchant,
		RawJSON:     []byte(`{}`),
	}); err != nil {
		t.Fatalf("upsert reporting txn %s: %v", txn.id, err)
	}

	if txn.category != nil {
		mustUpsertAssignment(t, NewAssignmentRepo(repo.db), userID, txn.id, *txn.category, domain.AssignmentSourceManual, nil)
	}
}
