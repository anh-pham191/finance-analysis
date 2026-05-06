# M1 — Skeleton & Database

> Spec reference: §3 Architecture, §4 Data model, §11 Testing. Architecture: `docs/architecture/overview.md`. Security: `docs/architecture/security.md`.

## Goal

Stand up the project skeleton and a working Postgres database with the full multi-tenant schema, FORCED RLS, a non-owner application role, and tested per-aggregate repository implementations. After M1, an agent can write a transaction (and every other entity) for a given user to Postgres and read it back via the domain types — and any cross-tenant attempt fails.

This is the foundation every later milestone depends on; getting RLS, roles, and the `withUserTx` pattern right here is the single most important thing.

## Scope

### In
- Repository layout per spec §3 with `internal/ports/`, `internal/archtest/`, no `internal/akahu/` yet.
- `go.mod` initialised at module path `github.com/anh-pham191/finance-analysis`.
- `docker-compose.yml` running Postgres 16; two roles created in init SQL: `finance_owner` (migrations) and `finance_app` (application; **no `BYPASSRLS`**).
- `migrations/` with initial migrations for: `users`, `accounts`, `transactions`, `categories`, `category_assignments`, `rules`, `sync_state`. **`akahu_tokens` is NOT in M1** — added in M8a.
  - All tables: `user_id` column, FORCE RLS, isolation policy, `ON DELETE CASCADE` from `users`.
  - Composite PKs `(user_id, id)` on `accounts` and `transactions`.
  - Composite FKs from `transactions(user_id, account_id)`, `category_assignments(user_id, txn_id)`, `sync_state(user_id, account_id)`.
  - Seed: `users(id=1, email='local@finance-analysis')` + `categories(user_id=1, name='Uncategorised', kind='expense')`.
- `internal/domain/` types: `UserID`, `Account`, `Transaction`, `Category`, `CategoryAssignment`, `Rule`, `SyncState`, `Direction`, `Money` (around `shopspring/decimal`).
- `internal/ports/`: per-aggregate interfaces (`AccountRepo`, `TxnRepo`, `CategoryRepo`, `RuleRepo`, `AssignmentRepo`, `SyncStateRepo`) and `Clock`. Each method takes `userID UserID` first.
- `internal/storage/postgres/`:
  - `withUserTx(ctx, db, userID, fn)` helper that opens a tx, `SET LOCAL app.user_id = $userID`, runs `fn`, commits.
  - `sqlc`-generated queries.
  - One repo struct per port; every method body delegates to `withUserTx`.
- `internal/archtest/archtest_test.go`: walks the import graph; fails if any forbidden import appears.
- `cmd/cli/`: `cobra` skeleton; commands `migrate up|down`, `--version`, `health` (DB ping; in M1 there is no Akahu yet, so `health` only pings DB).
- `Makefile`: `db-up`, `db-down`, `migrate`, `test`, `test-integration`, `lint`, `help`.
- `.golangci.yml`, `.gitignore` (covers `.env`, `config/*.yaml`, real fixtures, dumps).
- `.env.example`, `config/config.example.yaml`. **No `categories.example.yaml` or `rules.example.yaml` yet** — those land in M3 with the loaders.

### Out
- Akahu adapter (M2).
- Categorisation, rule engine, yaml loaders (M3).
- Reporting (M4).
- Token storage and the `akahu_tokens` table (M8a).

## Prerequisites

- Go 1.23+
- Docker Desktop running

## Deliverables

- [ ] `docker-compose up -d` brings up Postgres with `finance_owner` and `finance_app` roles.
- [ ] `make migrate` runs migrations as `finance_owner` and creates all tables, RLS policies, seed user + Uncategorised category.
- [ ] All tests pass with `make test` (unit) and `make test-integration` (testcontainers).
- [ ] Each port has a Postgres impl with at least one method per aggregate exercised by integration tests.
- [ ] `internal/archtest` test fails CI when a deliberate forbidden-import is added (verified by a one-time experiment, then reverted).
- [ ] Cross-tenant test seeded: two users, every M1 method confirmed scoped — user A cannot read or write user B's rows even when crafting raw queries (RLS rejects).
- [ ] `RLS bypass test`: connecting as `finance_owner` (which has BYPASSRLS implicitly via ownership) confirms the bypass — and a separate test confirms `finance_app` cannot bypass. Documents the threat model concretely.
- [ ] `finance --version` prints version. `finance health` returns 0 on healthy DB, non-zero otherwise.

