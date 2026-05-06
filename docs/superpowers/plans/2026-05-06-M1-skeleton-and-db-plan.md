# M1 — Skeleton & Database Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a Go hexagonal skeleton with Postgres 16, FORCE RLS, `finance_app` / `finance_owner` roles, sqlc-backed per-aggregate repositories, mandatory `withUserTx`, archtest, and a cobra CLI (`migrate`, `health`, `--version`) so every entity can be written and read scoped by `user_id` with cross-tenant isolation proven by integration tests.

**Architecture:** Migrations run as `finance_owner` (table owner, bypass for DDL only). The application opens pooled connections as `finance_app` (no `BYPASSRLS`). Every repository method uses `withUserTx(ctx, db, userID, fn)` to run inside `BEGIN` → `SET LOCAL app.user_id = '<bigint>'` → `fn` → `COMMIT`. SQL lives in `.sql` files; sqlc generates typed queries. Forbidden imports are enforced by `internal/archtest`. Integration tests use testcontainers-go against Postgres 16.

**Tech Stack:** Go 1.23+, Postgres 16, `github.com/golang-migrate/migrate/v4` (embedded in CLI — see Decision), `github.com/sqlc-dev/sqlc`, `github.com/testcontainers/testcontainers-go`, `github.com/spf13/cobra`, `github.com/shopspring/decimal`, `golangci-lint`, Docker Compose for local dev.

**Inputs:** `docs/milestones/M1-skeleton-and-db.md`, spec §3–4 §11, `docs/architecture/overview.md`, `docs/architecture/security.md`.

**Land this doc:** Branch `feature/m1-plan` off `develop`, PR into `develop`, human review **before** implementation code (per `docs/STATUS.md`).

---

## Decisions locked in this plan

| Topic | Choice | Rationale |
|-------|--------|-----------|
| golang-migrate | **Embed** `migrate` in `finance migrate` via `migrate.NewWithDatabaseInstance` + `postgres.WithInstance` | Same binary CI/dev use; no PATH dependency on CLI |
| sqlc version | Pin in `Makefile` / `go generate` | Reproducible gen |
| Integration DB | testcontainers `postgres:16-alpine` | Matches compose |
| `citext` | `CREATE EXTENSION citext` in first migration | Spec `email citext` |

---

## File map (create/modify)

| Path | Responsibility |
|------|----------------|
| `go.mod`, `go.sum` | Module `github.com/anh-pham191/finance-analysis`, toolchain Go 1.23 |
| `docker-compose.yml` | Postgres 16, mount `init/01-roles.sql`, port 5432 |
| `init/01-roles.sql` | Roles `finance_owner`, `finance_app`; grants for DB created by compose |
| `.env.example` | `DATABASE_URL` templates for owner vs app |
| `config/config.example.yaml` | Placeholder DB URL key names for CLI loading later |
| `.gitignore` | Per M1 brief + `security.md` |
| `.golangci.yml` | Standard linters; enable `goimports` / `govet` / `staticcheck` |
| `Makefile` | `db-up`, `db-down`, `migrate`, `test`, `test-integration`, `lint`, `sqlc`, `help` |
| `.github/workflows/ci.yml` | GitHub Actions CI for unit tests on pushes and PRs |
| `migrations/*.up.sql`, `*.down.sql` | Ordered DDL + RLS + seed |
| `internal/domain/*.go` | Pure types: `UserID`, `Money`, `Direction`, aggregates |
| `internal/ports/*.go` | Per-aggregate interfaces + `Clock` |
| `internal/storage/postgres/` | `withUserTx`, sqlc `queries.sql`, `sqlc.yaml`, generated `gen/`, repo structs |
| `internal/archtest/archtest_test.go` | Import graph assertions |
| `cmd/cli/*.go` | Root, migrate, version, health |

---

## Spec ↔ task coverage (self-review)

