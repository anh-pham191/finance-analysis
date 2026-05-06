# M2 — Akahu Ingest

> Spec reference: §5 Akahu sync. Architecture: `docs/architecture/overview.md`. Builds on M1.

## Goal

Pull transactions from Akahu (personal plan, ANZ account) into Postgres. Idempotent — running `finance sync` twice in a row results in no duplicate rows and no lost data.

## Scope

### In
- `internal/ingest/AkahuClient` port:
  - `ListAccounts(ctx) ([]RawAccount, error)`
  - `FetchTransactions(ctx, accountID string, since time.Time) ([]RawTxn, error)`
- `internal/akahu/` HTTP adapter implementing the port. Uses `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN` env vars. Honours pagination.
- `internal/ingest/Sync(ctx, repo, akahu, clock)` use case:
  - For each account known to Akahu, upsert into `accounts` table.
  - For each account, fetch txns since `sync_state.last_synced_at - 24h` (overlap window for late-posting).
  - Upsert transactions (PK = Akahu txn id), preserve `created_at`.
  - Update `sync_state.last_synced_at = clock.Now()`.
- `finance sync` CLI command wiring all of the above.
- Repository methods to support the above (extend M1's repository).
- Loaded `.env` via `godotenv` in dev.

### Out
- Webhook ingestion.
- Westpac (just adding the Akahu connection in the user's Akahu dashboard would work; nothing here is bank-specific).
- Categorisation (M3).

## Prerequisites

- M1 complete and green.
- User has signed up for Akahu personal plan and connected ANZ.
- User has generated `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN`.

## Deliverables

- [ ] `finance sync` populates `accounts` and `transactions` from Akahu.
- [ ] Re-running `finance sync` immediately after produces zero new rows and zero updates beyond `updated_at`/`raw_json`.
- [ ] `sync_state` is correctly maintained per account.
- [ ] Unit tests with in-memory `AkahuClient` fake covering: first sync, incremental sync, late-posting overlap, account creation, idempotency.
- [ ] Integration test for `internal/akahu/` against `httptest.NewServer` fixtures.

## Architecture context

The `AkahuClient` port lives in `internal/ingest/` because that's the consumer. The HTTP adapter in `internal/akahu/` imports `ingest`'s interface and implements it.

`internal/ingest/` knows nothing about HTTP. It works with `[]RawTxn` and translates them to `domain.Transaction` before calling the repository. This translation step is where Akahu-specific quirks (string→decimal, timezone normalisation, direction inference) get isolated.

`clock` is passed in as an interface (`type Clock interface{ Now() time.Time }`) so tests can freeze time. Production clock is `realClock{}.Now()`.

## Test plan (TDD)

1. `internal/ingest/sync_test.go` (with fake client + in-memory repo):
   - First sync inserts accounts and transactions.
   - Second sync with same fake data: no duplicates.
   - New transactions arriving in second sync get inserted.
   - `sync_state` is per-account and updated.
   - Late-posting: a txn dated before `last_synced_at` but inside the overlap window is captured.
2. `internal/akahu/client_test.go`:
   - GET `/transactions/{accountId}` with `start` query param hits expected URL.
   - Pagination: cursor-based fetch loops until `cursor.next == nil`.
   - Auth headers present.
   - Non-2xx response surfaces a wrapped error with body excerpt.
3. `cmd/cli/sync_test.go`:
   - End-to-end with testcontainers Postgres + `httptest` Akahu server.

## Acceptance criteria

- `finance sync` works against the user's real Akahu account (manual smoke test).
- All automated tests pass without internet access.
- No secrets logged. Run with `--verbose` and inspect output.

## Files an agent will touch

```
internal/ingest/{ports.go, sync.go, sync_test.go, mapper.go, mapper_test.go}
internal/akahu/{client.go, client_test.go, types.go, fixtures/...}
internal/storage/postgres/repository.go (extend with sync_state + upsert methods)
internal/storage/postgres/queries.sql (extend)
cmd/cli/{sync.go, sync_test.go}
```

## Pitfalls

- Akahu returns amounts as decimal strings or numbers depending on field — always parse via `decimal.NewFromString` after coercing to string.
- Direction (`DEBIT`/`CREDIT`) is inferred from sign of amount in some Akahu fields. Confirm against current Akahu API docs at implementation time; do not assume.
- Don't bake the Akahu base URL into adapter code as a constant — make it injectable so `httptest` can override.
- Don't log tokens or full raw JSON at INFO. Raw JSON goes to DB only.
