# M2 — Akahu Ingest

> Spec reference: §5 Akahu sync. Architecture: `docs/architecture/overview.md`. Builds on M1.

## Goal

Pull transactions from Akahu (personal plan, ANZ account) into Postgres. Idempotent — running `finance sync` twice in a row results in no duplicate rows and no lost data.

## Scope

### In
- `internal/ingest/AkahuClient` port:
  - `ListAccounts(ctx) ([]RawAccount, error)`
  - `FetchTransactions(ctx, accountID string, since time.Time) ([]RawTxn, error)`
- `internal/ingest/TokenStore` port:
  - `AkahuTokens(ctx, userID) (app, user string, err error)`
- `internal/akahu/` HTTP adapter implementing `AkahuClient`. Constructed with explicit `appToken, userToken string` — does not read env directly. Honours pagination.
- `internal/akahu/EnvTokenStore` implementing `TokenStore` by reading `AKAHU_APP_TOKEN` / `AKAHU_USER_TOKEN`. Returns the same tokens regardless of `userID` (acceptable while the only user is `id=1`).
- `internal/ingest/Sync(ctx, userID, repo, tokenStore, akahuFactory, clock)` use case:
  - Resolve the user's Akahu tokens via `TokenStore`.
  - Build an `AkahuClient` from those tokens via `akahuFactory`.
  - For each account known to Akahu, upsert into `accounts` (with `user_id = userID`).
  - For each account, fetch txns since `sync_state.last_synced_at - 24h` (overlap window for late-posting).
  - Upsert transactions (PK = Akahu txn id, scoped by `user_id` in the `WHERE`), preserve `created_at`.
  - Update `sync_state.last_synced_at = clock.Now()` for `(user_id, account_id)`.
- `finance sync` CLI command wiring all of the above. Resolves dev user (`UserID(1)`).
- Repository methods to support the above (extend M1's repository), all `userID`-scoped.
- Loaded `.env` via `godotenv` in dev.
- Slog redaction filter for token-shaped attributes — verified by a unit test that asserts a token logged at DEBUG comes out as `***`.

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

The `AkahuClient` and `TokenStore` ports live in `internal/ingest/` because that's the consumer. The HTTP adapter and `EnvTokenStore` in `internal/akahu/` import `ingest`'s interfaces and implement them.

`internal/ingest/` knows nothing about HTTP. It works with `[]RawTxn` and translates them to `domain.Transaction` (with `user_id` set) before calling the repository. This translation step is where Akahu-specific quirks (string→decimal, timezone normalisation, direction inference) get isolated.

The `TokenStore` indirection is the key piece for M8: swapping `EnvTokenStore` for `DBTokenStore` then is a wiring change, not a use-case change. Don't shortcut by reading env from inside the use case.

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
