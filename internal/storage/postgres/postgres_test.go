//go:build integration

package postgres

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	migrate "github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type testDatabase struct {
	owner *sql.DB
	app   *sql.DB
}

func newTestDatabase(t *testing.T) testDatabase {
	t.Helper()

	ownerURL := os.Getenv("DATABASE_URL_OWNER")
	appURL := os.Getenv("DATABASE_URL_APP")
	if ownerURL == "" || appURL == "" {
		t.Skip("DATABASE_URL_OWNER and DATABASE_URL_APP are required for integration tests")
	}

	root := repoRoot(t)
	owner := openDB(t, ownerURL)
	runMigrations(t, owner, filepath.Join(root, "migrations"))
	app := openDB(t, appURL)

	t.Cleanup(func() {
		_ = app.Close()
		_ = owner.Close()
	})

	return testDatabase{owner: owner, app: app}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}

func openDB(t *testing.T, dsn string) *sql.DB {
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

func runMigrations(t *testing.T, db *sql.DB, migrationsPath string) {
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
