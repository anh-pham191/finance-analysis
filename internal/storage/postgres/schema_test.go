//go:build integration

package postgres

import (
	"context"
	"testing"
)

func TestDatabaseRolesAreNotSuperuserOrBypassRLS(t *testing.T) {
	db := newTestDatabase(t)

	for _, role := range []string{"finance_owner", "finance_app"} {
		var superuser bool
		var bypassRLS bool
		if err := db.owner.QueryRowContext(context.Background(), `
			SELECT rolsuper, rolbypassrls
			FROM pg_roles
			WHERE rolname = $1
		`, role).Scan(&superuser, &bypassRLS); err != nil {
			t.Fatalf("query role %s: %v", role, err)
		}
		if superuser || bypassRLS {
			t.Fatalf("%s rolsuper=%t rolbypassrls=%t, want both false", role, superuser, bypassRLS)
		}
	}
}

func TestUserOwnedTablesHaveForcedRLS(t *testing.T) {
	db := newTestDatabase(t)

	tables := []string{
		"users",
		"accounts",
		"transactions",
		"categories",
		"rules",
		"category_assignments",
		"sync_state",
	}
	for _, table := range tables {
		var rlsEnabled bool
		var rlsForced bool
		if err := db.owner.QueryRowContext(context.Background(), `
			SELECT relrowsecurity, relforcerowsecurity
			FROM pg_class
			WHERE oid = $1::regclass
		`, table).Scan(&rlsEnabled, &rlsForced); err != nil {
			t.Fatalf("query RLS for %s: %v", table, err)
		}
		if !rlsEnabled || !rlsForced {
			t.Fatalf("%s relrowsecurity=%t relforcerowsecurity=%t, want both true", table, rlsEnabled, rlsForced)
		}
	}
}