## Architecture context

**RLS is the load-bearing security control here.** Read `docs/architecture/security.md` "Multi-tenancy from day 1" before touching the migrations. Specifically:
- `FORCE ROW LEVEL SECURITY` on every table — without it, the owner bypasses.
- The app connects as `finance_app`, not `finance_owner`. Migrations are the only operations that need owner.
- `SET LOCAL app.user_id` only persists for the current tx. Every repo method must run inside a `BEGIN/COMMIT`. The `withUserTx` helper is the single enforcement point — a repo method that sidesteps it is a bug.

Per-aggregate repos (rather than a single `Repository`) keeps each interface small and each in-memory fake easy to write.

## Test plan (TDD)

Write tests in this order, each must fail first.

1. **`internal/domain/`** — direction parsing, money arithmetic, `UserID` type behaviour.
2. **`internal/storage/postgres/withusertx_test.go`** — given two users, `withUserTx(userA)` only sees user A's seeded rows; `withUserTx(userB)` sees user B's; raw `SELECT` outside `withUserTx` returns zero rows (because RLS rejects without `app.user_id`).
3. **`internal/storage/postgres/<aggregate>_test.go`** for each aggregate:
   - Round-trip: insert, read.
   - Cross-tenant: insert as user A, read as user B, expect empty/error. Try to write user A's id from user B's session, expect failure.
   - Cascade: delete user, all children gone.
4. **`internal/archtest/archtest_test.go`** — runs `go list -deps` for each `internal/...` package, asserts no forbidden import.
5. **`cmd/cli/migrate_test.go`** — `finance migrate up` against a clean DB creates expected tables + seed rows.
6. **`cmd/cli/health_test.go`** — exit code 0 with healthy DB, non-zero with the DB stopped (skipped in CI; manual smoke).

## Acceptance criteria

- All deliverables ticked.
- A new agent on a clean machine: `git clone && make db-up && make migrate && make test test-integration` → green.
- Cross-tenant test patterns are templated and ready for M2 to copy.

## Files an agent will touch

```
go.mod, go.sum
docker-compose.yml
init/01-roles.sql                      # creates finance_owner + finance_app
.env.example, .gitignore, .golangci.yml
Makefile
config/config.example.yaml
migrations/0001_users.up.sql / .down.sql
migrations/0002_accounts.up.sql / .down.sql
migrations/0003_transactions.up.sql / .down.sql
migrations/0004_categories.up.sql / .down.sql
migrations/0005_rules.up.sql / .down.sql
migrations/0006_category_assignments.up.sql / .down.sql
migrations/0007_sync_state.up.sql / .down.sql
migrations/0008_seed_dev_user.up.sql / .down.sql
internal/domain/{user_id,direction,money,account,transaction,category,rule,sync_state,assignment}.go
internal/ports/{account,txn,category,rule,assignment,sync_state,clock}.go
internal/storage/postgres/{withusertx.go,account.go,txn.go,category.go,rule.go,assignment.go,sync_state.go,queries.sql,sqlc.yaml,gen/}
internal/archtest/archtest_test.go
cmd/cli/{root.go,migrate.go,version.go,health.go}
```

## Pitfalls

- **Don't grant `finance_app` ownership of any table.** Owners bypass RLS. If your migration says `ALTER TABLE ... OWNER TO finance_app`, you've broken the model.
- **Don't run repo methods outside `withUserTx`.** `SET LOCAL` outside a tx silently does nothing.
- **Don't add `akahu_tokens` to M1** — YAGNI. The table is M8a's responsibility, designed against real rotation requirements.
- **Don't omit `FORCE ROW LEVEL SECURITY`.** Without it, even a non-owner can be granted bypass via `BYPASSRLS`. Belt-and-braces.
- **Don't put SQL strings in repo Go files.** SQL goes in `.sql` files; `sqlc` generates the Go.
- **Don't import `cobra` from `internal/`.** CLI wiring lives in `cmd/cli/`.
- **Migrations: each `*.up.sql` must have a working `*.down.sql`** for local dev iteration.
