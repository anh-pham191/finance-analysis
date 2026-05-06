package main

import (
	"database/sql"
	"errors"
	"fmt"

	migrate "github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}
	cmd.AddCommand(newMigrateUpCommand())
	cmd.AddCommand(newMigrateDownCommand())
	return cmd
}

func newMigrateUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, closeFn, err := newMigrator()
			if err != nil {
				return err
			}
			defer closeFn()
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				return fmt.Errorf("migrate up: %w", err)
			}
			return nil
		},
	}
}

func newMigrateDownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, closeFn, err := newMigrator()
			if err != nil {
				return err
			}
			defer closeFn()
			if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				return fmt.Errorf("migrate down: %w", err)
			}
			return nil
		},
	}
}

func newMigrator() (*migrate.Migrate, func(), error) {
	dsn := firstEnv("DATABASE_URL_OWNER", "DATABASE_URL")
	if dsn == "" {
		return nil, nil, errors.New("DATABASE_URL_OWNER or DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open migration database: %w", err)
	}
	closeFn := func() {
		_ = db.Close()
	}
	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("create migration driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir, "postgres", driver)
	if err != nil {
		closeFn()
		return nil, nil, fmt.Errorf("create migrator: %w", err)
	}
	return m, closeFn, nil
}
