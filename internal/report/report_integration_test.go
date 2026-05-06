//go:build integration

package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/domain"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	migrate "github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	reportSharedAccountID  = "m4-report-shared-account"
	reportSharedAprilTxnID = "m4-report-shared-april"
	reportSharedMayTxnID   = "m4-report-shared-may"
	reportBOnlyTxnID       = "m4-report-b-only"
	reportGroceriesName    = "M4 Reporting/Groceries"
	reportBOnlyName        = "M4 Reporting/B Only"
)

func TestReportsAreTenantScoped(t *testing.T) {
	db := newReportTestDatabase(t)
	fixture := seedReportTenantFixture(t, db)
	ctx := context.Background()

	deps := SummaryDeps{
		Txns:        postgres.NewTxnRepo(db.app),
		Categories:  postgres.NewCategoryRepo(db.app),
		Assignments: postgres.NewAssignmentRepo(db.app),
	}
	txnDeps := TransactionsDeps{Txns: postgres.NewTxnRepo(db.app)}

	t.Run("summary excludes other tenant rows", func(t *testing.T) {
		got, err := Summary(ctx, fixture.userA, deps, fixture.april)
		if err != nil {
			t.Fatalf("summary user A: %v", err)
		}

		if got.Expense != MoneyAmount("10.00") {
			t.Fatalf("expense = %q, want only user A total 10.00", got.Expense)
		}
		if got.Net != MoneyAmount("-10.00") {
			t.Fatalf("net = %q, want only user A net -10.00", got.Net)
		}
		assertCategoryTotal(t, got.Categories, reportGroceriesName, "10.00")
		assertNoCategory(t, got.Categories, reportBOnlyName)
	})

	t.Run("compare excludes other tenant rows", func(t *testing.T) {
		got, err := Compare(ctx, fixture.userA, deps, fixture.april, fixture.may, CompareOptions{})
		if err != nil {
			t.Fatalf("compare user A: %v", err)
		}

		row := requireCompareCategory(t, got.Categories, reportGroceriesName)
		if row.A != MoneyAmount("10.00") || row.B != MoneyAmount("40.00") || row.Delta != MoneyAmount("30.00") {
			t.Fatalf("compare row = %+v, want only user A amounts 10.00 -> 40.00", row)
		}
		assertNoCompareCategory(t, got.Categories, reportBOnlyName)
	})

	t.Run("transactions excludes other tenant rows", func(t *testing.T) {
		got, err := Transactions(ctx, fixture.userA, txnDeps, TxnFilter{
			Period: fixture.april,
			Sort:   "date",
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("transactions user A: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("transactions = %+v, want only user A April transaction", got)
		}
		row := got[0]
		if row.TxnID != reportSharedAprilTxnID ||
			row.Amount != MoneyAmount("10.00") ||
			row.Merchant != "Synthetic A Market" ||
			row.Category != reportGroceriesName {
			t.Fatalf("transaction row = %+v, want user A shared transaction only", row)
		}
		if row.TxnID == reportBOnlyTxnID || row.Category == reportBOnlyName || row.Merchant == "Synthetic B Only Market" {
			t.Fatalf("transaction row leaked user B data: %+v", row)
		}
	})

	t.Run("app role unfiltered counts see only current tenant", func(t *testing.T) {
		assertAppVisibleCount(t, db.app, fixture.userA, `SELECT count(*) FROM accounts WHERE id = $1`, 1, reportSharedAccountID)
		assertAppVisibleCount(t, db.app, fixture.userB, `SELECT count(*) FROM accounts WHERE id = $1`, 1, reportSharedAccountID)
		assertAppVisibleCount(t, db.app, fixture.userA, `SELECT count(*) FROM transactions WHERE id = $1`, 1, reportSharedAprilTxnID)
		assertAppVisibleCount(t, db.app, fixture.userB, `SELECT count(*) FROM transactions WHERE id = $1`, 1, reportSharedAprilTxnID)
		assertAppVisibleCount(t, db.app, fixture.userA, `SELECT count(*) FROM categories WHERE name = $1`, 1, reportGroceriesName)
		assertAppVisibleCount(t, db.app, fixture.userB, `SELECT count(*) FROM categories WHERE name = $1`, 1, reportGroceriesName)
		assertAppVisibleCount(t, db.app, fixture.userA, `SELECT count(*) FROM category_assignments WHERE txn_id = $1`, 1, reportSharedAprilTxnID)
		assertAppVisibleCount(t, db.app, fixture.userB, `SELECT count(*) FROM category_assignments WHERE txn_id = $1`, 1, reportSharedAprilTxnID)
	})
}

type reportTenantFixture struct {
	userA domain.UserID
	userB domain.UserID
	april domain.Range
	may   domain.Range
}

func seedReportTenantFixture(t *testing.T, db reportTestDatabase) reportTenantFixture {
	t.Helper()

	runID := time.Now().UnixNano()
	userA := seedReportUser(t, db.owner, runID+1, "report-tenant-a@example.test")
	userB := seedReportUser(t, db.owner, runID+2, "report-tenant-b@example.test")
	t.Cleanup(func() {
		cleanupReportUser(t, db.owner, userA)
		cleanupReportUser(t, db.owner, userB)
	})

	accountRepo := postgres.NewAccountRepo(db.app)
	categoryRepo := postgres.NewCategoryRepo(db.app)
	txnRepo := postgres.NewTxnRepo(db.app)
	assignmentRepo := postgres.NewAssignmentRepo(db.app)

	mustUpsertReportAccount(t, accountRepo, userA, reportSharedAccountID)
	mustUpsertReportAccount(t, accountRepo, userB, reportSharedAccountID)

	userAGroceries := mustUpsertReportCategory(t, categoryRepo, userA, reportGroceriesName, domain.CategoryKindExpense)
	userBGroceries := mustUpsertReportCategory(t, categoryRepo, userB, reportGroceriesName, domain.CategoryKindExpense)
	userBOnly := mustUpsertReportCategory(t, categoryRepo, userB, reportBOnlyName, domain.CategoryKindExpense)

	april := domain.Range{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	may := domain.Range{
		From: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	mustUpsertReportTxn(t, txnRepo, assignmentRepo, userA, reportTxnFixture{
		id:         reportSharedAprilTxnID,
		postedAt:   april.From.AddDate(0, 0, 5),
		amount:     "10.00",
		merchant:   "Synthetic A Market",
		categoryID: userAGroceries.ID,
	})
	mustUpsertReportTxn(t, txnRepo, assignmentRepo, userA, reportTxnFixture{
		id:         reportSharedMayTxnID,
		postedAt:   may.From.AddDate(0, 0, 5),
		amount:     "40.00",
		merchant:   "Synthetic A Market",
		categoryID: userAGroceries.ID,
	})
	mustUpsertReportTxn(t, txnRepo, assignmentRepo, userB, reportTxnFixture{
		id:         reportSharedAprilTxnID,
		postedAt:   april.From.AddDate(0, 0, 5),
		amount:     "900.00",
		merchant:   "Synthetic B Market",
		categoryID: userBGroceries.ID,
	})
	mustUpsertReportTxn(t, txnRepo, assignmentRepo, userB, reportTxnFixture{
		id:         reportSharedMayTxnID,
		postedAt:   may.From.AddDate(0, 0, 5),
		amount:     "800.00",
		merchant:   "Synthetic B Market",
		categoryID: userBGroceries.ID,
	})
	mustUpsertReportTxn(t, txnRepo, assignmentRepo, userB, reportTxnFixture{
		id:         reportBOnlyTxnID,
		postedAt:   april.From.AddDate(0, 0, 6),
		amount:     "333.00",
		merchant:   "Synthetic B Only Market",
		categoryID: userBOnly.ID,
	})

	return reportTenantFixture{
		userA: userA,
		userB: userB,
		april: april,
		may:   may,
	}
}

type reportTxnFixture struct {
	id         string
	postedAt   time.Time
	amount     string
	merchant   string
	categoryID int64
}

func mustUpsertReportAccount(t *testing.T, repo *postgres.AccountRepo, userID domain.UserID, id string) {
	t.Helper()
	if err := repo.Upsert(context.Background(), userID, domain.Account{
		ID:       id,
		Name:     "Synthetic reporting account",
		Bank:     "SYNTHETIC",
		Type:     "CHECKING",
		Currency: "NZD",
	}); err != nil {
		t.Fatalf("upsert account %s for user %s: %v", id, userID, err)
	}
}

func mustUpsertReportCategory(
	t *testing.T,
	repo *postgres.CategoryRepo,
	userID domain.UserID,
	name string,
	kind domain.CategoryKind,
) domain.Category {
	t.Helper()
	category, err := repo.Upsert(context.Background(), userID, domain.Category{Name: name, Kind: kind})
	if err != nil {
		t.Fatalf("upsert category %s for user %s: %v", name, userID, err)
	}
	return category
}

func mustUpsertReportTxn(
	t *testing.T,
	txnRepo *postgres.TxnRepo,
	assignmentRepo *postgres.AssignmentRepo,
	userID domain.UserID,
	txn reportTxnFixture,
) {
	t.Helper()
	if err := txnRepo.Upsert(context.Background(), userID, domain.Transaction{
		ID:          txn.id,
		AccountID:   reportSharedAccountID,
		PostedAt:    txn.postedAt,
		Amount:      domain.MustMoneyFromString(txn.amount),
		Direction:   domain.DirectionDebit,
		Description: "Synthetic reporting transaction",
		Merchant:    txn.merchant,
		RawJSON:     json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("upsert transaction %s for user %s: %v", txn.id, userID, err)
	}
	if err := assignmentRepo.Upsert(context.Background(), userID, domain.CategoryAssignment{
		TxnID:      txn.id,
		CategoryID: txn.categoryID,
		Source:     domain.AssignmentSourceManual,
	}); err != nil {
		t.Fatalf("upsert assignment %s for user %s: %v", txn.id, userID, err)
	}
}

func assertCategoryTotal(t *testing.T, categories []CategoryTotal, name string, want MoneyAmount) {
	t.Helper()
	for _, category := range categories {
		if category.Category == name {
			if category.Total != want {
				t.Fatalf("category %q total = %q, want %q", name, category.Total, want)
			}
			return
		}
	}
	t.Fatalf("category %q not found in %+v", name, categories)
}

func assertNoCategory(t *testing.T, categories []CategoryTotal, name string) {
	t.Helper()
	for _, category := range categories {
		if category.Category == name {
			t.Fatalf("category %q leaked into summary: %+v", name, categories)
		}
	}
}

func requireCompareCategory(t *testing.T, categories []CompareCategory, name string) CompareCategory {
	t.Helper()
	for _, category := range categories {
		if category.Category == name {
			return category
		}
	}
	t.Fatalf("compare category %q not found in %+v", name, categories)
	return CompareCategory{}
}

func assertNoCompareCategory(t *testing.T, categories []CompareCategory, name string) {
	t.Helper()
	for _, category := range categories {
		if category.Category == name {
			t.Fatalf("category %q leaked into compare: %+v", name, categories)
		}
	}
}

func assertAppVisibleCount(t *testing.T, db *sql.DB, userID domain.UserID, query string, want int, args ...any) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin app-visible count tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		t.Fatalf("set app-visible user id: %v", err)
	}
	var got int
	if err := tx.QueryRowContext(context.Background(), query, args...).Scan(&got); err != nil {
		t.Fatalf("query app-visible count for user %s: %v", userID, err)
	}
	if got != want {
		t.Fatalf("app-visible count for user %s = %d, want %d", userID, got, want)
	}
}

type reportTestDatabase struct {
	owner *sql.DB
	app   *sql.DB
}

func newReportTestDatabase(t *testing.T) reportTestDatabase {
	t.Helper()

	ownerURL := os.Getenv("DATABASE_URL_OWNER")
	appURL := os.Getenv("DATABASE_URL_APP")
	if ownerURL == "" || appURL == "" {
		t.Skip("DATABASE_URL_OWNER and DATABASE_URL_APP are required for integration tests")
	}

	root := reportRepoRoot(t)
	owner := openReportDB(t, ownerURL)
	runReportMigrations(t, owner, filepath.Join(root, "migrations"))
	app := openReportDB(t, appURL)

	t.Cleanup(func() {
		_ = app.Close()
		_ = owner.Close()
	})
	return reportTestDatabase{owner: owner, app: app}
}

func reportRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func openReportDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	return db
}

func runReportMigrations(t *testing.T, db *sql.DB, migrationsPath string) {
	t.Helper()
	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	if err != nil {
		t.Fatalf("create migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("run migrations: %v", err)
	}
}

func seedReportUser(t *testing.T, db *sql.DB, id int64, email string) domain.UserID {
	t.Helper()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin seed user tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, fmt.Sprint(id)); err != nil {
		t.Fatalf("set seed user id: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO users (id, email, display_name)
		OVERRIDING SYSTEM VALUE
		VALUES ($1, $2, $3)
	`, id, fmt.Sprintf("%d-%s", id, email), "Synthetic reporting test user"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed user: %v", err)
	}
	userID, err := domain.NewUserID(id)
	if err != nil {
		t.Fatalf("new user id: %v", err)
	}
	return userID
}

func cleanupReportUser(t *testing.T, db *sql.DB, userID domain.UserID) {
	t.Helper()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin cleanup user tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		t.Fatalf("set cleanup user id: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID.Int64()); err != nil {
		t.Fatalf("cleanup user %s: %v", userID, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit cleanup user: %v", err)
	}
}
