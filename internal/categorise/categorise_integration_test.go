//go:build integration

package categorise

import (
	"context"
	"database/sql"
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

func TestCategoriseIntegrationConvergesPreservesManualAndIsolatesTenants(t *testing.T) {
	db := newCategoriseIntegrationDatabase(t)
	ctx := context.Background()
	runID := time.Now().UnixNano()
	userOne := seedCategoriseIntegrationUser(t, db.owner, runID+1, "categorise-integration-user1@example.test")
	userTwo := seedCategoriseIntegrationUser(t, db.owner, runID+2, "categorise-integration-user2@example.test")
	t.Cleanup(func() {
		cleanupCategoriseIntegrationUser(t, db.owner, userOne)
		cleanupCategoriseIntegrationUser(t, db.owner, userTwo)
	})

	accountRepo := postgres.NewAccountRepo(db.app)
	txnRepo := postgres.NewTxnRepo(db.app)
	categoryRepo := postgres.NewCategoryRepo(db.app)
	ruleRepo := postgres.NewRuleRepo(db.app)
	assignmentRepo := postgres.NewAssignmentRepo(db.app)
	deps := Deps{
		Categories:  categoryRepo,
		Rules:       ruleRepo,
		Assignments: assignmentRepo,
		Txns:        txnRepo,
		Clock:       categoriseIntegrationClock{now: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)},
	}

	seedCategoriseIntegrationAccount(t, ctx, accountRepo, userOne)
	seedCategoriseIntegrationAccount(t, ctx, accountRepo, userTwo)
	seedCategoriseIntegrationTxn(t, ctx, txnRepo, userOne, "txn-m3-groceries", "Synthetic grocery transaction", "Synthetic Grocer", domain.DirectionDebit)
	seedCategoriseIntegrationTxn(t, ctx, txnRepo, userOne, "txn-m3-unknown", "Synthetic unmatched transaction", "Synthetic Unknown", domain.DirectionDebit)
	seedCategoriseIntegrationTxn(t, ctx, txnRepo, userOne, "txn-m3-income", "Synthetic payroll credit", "Synthetic Employer", domain.DirectionCredit)
	seedCategoriseIntegrationTxn(t, ctx, txnRepo, userTwo, "txn-m3-groceries", "Synthetic tenant-two transaction", "Synthetic Grocer", domain.DirectionDebit)

	cfg := categoriseIntegrationConfig([]Rule{
		{
			Name:     "Groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Grocer"},
			},
			Category: "Food/Groceries",
		},
		{
			Name:     "Income",
			Priority: 20,
			Predicate: Predicate{
				Direction:          string(domain.DirectionCredit),
				DescriptionMatches: "(?i)payroll",
			},
			Category: "Income",
		},
	})

	if err := Categorise(ctx, userOne, deps, cfg); err != nil {
		t.Fatalf("categorise first run: %v", err)
	}
	firstCount := countAppVisibleAssignments(t, db.app, userOne)
	if firstCount != 3 {
		t.Fatalf("first run assignment rows = %d, want 3", firstCount)
	}
	firstAssignedAt := map[string]time.Time{
		"txn-m3-groceries": mustGetIntegrationAssignment(t, ctx, assignmentRepo, userOne, "txn-m3-groceries").AssignedAt,
		"txn-m3-unknown":   mustGetIntegrationAssignment(t, ctx, assignmentRepo, userOne, "txn-m3-unknown").AssignedAt,
		"txn-m3-income":    mustGetIntegrationAssignment(t, ctx, assignmentRepo, userOne, "txn-m3-income").AssignedAt,
	}

	if err := Categorise(ctx, userOne, deps, cfg); err != nil {
		t.Fatalf("categorise second run: %v", err)
	}
	if secondCount := countAppVisibleAssignments(t, db.app, userOne); secondCount != firstCount {
		t.Fatalf("second run assignment rows = %d, want %d", secondCount, firstCount)
	}
	assertAssignedAtStable(t, ctx, assignmentRepo, userOne, firstAssignedAt)

	userTwoCfg := categoriseIntegrationConfig([]Rule{
		{
			Name:     "Tenant two groceries",
			Priority: 10,
			Predicate: Predicate{
				MerchantIn: []string{"Synthetic Grocer"},
			},
			Category: "Tenant Two Only",
		},
	})
	userTwoCfg.Categories = append(userTwoCfg.Categories, Category{Name: "Tenant Two Only", Kind: KindExpense})
	if err := Categorise(ctx, userTwo, deps, userTwoCfg); err != nil {
		t.Fatalf("categorise second tenant: %v", err)
	}
	assertAppVisibleNameCount(t, db.app, userOne, "categories", "Tenant Two Only", 0)
	assertAppVisibleNameCount(t, db.app, userTwo, "categories", "Tenant Two Only", 1)
	assertAppVisibleNameCount(t, db.app, userOne, "rules", "Tenant two groceries", 0)
	assertAppVisibleNameCount(t, db.app, userTwo, "rules", "Tenant two groceries", 1)
	assertAppVisibleAssignmentCount(t, db.app, userOne, "txn-m3-groceries", 1)
	assertAppVisibleAssignmentCount(t, db.app, userTwo, "txn-m3-groceries", 1)

	manualCategory, err := categoryRepo.GetByName(ctx, userOne, "Food")
	if err != nil {
		t.Fatalf("get manual category: %v", err)
	}
	if changed, err := assignmentRepo.UpsertIfChanged(ctx, userOne, domain.CategoryAssignment{
		TxnID:      "txn-m3-groceries",
		CategoryID: manualCategory.ID,
		Source:     domain.AssignmentSourceManual,
	}); err != nil {
		t.Fatalf("seed manual assignment: %v", err)
	} else if !changed {
		t.Fatal("manual assignment changed = false, want true")
	}
	manualBefore := mustGetIntegrationAssignment(t, ctx, assignmentRepo, userOne, "txn-m3-groceries")

	editedCfg := cfg
	editedCfg.Rules[0].Category = "Uncategorised"
	if err := Categorise(ctx, userOne, deps, editedCfg); err != nil {
		t.Fatalf("categorise after rule edit: %v", err)
	}
	deletedRuleCfg := cfg
	deletedRuleCfg.Rules = deletedRuleCfg.Rules[1:]
	if err := Categorise(ctx, userOne, deps, deletedRuleCfg); err != nil {
		t.Fatalf("categorise after rule deletion: %v", err)
	}
	manualAfter := mustGetIntegrationAssignment(t, ctx, assignmentRepo, userOne, "txn-m3-groceries")
	if manualAfter.CategoryID != manualBefore.CategoryID || manualAfter.Source != domain.AssignmentSourceManual || manualAfter.RuleID != nil {
		t.Fatalf("manual assignment after rule edit/deletion = %+v, want category %d manual with nil rule", manualAfter, manualBefore.CategoryID)
	}
	if !manualAfter.AssignedAt.Equal(manualBefore.AssignedAt) {
		t.Fatalf("manual assigned_at changed from %s to %s", manualBefore.AssignedAt, manualAfter.AssignedAt)
	}
	assertAppVisibleNameCount(t, db.app, userTwo, "rules", "Tenant two groceries", 1)
}