| Requirement | Task(s) |
|-------------|---------|
| Layout §3, overview layer map | Tasks 1–3, 20 |
| Tables §4 M1 (DDL + seed + grants) | Tasks 4–12 |
| FORCE RLS, policies | Tasks 4–11 |
| `finance_owner` / `finance_app` | Tasks 2–3, 12 |
| Domain types | Task 13 |
| Per-aggregate ports | Task 14 |
| `withUserTx` | Task 15 |
| sqlc + repos | Tasks 16–17 |
| Cross-tenant + cascade tests §11 | Tasks 15–18 |
| RLS bypass (owner vs app) | Task 19 |
| archtest | Task 20 |
| CLI migrate/version/health | Task 21 |
| Makefile / lint | Task 22 |
| GitHub Actions CI | Task 23 |
| STATUS tracker | Task 24 |

---

### Task 1: Initialise module and empty packages

**Files:**
- Create: `go.mod`
- Create: `internal/domain/doc.go`, `internal/ports/doc.go`, `internal/storage/postgres/doc.go`, `internal/archtest/doc.go`, `cmd/cli/doc.go` (empty package comments only if needed for `go list`)

- [ ] **Step 1: Write `go.mod`**

```text
module github.com/anh-pham191/finance-analysis

go 1.23
```

- [ ] **Step 2: Verify**

Run: `cd /path/to/finance-analysis && go mod tidy`
Expected: completes (may no-op).

- [ ] **Step 3: Commit**

```bash
git checkout develop && git pull
git checkout -b feature/m1-skeleton-db   # implementation branch after plan merged
git add go.mod
git commit -m "chore: initialise Go module"
```

---

### Task 2: Docker Compose and database roles

**Files:**
- Create: `docker-compose.yml`
- Create: `init/01-roles.sql`

- [ ] **Step 1: Add compose**

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: finance_owner
      POSTGRES_PASSWORD: finance_owner_dev
      POSTGRES_DB: finance
    ports:
      - "5432:5432"
    volumes:
      - ./init:/docker-entrypoint-initdb.d
```

- [ ] **Step 2: Add role bootstrap** (`init/01-roles.sql`)

Run after DB exists (init scripts run as superuser):

```sql
-- Application role: connects from app; must not own tables (owner bypasses RLS without FORCE — we use FORCE anyway; still no ownership).
CREATE ROLE finance_app LOGIN PASSWORD 'finance_app_dev';

GRANT CONNECT ON DATABASE finance TO finance_app;
GRANT USAGE ON SCHEMA public TO finance_app;
-- Table grants added after migrations create objects (follow-up migration or second init script).
```

Init scripts run before any migration; table privileges cannot exist yet. **`finance_app` grants are applied in migration `0009` (Task 12)** after all tables and sequences exist.

- [ ] **Step 3: Run compose**

Run: `docker compose up -d`
Expected: healthy Postgres.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml init/01-roles.sql
git commit -m "chore: add local Postgres 16 and finance_app role"
```

---

### Task 3: Repository tooling files

**Files:**
- Create: `.gitignore`, `.golangci.yml`, `.env.example`, `config/config.example.yaml`

- [ ] **Step 1: `.gitignore`** (minimum)

```
.env
.env.local
config/config.yaml
*.dump
bin/
dist/
```

- [ ] **Step 2: `.env.example`**

```
# Owner URL — golang-migrate CLI / finance migrate
DATABASE_URL_OWNER=postgres://finance_owner:finance_owner_dev@localhost:5432/finance?sslmode=disable

# App URL — integration tests & future app default
DATABASE_URL_APP=postgres://finance_app:finance_app_dev@localhost:5432/finance?sslmode=disable
```

- [ ] **Step 3: `config/config.example.yaml`**

```yaml
database:
  owner_url: "postgres://finance_owner:finance_owner_dev@localhost:5432/finance?sslmode=disable"
  app_url: "postgres://finance_app:finance_app_dev@localhost:5432/finance?sslmode=disable"
```

- [ ] **Step 4: Commit**

```bash
git add .gitignore .golangci.yml .env.example config/config.example.yaml
git commit -m "chore: add ignore rules, lint config, env templates"
```

---

### Task 4: Migration 0001 — extensions + users

**Files:**
- Create: `migrations/000001_extensions_users.up.sql`
- Create: `migrations/000001_extensions_users.down.sql`

- [ ] **Step 1: Write up migration**

The `users` table has **no `user_id` column** (spec §4). Tenant scope for this table is **`users.id = current_setting('app.user_id')`**.

