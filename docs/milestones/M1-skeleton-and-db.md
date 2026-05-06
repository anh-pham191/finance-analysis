# M1 — Skeleton & Database

> Spec reference: §3 Architecture, §4 Data model. Architecture: `docs/architecture/overview.md`.

## Goal

Stand up the project skeleton and a working Postgres database with migrated schema and a tested `Repository` implementation. After M1, an agent can write a transaction to Postgres and read it back via the domain types — nothing else.

## Scope

### In
- Repository layout per spec §3.
- `go.mod` initialised at module path `github.com/anhpham/finance-analysis` (or whatever the user's chosen path is — confirm before init).
- `docker-compose.yml` running Postgres 16 on `localhost:5432` with database `finance`, user `finance`, password `finance`.
- `migrations/` with initial SQL migrations for all six tables in spec §4.
- `internal/domain/` types: `Account`, `Transaction`, `Category`, `CategoryAssignment`, `Rule`, `SyncState`, `Direction`, `Money` wrapper around `shopspring/decimal`.
- `internal/domain/ports/Repository` interface with the methods needed in M1 only (account + transaction CRUD; the rest land in their respective milestones).
- `internal/storage/postgres/` implementing `Repository` for accounts and transactions, using `sqlc`-generated queries.
- `cmd/cli/` skeleton with `cobra`, `finance migrate up|down` command using `golang-migrate`.
- `Makefile` with `db-up`, `db-down`, `migrate`, `test`, `test-integration`, `lint` targets.
- `golangci-lint` config (`.golangci.yml`).
- `.env.example`, `.gitignore`.

### Out
- Akahu adapter (M2).
- Categorisation (M3).
- Reporting (M4).
- Anything not strictly needed to insert/read a transaction.

## Prerequisites

- Go 1.23+
- Docker Desktop running
- `migrate` CLI (or vendored Go library — prefer library so `finance migrate` works without external binary).

## Deliverables

- [ ] `docker-compose up -d` brings up Postgres.
- [ ] `make migrate` (or `finance migrate up`) creates all six tables.
- [ ] Round-trip integration test: insert account → insert transaction → read both → values match.
- [ ] Unit tests for domain types (e.g. `Direction` validation, `Money` arithmetic).
- [ ] `golangci-lint run` clean.
- [ ] `go test ./...` green.

## Architecture context

`Repository` is **defined by the consumer**. Even though only `storage/postgres/` implements it, the interface lives in `internal/domain/ports/` (or beside the use case once we have one). This keeps `internal/domain/` free of any storage concern.

`shopspring/decimal` is the chosen money type. Postgres `numeric(14,2)` ↔ `decimal.Decimal`. Never use `float64` for money.

Time columns are `timestamptz`. Use `time.Time` in Go; convert to `Pacific/Auckland` for display only.

## Test plan (TDD)

Write tests in this order — each must fail first, then pass.

1. `internal/domain/direction_test.go` — `ParseDirection("DEBIT")` returns enum; invalid input errors.
2. `internal/domain/money_test.go` — wrapping decimal; addition; comparison; zero value.
3. `internal/storage/postgres/repository_test.go` — uses testcontainers-go (or `docker-compose up` in CI):
   - `InsertAccount` then `GetAccount` returns same fields.
   - `InsertTransaction` upsert: same `id` updates `raw_json` but preserves `created_at`.
   - Query by `account_id` returns transactions ordered by `posted_at desc`.
4. `cmd/cli/migrate_test.go` — invoking `finance migrate up` against a clean DB creates expected tables (assert via `pg_catalog`).

## Acceptance criteria

- All deliverables ticked.
- `make test-integration` passes against fresh Postgres.
- A new agent can clone the repo, run `make db-up && make migrate && make test`, and see green.

## Files an agent will touch

```
go.mod, go.sum
docker-compose.yml
.env.example, .gitignore
.golangci.yml
Makefile
migrations/0001_init_accounts.up.sql
migrations/0001_init_accounts.down.sql
migrations/0002_init_transactions.up.sql
... (one migration per table or one combined initial)
internal/domain/{direction.go,money.go,account.go,transaction.go,category.go,rule.go,sync_state.go}
internal/domain/ports/repository.go
internal/storage/postgres/{repository.go,queries.sql,sqlc.yaml,gen/...}
cmd/cli/{main.go,migrate.go,root.go}
```

## Pitfalls

- Do not put SQL strings in `internal/storage/postgres/repository.go`. Use `sqlc` so SQL lives in `.sql` files and Go calls typed methods.
- Do not import `cobra` from `internal/`. CLI wiring lives in `cmd/cli/`.
- Migrations are not reversible by default in production, but each `*.up.sql` must have a `*.down.sql` for local dev.