type categoriseIntegrationDatabase struct {
	owner *sql.DB
	app   *sql.DB
}

func newCategoriseIntegrationDatabase(t *testing.T) categoriseIntegrationDatabase {
	t.Helper()

	ownerURL := os.Getenv("DATABASE_URL_OWNER")
	appURL := os.Getenv("DATABASE_URL_APP")
	if ownerURL == "" || appURL == "" {
		t.Skip("DATABASE_URL_OWNER and DATABASE_URL_APP are required for integration tests")
	}

	root := categoriseIntegrationRepoRoot(t)
	owner := openCategoriseIntegrationDB(t, ownerURL)
	runCategoriseIntegrationMigrations(t, owner, filepath.Join(root, "migrations"))
	app := openCategoriseIntegrationDB(t, appURL)

	t.Cleanup(func() {
		_ = app.Close()
		_ = owner.Close()
	})
	return categoriseIntegrationDatabase{owner: owner, app: app}
}

func categoriseIntegrationRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func openCategoriseIntegrationDB(t *testing.T, dsn string) *sql.DB {
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

func runCategoriseIntegrationMigrations(t *testing.T, db *sql.DB, migrationsPath string) {
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

func seedCategoriseIntegrationUser(t *testing.T, db *sql.DB, id int64, email string) domain.UserID {
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
	`, id, fmt.Sprintf("%d-%s", id, email), "Synthetic categorisation test user"); err != nil {
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

func cleanupCategoriseIntegrationUser(t *testing.T, db *sql.DB, userID domain.UserID) {
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

func seedCategoriseIntegrationAccount(t *testing.T, ctx context.Context, repo *postgres.AccountRepo, userID domain.UserID) {
	t.Helper()
	if err := repo.Upsert(ctx, userID, domain.Account{
		ID:       "acc-m3-categorise",
		Name:     "Synthetic categorisation account",
		Bank:     "SYNTHETIC",
		Type:     "CHECKING",
		Currency: "NZD",
	}); err != nil {
		t.Fatalf("seed account for user %s: %v", userID, err)
	}
}

func seedCategoriseIntegrationTxn(
	t *testing.T,
	ctx context.Context,
	repo *postgres.TxnRepo,
	userID domain.UserID,
	id string,
	description string,
	merchant string,
	direction domain.Direction,
) {
	t.Helper()
	amount := "12.34"
	if direction == domain.DirectionCredit {
		amount = "123.45"
	}
	if err := repo.Upsert(ctx, userID, domain.Transaction{
		ID:            id,
		AccountID:     "acc-m3-categorise",
		PostedAt:      time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		Amount:        domain.MustMoneyFromString(amount),
		Direction:     direction,
		Description:   description,
		Merchant:      merchant,
		AkahuCategory: "SYNTHETIC",
	}); err != nil {
		t.Fatalf("seed transaction %s for user %s: %v", id, userID, err)
	}
}

func categoriseIntegrationConfig(rules []Rule) Config {
	return Config{
		Categories: []Category{
			{Name: "Food", Kind: KindExpense},
			{Name: "Food/Groceries", Kind: KindExpense, Parent: "Food"},
			{Name: "Income", Kind: KindIncome},
			{Name: "Uncategorised", Kind: KindExpense},
		},
		Rules: rules,
	}
}

func mustGetIntegrationAssignment(
	t *testing.T,
	ctx context.Context,
	repo *postgres.AssignmentRepo,
	userID domain.UserID,
	txnID string,
) domain.CategoryAssignment {
	t.Helper()
	assignment, err := repo.Get(ctx, userID, txnID)
	if err != nil {
		t.Fatalf("get assignment %s for user %s: %v", txnID, userID, err)
	}
	return assignment
}

func assertAssignedAtStable(
	t *testing.T,
	ctx context.Context,
	repo *postgres.AssignmentRepo,
	userID domain.UserID,
	want map[string]time.Time,
) {
	t.Helper()
	for txnID, assignedAt := range want {
		got := mustGetIntegrationAssignment(t, ctx, repo, userID, txnID)
		if !got.AssignedAt.Equal(assignedAt) {
			t.Fatalf("assignment %s assigned_at changed from %s to %s", txnID, assignedAt, got.AssignedAt)
		}
	}
}

func countAppVisibleAssignments(t *testing.T, db *sql.DB, userID domain.UserID) int {
	t.Helper()
	return queryAppVisibleCount(t, db, userID, `SELECT count(*) FROM category_assignments`)
}

func assertAppVisibleNameCount(t *testing.T, db *sql.DB, userID domain.UserID, table string, name string, want int) {
	t.Helper()
	var query string
	switch table {
	case "categories":
		query = `SELECT count(*) FROM categories WHERE name = $1`
	case "rules":
		query = `SELECT count(*) FROM rules WHERE name = $1`
	default:
		t.Fatalf("unknown table %q", table)
	}
	got := queryAppVisibleCount(t, db, userID, query, name)
	if got != want {
		t.Fatalf("app-visible %s named %q for user %s = %d, want %d", table, name, userID, got, want)
	}
}

func assertAppVisibleAssignmentCount(t *testing.T, db *sql.DB, userID domain.UserID, txnID string, want int) {
	t.Helper()
	got := queryAppVisibleCount(t, db, userID, `SELECT count(*) FROM category_assignments WHERE txn_id = $1`, txnID)
	if got != want {
		t.Fatalf("app-visible assignment %q for user %s = %d, want %d", txnID, userID, got, want)
	}
}

func queryAppVisibleCount(t *testing.T, db *sql.DB, userID domain.UserID, query string, args ...any) int {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin app count tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		t.Fatalf("set app count user id: %v", err)
	}
	var count int
	if err := tx.QueryRowContext(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("query app-visible count for user %s: %v", userID, err)
	}
	return count
}

type categoriseIntegrationClock struct {
	now time.Time
}

func (c categoriseIntegrationClock) Now() time.Time {
	return c.now
}