```sql
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id bigserial PRIMARY KEY,
  email citext NOT NULL UNIQUE,
  display_name text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX users_email_idx ON users (email);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

CREATE POLICY users_self_access ON users
  USING (id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Write down migration**

```sql
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS citext;
```

(If other migrations depend on citext, only drop citext in final teardown — for isolated M1 down order, drop users first; keep citext until last migration's down.)

- [ ] **Step 3: Commit**

```bash
git add migrations/000001_extensions_users.up.sql migrations/000001_extensions_users.down.sql
git commit -m "feat(db): add users table with RLS"
```

---

### Task 5: Migration 0002 — accounts

**Files:**
- Create: `migrations/000002_accounts.up.sql`, `migrations/000002_accounts.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE accounts (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  id text NOT NULL,
  name text NOT NULL,
  bank text NOT NULL DEFAULT '',
  type text NOT NULL DEFAULT '',
  currency text NOT NULL DEFAULT 'NZD',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, id)
);

CREATE INDEX accounts_user_id_idx ON accounts (user_id);

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

CREATE POLICY accounts_tenant_isolation ON accounts
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS accounts;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000002_accounts.*.sql
git commit -m "feat(db): add accounts with composite PK and RLS"
```

---

### Task 6: Migration 0003 — transactions

**Files:**
- Create: `migrations/000003_transactions.up.sql`, `migrations/000003_transactions.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE transactions (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  id text NOT NULL,
  account_id text NOT NULL,
  posted_at timestamptz NOT NULL,
  amount numeric(14,2) NOT NULL,
  direction text NOT NULL CHECK (direction IN ('DEBIT','CREDIT')),
  description text NOT NULL DEFAULT '',
  merchant text NOT NULL DEFAULT '',
  akahu_category text NOT NULL DEFAULT '',
  raw_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, id),
  FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id) ON DELETE CASCADE
);

CREATE INDEX transactions_user_posted_idx ON transactions (user_id, posted_at DESC);

ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;

CREATE POLICY transactions_tenant_isolation ON transactions
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS transactions;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000003_transactions.*.sql
git commit -m "feat(db): add transactions with composite FK and RLS"
```

---

### Task 7: Migration 0004 — categories

**Files:**
- Create: `migrations/000004_categories.up.sql`, `migrations/000004_categories.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE categories (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  parent_id bigint REFERENCES categories(id) ON DELETE SET NULL,
  kind text NOT NULL CHECK (kind IN ('income','expense','transfer')),
  UNIQUE (user_id, name)
);

CREATE INDEX categories_user_id_idx ON categories (user_id);

ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories FORCE ROW LEVEL SECURITY;

CREATE POLICY categories_tenant_isolation ON categories
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS categories;
```

- [ ] **Step 3: Commit**

---

### Task 8: Migration 0005 — rules

**Files:**
- Create: `migrations/000005_rules.up.sql`, `migrations/000005_rules.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE rules (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  priority int NOT NULL,
  predicate jsonb NOT NULL DEFAULT '{}'::jsonb,
  category_id bigint NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, name)
);

CREATE INDEX rules_user_id_idx ON rules (user_id);

ALTER TABLE rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE rules FORCE ROW LEVEL SECURITY;

CREATE POLICY rules_tenant_isolation ON rules
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS rules;
```

- [ ] **Step 3: Commit**

---

### Task 9: Migration 0006 — category_assignments

**Files:**
- Create: `migrations/000006_category_assignments.up.sql`, `migrations/000006_category_assignments.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE category_assignments (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  txn_id text NOT NULL,
  category_id bigint NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
  source text NOT NULL CHECK (source IN ('RULE','MANUAL','AKAHU')),
  rule_id bigint REFERENCES rules(id) ON DELETE SET NULL,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, txn_id),
  FOREIGN KEY (user_id, txn_id) REFERENCES transactions(user_id, id) ON DELETE CASCADE
);

CREATE INDEX category_assignments_user_idx ON category_assignments (user_id);

