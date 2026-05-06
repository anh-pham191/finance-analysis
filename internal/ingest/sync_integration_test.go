//go:build integration

package ingest

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
	"github.com/anh-pham191/finance-analysis/internal/ports"
	"github.com/anh-pham191/finance-analysis/internal/storage/postgres"
	migrate "github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	sharedSyncAccountID = "acc-shared-sync"
	sharedSyncTxnID     = "txn-shared-sync"
)

func TestSyncCrossTenantIsolation(t *testing.T) {
	db := newSyncTestDatabase(t)
	ctx := context.Background()
	runID := time.Now().UnixNano()
	userOne := seedSyncUser(t, db.owner, runID+1, "sync-cross-tenant-user1@example.test")
	userTwo := seedSyncUser(t, db.owner, runID+2, "sync-cross-tenant-user2@example.test")
	t.Cleanup(func() {
		cleanupSyncUser(t, db.owner, userOne)
		cleanupSyncUser(t, db.owner, userTwo)
	})

	accountRepo := postgres.NewAccountRepo(db.app)
	txnRepo := postgres.NewTxnRepo(db.app)
	syncStateRepo := postgres.NewSyncStateRepo(db.app)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	depsA := syncDeps(accountRepo, txnRepo, syncStateRepo, "User A Account", []byte(`{"tenant":"A"}`), now)
	depsB := syncDeps(accountRepo, txnRepo, syncStateRepo, "User B Account", []byte(`{"tenant":"B"}`), now)

	if err := Sync(ctx, userOne, depsA, Options{}); err != nil {
		t.Fatalf("sync user one: %v", err)
	}
	if err := Sync(ctx, userTwo, depsB, Options{}); err != nil {
		t.Fatalf("sync user two: %v", err)
	}

	assertSyncedTenantRows(t, ctx, accountRepo, txnRepo, syncStateRepo, userOne, "User A Account", "A")
	assertSyncedTenantRows(t, ctx, accountRepo, txnRepo, syncStateRepo, userTwo, "User B Account", "B")
	assertVisibleRowCount(t, db.owner, userOne, "accounts", "id", sharedSyncAccountID, 1)
	assertVisibleRowCount(t, db.owner, userTwo, "accounts", "id", sharedSyncAccountID, 1)
	assertVisibleRowCount(t, db.owner, userOne, "transactions", "id", sharedSyncTxnID, 1)
	assertVisibleRowCount(t, db.owner, userTwo, "transactions", "id", sharedSyncTxnID, 1)
	assertAppVisibleRowCount(t, db.app, userOne, "accounts", "id", sharedSyncAccountID, 1)
	assertAppVisibleRowCount(t, db.app, userTwo, "accounts", "id", sharedSyncAccountID, 1)
	assertAppVisibleRowCount(t, db.app, userOne, "transactions", "id", sharedSyncTxnID, 1)
	assertAppVisibleRowCount(t, db.app, userTwo, "transactions", "id", sharedSyncTxnID, 1)
}

func assertSyncedTenantRows(
	t *testing.T,
	ctx context.Context,
	accountRepo ports.AccountRepo,
	txnRepo ports.TxnRepo,
	syncStateRepo ports.SyncStateRepo,
	userID domain.UserID,
	wantAccountName string,
	wantTenant string,
) {
	t.Helper()

	account, err := accountRepo.Get(ctx, userID, sharedSyncAccountID)
	if err != nil {
		t.Fatalf("get account for user %s: %v", userID, err)
	}
	if account.ID != sharedSyncAccountID || account.Name != wantAccountName {
		t.Fatalf("account for user %s has id %q name %q, want shared sync account", userID, account.ID, account.Name)
	}

	txn, err := txnRepo.Get(ctx, userID, sharedSyncTxnID)
	if err != nil {
		t.Fatalf("get txn for user %s: %v", userID, err)
	}
	if txn.ID != sharedSyncTxnID || txn.AccountID != sharedSyncAccountID {
		t.Fatalf("txn for user %s has id %q account_id %q, want shared sync transaction", userID, txn.ID, txn.AccountID)
	}
	var raw struct {
		Tenant string `json:"tenant"`
	}
	if err := json.Unmarshal(txn.RawJSON, &raw); err != nil {
		t.Fatalf("unmarshal raw JSON for user %s: %v", userID, err)
	}
	if raw.Tenant != wantTenant {
		t.Fatalf("raw JSON tenant for user %s = %q, want %q", userID, raw.Tenant, wantTenant)
	}

	state, err := syncStateRepo.Get(ctx, userID, sharedSyncAccountID)
	if err != nil {
		t.Fatalf("get sync state for user %s: %v", userID, err)
	}
	if state.AccountID != sharedSyncAccountID || state.LastSyncedAt == nil {
		t.Fatalf("sync state for user %s = %+v, want state for shared account", userID, state)
	}
}