ALTER TABLE category_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE category_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY category_assignments_tenant_isolation ON category_assignments
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS category_assignments;
```

- [ ] **Step 3: Commit**

---

### Task 10: Migration 0007 — sync_state

**Files:**
- Create: `migrations/000007_sync_state.up.sql`, `migrations/000007_sync_state.down.sql`

- [ ] **Step 1: Up**

```sql
CREATE TABLE sync_state (
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id text NOT NULL,
  last_synced_at timestamptz,
  last_cursor text,
  PRIMARY KEY (user_id, account_id),
  FOREIGN KEY (user_id, account_id) REFERENCES accounts(user_id, id) ON DELETE CASCADE
);

ALTER TABLE sync_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_state FORCE ROW LEVEL SECURITY;

CREATE POLICY sync_state_tenant_isolation ON sync_state
  USING (user_id = current_setting('app.user_id', true)::bigint)
  WITH CHECK (user_id = current_setting('app.user_id', true)::bigint);
```

- [ ] **Step 2: Down**

```sql
DROP TABLE IF EXISTS sync_state;
```

- [ ] **Step 3: Commit**

---

### Task 11: Migration 0008 — seed dev user + Uncategorised category

**Files:**
- Create: `migrations/000008_seed_local_user.up.sql`, `migrations/000008_seed_local_user.down.sql`

**Note:** With `FORCE ROW LEVEL SECURITY`, sessions obey policies even when connected as the table owner (unless `BYPASSRLS`). The migration runner must set `app.user_id` in the same transaction as inserts into tenant-scoped tables.

- [ ] **Step 1: Up**

```sql
BEGIN;
SELECT set_config('app.user_id', '1', true);

INSERT INTO users (id, email, display_name)
OVERRIDING SYSTEM VALUE
VALUES (1, 'local@finance-analysis', 'Local Dev')
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email,
  display_name = EXCLUDED.display_name;

SELECT setval(pg_get_serial_sequence('users', 'id'), (SELECT COALESCE(MAX(id), 1) FROM users));

INSERT INTO categories (user_id, name, kind)
VALUES (1, 'Uncategorised', 'expense')
ON CONFLICT (user_id, name) DO NOTHING;

COMMIT;
```

- [ ] **Step 2: Down**

```sql
DELETE FROM categories WHERE user_id = 1 AND name = 'Uncategorised';
DELETE FROM users WHERE id = 1;
```

(Order respects FKs — categories first.)

- [ ] **Step 3: Commit**

---

### Task 12: Migration 0009 — grants to finance_app

**Files:**
- Create: `migrations/000009_grants_finance_app.up.sql`, `migrations/000009_grants_finance_app.down.sql`

- [ ] **Step 1: Up**

```sql
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO finance_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO finance_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO finance_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO finance_app;
```

- [ ] **Step 2: Down**

Revoke mirrors (optional for dev).

- [ ] **Step 3: Commit**

---

### Task 13: Domain types (TDD)

**Files:**
- Create: `internal/domain/user_id.go`, `direction.go`, `money.go`, `account.go`, `transaction.go`, `category.go`, `rule.go`, `assignment.go`, `sync_state.go`
- Create: `internal/domain/direction_test.go`, `money_test.go`, `user_id_test.go`

- [ ] **Step 1: Write failing tests** for `Direction` parsing (`DEBIT`/`CREDIT`), `Money` helpers wrapping `decimal.Decimal`, `UserID` string conversion / validation if any.

- [ ] **Step 2: Run**

Run: `go test ./internal/domain/... -v`
Expected: FAIL until types implemented.

- [ ] **Step 3: Implement** minimal structs matching DB columns and spec naming.

- [ ] **Step 4: Run until PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add core types and unit tests"
```

---

### Task 14: Ports (per-aggregate interfaces)

**Files:**
- Create: `internal/ports/account.go`, `txn.go`, `category.go`, `rule.go`, `assignment.go`, `sync_state.go`, `clock.go`

- [ ] **Step 1: Define interfaces** — each method: `(ctx context.Context, userID domain.UserID, ...)`.

**Minimal methods for M1** (extend later):

```go
// Example signatures — align names across impl:
type AccountRepo interface {
  Upsert(ctx context.Context, userID domain.UserID, a domain.Account) error
  Get(ctx context.Context, userID domain.UserID, akahuID string) (domain.Account, error)
}
```