type syncTestDatabase struct {
	owner *sql.DB
	app   *sql.DB
}

func newSyncTestDatabase(t *testing.T) syncTestDatabase {
	t.Helper()

	ownerURL := os.Getenv("DATABASE_URL_OWNER")
	appURL := os.Getenv("DATABASE_URL_APP")
	if ownerURL == "" || appURL == "" {
		t.Skip("DATABASE_URL_OWNER and DATABASE_URL_APP are required for integration tests")
	}

	root := syncRepoRoot(t)
	owner := openSyncDB(t, ownerURL)
	runSyncMigrations(t, owner, filepath.Join(root, "migrations"))
	app := openSyncDB(t, appURL)

	t.Cleanup(func() {
		_ = app.Close()
		_ = owner.Close()
	})
	return syncTestDatabase{owner: owner, app: app}
}

func syncRepoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func openSyncDB(t *testing.T, dsn string) *sql.DB {
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

func runSyncMigrations(t *testing.T, db *sql.DB, migrationsPath string) {
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

func seedSyncUser(t *testing.T, db *sql.DB, id int64, email string) domain.UserID {
	t.Helper()

	uniqueEmail := fmt.Sprintf("%d-%s", id, email)
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
	`, id, uniqueEmail, uniqueEmail); err != nil {
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

func cleanupSyncUser(t *testing.T, db *sql.DB, userID domain.UserID) {
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

func assertVisibleRowCount(t *testing.T, db *sql.DB, userID domain.UserID, table, idColumn, id string, want int) {
	t.Helper()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin count tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(context.Background(), `SELECT set_config('app.user_id', $1, true)`, userID.String()); err != nil {
		t.Fatalf("set count user id: %v", err)
	}
	var count int
	query := fmt.Sprintf("SELECT count(*) FROM %s WHERE user_id = $1 AND %s = $2", table, idColumn)
	if err := tx.QueryRowContext(context.Background(), query, userID.Int64(), id).Scan(&count); err != nil {
		t.Fatalf("count %s for user %s: %v", table, userID, err)
	}
	if count != want {
		t.Fatalf("%s rows for user %s = %d, want %d", table, userID, count, want)
	}
}

func assertAppVisibleRowCount(t *testing.T, db *sql.DB, userID domain.UserID, table, idColumn, id string, want int) {
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
	query := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = $1", table, idColumn)
	if err := tx.QueryRowContext(context.Background(), query, id).Scan(&count); err != nil {
		t.Fatalf("count app-visible %s for user %s: %v", table, userID, err)
	}
	if count != want {
		t.Fatalf("app-visible %s for user %s = %d, want %d", table, userID, count, want)
	}
}

func syncDeps(
	accountRepo ports.AccountRepo,
	txnRepo ports.TxnRepo,
	syncStateRepo ports.SyncStateRepo,
	accountName string,
	rawJSON []byte,
	now time.Time,
) Deps {
	return Deps{
		Accounts:   accountRepo,
		Txns:       txnRepo,
		SyncStates: syncStateRepo,
		Tokens:     syncFakeTokenStore{app: "same-app-token", user: "same-user-token"},
		NewAkahuClient: func(string, string) ports.AkahuClient {
			return syncFakeAkahuClient{accountName: accountName, rawJSON: rawJSON}
		},
		Clock: syncFakeClock{now: now},
	}
}

type syncFakeTokenStore struct {
	app  string
	user string
}

func (s syncFakeTokenStore) AkahuTokens(context.Context, domain.UserID) (string, string, error) {
	return s.app, s.user, nil
}

type syncFakeAkahuClient struct {
	accountName string
	rawJSON     []byte
}

func (c syncFakeAkahuClient) ListAccounts(context.Context) ([]ports.RawAccount, error) {
	return []ports.RawAccount{{
		ID:       sharedSyncAccountID,
		Name:     c.accountName,
		Bank:     "ANZ",
		Type:     "CHECKING",
		Currency: "NZD",
	}}, nil
}

func (c syncFakeAkahuClient) FetchTransactions(context.Context, string, time.Time) ([]ports.RawTxn, error) {
	return []ports.RawTxn{{
		ID:            sharedSyncTxnID,
		AccountID:     sharedSyncAccountID,
		PostedAt:      time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		Amount:        "12.34",
		Direction:     "DEBIT",
		AkahuCategory: "testing",
		RawJSON:       c.rawJSON,
	}}, nil
}

type syncFakeClock struct {
	now time.Time
}

func (c syncFakeClock) Now() time.Time {
	return c.now
}