Repeat pattern for `TxnRepo`, `CategoryRepo` (GetByName, Insert for tests), `RuleRepo`, `AssignmentRepo`, `SyncStateRepo`.

```go
type Clock interface {
  Now() time.Time
}
```

- [ ] **Step 2: `go test ./internal/ports/...`** — compile-only packages may use `go test -c` or a trivial `doc.go` test.

- [ ] **Step 3: Commit**

---

### Task 15: sqlc configuration and `withUserTx`

**Files:**
- Create: `internal/storage/postgres/sqlc.yaml`
- Create: `internal/storage/postgres/queries.sql` (start with `SetUserID` — actually `SET LOCAL` is not sqlc; keep in Go)
- Create: `internal/storage/postgres/withusertx.go`
- Create: `internal/storage/postgres/withusertx_test.go`

- [ ] **Step 1: Write failing integration test** `withusertx_test.go` (build tag `integration`):

  - Start postgres container.
  - Connect as `finance_app`.
  - Migrate DB (use migrate programmatically or exec migrate with owner URL from container exec — **pattern:** test starts container, runs migrations using **owner** URL from env, opens **app** pool for assertions).
  - Seed two users in test (user 1 may exist from migration; insert user 2 with `SET LOCAL` via owner connection or insert as superuser for seed only).
  - Assert `withUserTx(ctx, pool, user1, fn)` only sees user 1 rows.

- [ ] **Step 2: Implement `withUserTx`:**

```go
func withUserTx(ctx context.Context, db *sql.DB, userID domain.UserID, fn func(ctx context.Context, tx *sql.Tx) error) error {
  tx, err := db.BeginTx(ctx, nil)
  if err != nil { return err }
  defer tx.Rollback()
  if _, err := tx.ExecContext(ctx, `SET LOCAL app.user_id = $1`, userID.String()); err != nil { return err }
  if err := fn(ctx, tx); err != nil { return err }
  return tx.Commit()
}
```

Adjust `UserID` API to match domain type.

- [ ] **Step 3: Run integration tests**

Run: `go test -tags=integration ./internal/storage/postgres/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

---

### Task 16: sqlc queries + Account repo + tests

**Files:**
- Modify: `queries.sql`, `sqlc.yaml`
- Create: `internal/storage/postgres/account_repo.go`
- Create: `internal/storage/postgres/account_repo_test.go` (integration)

- [ ] **Step 1: Add SQL** for upsert/select account by PK.

- [ ] **Step 2: Run** `sqlc generate`

- [ ] **Step 3: Implement** `PostgresAccountRepo` calling `withUserTx` for each method.

- [ ] **Step 4: Integration test** — round-trip; cross-tenant read returns `sql.ErrNoRows` or empty; attempt to upsert row with wrong `user_id` fails.

- [ ] **Step 5: Commit**

---

### Task 17: Remaining repos (Txn, Category, Rule, Assignment, SyncState)

**Files:**
- Create: one `*_repo.go` + `*_repo_test.go` per aggregate

- [ ] **For each aggregate:** duplicate Task 16 pattern — sqlc queries, repo struct, `withUserTx`, integration tests including cross-tenant and cascade delete from `users`.

- [ ] **Commit per aggregate** or single commit if batching:

```bash
git commit -m "feat(storage): implement postgres repos for all M1 aggregates"
```

---

### Task 18: Cascade delete integration test

**Files:**
- Create: `internal/storage/postgres/cascade_test.go` (integration)

- [ ] **Step 1: Test** inserts user B graph, deletes user B via owner connection `DELETE FROM users WHERE id = $1`, asserts no orphan rows (counts).

- [ ] **Step 2: Commit**

---

### Task 19: RLS bypass `finance_owner` vs `finance_app`

**Files:**
- Create: `internal/storage/postgres/rls_bypass_test.go` (integration)

- [ ] **Step 1: Owner connection** — query without `SET LOCAL` should still see rows (owner bypass — **verify PG version**: with FORCE, test expected behaviour from Postgres docs).

- [ ] **Step 2: App connection** — query without `SET LOCAL` returns zero rows for tenant tables.

- [ ] **Step 3: Document result** in test comment — anchors threat model.

- [ ] **Step 4: Commit**

---

### Task 20: `internal/archtest`

**Files:**
- Create: `internal/archtest/archtest_test.go`

- [ ] **Step 1: Implement test** running `go list -json` / parsing imports for packages under `./internal/...` — forbid patterns from `overview.md`:

  - `internal/domain` importing non-standard internal packages
  - `internal/ports` importing anything except `internal/domain`
  - `internal/{ingest,categorise,report}` — not present yet; skip or future-proof
  - etc.

Use `go list -deps -json ./internal/...` and assert forbidden edges.

- [ ] **Step 2: One-time experiment** — add forbidden import on branch, confirm failure, revert — **do not merge** experiment.

- [ ] **Step 3: Commit**

---

### Task 21: CLI — root, version, migrate, health

**Files:**
- Create: `cmd/cli/main.go`, `root.go`, `version.go`, `migrate.go`, `health.go`
- Create: `cmd/cli/migrate_test.go`, `cmd/cli/health_test.go`

- [ ] **Step 1: `main.go`** calls `Execute()` on root command.

- [ ] **Step 2: `version`** — ldflags `-X main.version=` from Makefile; prints semver.

- [ ] **Step 3: `migrate`** — loads owner DSN from env `DATABASE_URL` / flag; runs embedded migrate up/down.

- [ ] **Step 4: `health`** — opens app DSN, `db.PingContext`; exit 0 / 1.

- [ ] **Step 5: Tests** — migrate_test uses testcontainers; health_test optional skip if no docker.

- [ ] **Step 6: Commit**

---

### Task 22: Makefile and CI verification targets

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Targets**

```makefile
.PHONY: db-up db-down migrate test test-integration lint sqlc help

db-up:
	docker compose up -d

db-down:
	docker compose down

migrate:
	go run ./cmd/cli migrate up

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

sqlc:
	sqlc generate -f internal/storage/postgres/sqlc.yaml

help:
	@echo "targets: db-up db-down migrate test test-integration lint sqlc"
```

Adjust migrate to pass DATABASE_URL.

- [ ] **Step 2: Run full suite**

Run: `make db-up && make migrate && make test && make test-integration && make lint`
Expected: all green.

- [ ] **Step 3: Commit**

---

### Task 23: GitHub Actions CI for unit tests

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Add workflow**

This is CI only. There is no CD/deployment step in M1 because the project has no deployable service yet.

```yaml
name: CI

on:
  push:
    branches:
      - main
      - develop
  pull_request:
    branches:
      - develop

permissions:
  contents: read

jobs:
  unit:
    name: Unit tests
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Run unit tests
        run: make test
```

- [ ] **Step 2: Verify workflow locally as far as possible**

Run: `make test`
Expected: PASS.

Run: `git diff --check .github/workflows/ci.yml`
Expected: no whitespace errors.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run unit tests in GitHub Actions"
```

---

### Task 24: Update `docs/STATUS.md` milestone tracker

**Files:**
- Modify: `docs/STATUS.md`

- [ ] **Step 1:** Set M1 **Plan** column to done; **Implementation** to in progress when coding starts.

- [ ] **Step 2: Commit** (docs-only PR acceptable).

---

## Placeholder scan (self-review)

- `users` RLS uses `id = app.user_id`, not a `user_id` column.
- Seed migration uses one transaction with `set_config` + `OVERRIDING SYSTEM VALUE` for `id = 1`.
- golang-migrate filename pattern: use the tool’s expected convention (e.g. `000001_name.up.sql`); align `migrate` driver config with on-disk names.

---

## Execution handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-06-M1-skeleton-and-db-plan.md`. Two execution options:**

**1. Subagent-driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration. **Required sub-skill:** `superpowers:subagent-driven-development`.

**2. Inline execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

**Which approach do you want?**

---

## Next step for this repository (plan-only PR)

1. `git checkout develop && git pull`
2. `git checkout -b feature/m1-plan`
3. Add this file under `docs/superpowers/plans/`
4. Open PR to `develop`; human reviews plan before any implementation branch.
