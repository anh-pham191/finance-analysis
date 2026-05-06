# M2 — Akahu Ingest

> Spec reference: §5 Akahu sync, §8 secrets. Architecture: `docs/architecture/security.md`. Builds on M1.

## Goal

Pull transactions from Akahu (personal plan) into Postgres, idempotently, with safe retry/backoff and zero token leakage. By end of M2, `finance sync` works against the user's real ANZ account and — as a smoke-test acceptance — against Westpac too once the user adds it in their Akahu dashboard.

## Scope

### In
- `internal/ports/AkahuClient`:
  - `ListAccounts(ctx) ([]RawAccount, error)`
  - `FetchTransactions(ctx, accountID string, since time.Time) ([]RawTxn, error)` (handles pagination internally)
- `internal/ports/TokenStore` — already declared in M1 if not; otherwise add here.
- `internal/akahu/`:
  - `Client` implementing `AkahuClient`. Constructed with explicit `appToken, userToken, baseURL string` — does not read env directly.
  - **Retry policy:** `429` and `5xx` retried up to 3 times with exponential backoff `1s, 2s, 4s` ±25% jitter; honours `Retry-After`; respects `ctx`.
  - **Pagination:** cursor-loop until `next == ""`.
  - `EnvTokenStore` implementing `TokenStore` — reads `AKAHU_APP_TOKEN`, `AKAHU_USER_TOKEN`. Returns same tokens regardless of `userID` (intentional MVP weakening, called out in code comment + test).
- `internal/ingest/Sync(ctx, userID, deps)` use case:
  - Resolve tokens via `TokenStore`.
  - Build `AkahuClient` from tokens (`akahuFactory` injected).
  - List accounts → upsert.
  - For each account: fetch since `max(sync_state.last_synced_at - 24h, now - 30d if first sync)`.
  - Upsert transactions via `(user_id, id)` conflict target.
  - Update `sync_state` for `(user_id, account_id)` with `last_synced_at = clock.Now()`.
- `cmd/cli/sync.go` — `finance sync [--from DATE]` wired to dev user (`UserID(1)`). `--from` overrides the 30d default.
- Slog redaction filter (key-side + value-side per security doc) — wired in CLI bootstrap, tested.
- `finance health` (already in M1) extended to include an Akahu reachability check.

### Out
- Webhook ingestion (future).
- Categorisation (M3).

## Prerequisites

- M1 complete and green.
- User has Akahu personal-plan tokens. ANZ connected.

## Deliverables

- [ ] `finance sync` populates `accounts` and `transactions` from Akahu (real smoke-test).
- [ ] **Westpac smoke-test acceptance:** user connects Westpac in their Akahu dashboard, re-runs `finance sync`, Westpac txns appear with no code change. M2 acceptance includes this verification step.
- [ ] Re-running `finance sync` immediately produces zero new rows; `updated_at` may move on `transactions` if Akahu re-enriched, but `created_at`, `amount`, `posted_at` are preserved.
- [ ] First sync (no `sync_state` row) defaults to 30d lookback; `--from 2024-01-01` override works.
- [ ] Retry test: stub server returns 429 twice then 200; client succeeds. Stub returns 500 four times; client errors after 3 retries.
- [ ] **Token-redaction tests** — both pass:
  - Slog: emit a structured log with key `app_token` and a real-shaped value; captured output contains `***`.
  - Error wrapping: stub returns body containing `Authorization: Bearer abc123def...`; the resulting wrapped error string does NOT contain `abc123def...`.
- [ ] **`raw_json` redaction test:** force a parse error after Akahu returns a row; the resulting error chain contains the txn id but no `raw_json` content.
- [ ] Cross-tenant test: seed `EnvTokenStore` to return user A's tokens; call `Sync(ctx, userIDA)` and `Sync(ctx, userIDB)`; assert each user only sees their own rows. (Confirms RLS isolation extends through the ingest path even with the shared-token EnvTokenStore.)

## Architecture context

The `EnvTokenStore` returning the same tokens for any `userID` is a deliberate, documented multi-tenancy weakening. M8a swaps in `DBTokenStore` and re-runs sync's cross-tenant tests with per-user real tokens.

`Clock` (port from M1) is injected; tests freeze time. No `time.Now()` outside the clock.

Adapter is constructed per-sync from `akahuFactory(appToken, userToken) AkahuClient` so tests can inject a fake without env at all.

## Test plan (TDD)

1. `internal/akahu/retry_test.go` — backoff timing, jitter bounds, Retry-After honoured, 4xx-not-429 fails fast, ctx cancellation cancels mid-backoff.
2. `internal/akahu/client_test.go` — pagination loop; auth headers present; non-2xx wraps body excerpt with token redaction; malformed JSON → typed error.
3. `internal/ingest/sync_test.go` (in-memory fakes for repos + AkahuClient + TokenStore + Clock):
   - First sync inserts accounts and txns.
   - Second sync: idempotent for unchanged data.
   - 24h overlap window catches late-posting txn.
   - First-sync 30d default; `--from` override.
   - `sync_state` per (user, account).
4. `internal/observability/redaction_test.go` — both layers (key + value scanner) tested with golden cases.
5. `cmd/cli/sync_test.go` — end-to-end with testcontainers Postgres + `httptest` Akahu.

## Acceptance criteria

- All deliverables ticked.
- Real ANZ smoke-test passes; real Westpac smoke-test passes (after user connects Westpac).
- No token in any log file or error message under `--verbose`.

## Files an agent will touch

```
internal/ports/{akahu_client.go, token_store.go}
internal/akahu/{client.go, env_token_store.go, retry.go, types.go, fixtures/, *_test.go}
internal/ingest/{sync.go, mapper.go, *_test.go}
internal/observability/redaction.go (slog handler wrapper)
internal/storage/postgres/{txn.go, account.go, sync_state.go} (extend with upsert + sync_state methods)
internal/storage/postgres/queries.sql (extend)
cmd/cli/{sync.go, health.go (extend), *_test.go}
```

## Pitfalls

- Akahu amount fields can come as decimal strings or numbers; coerce to string then `decimal.NewFromString`.
- Direction (`DEBIT`/`CREDIT`) inference rules per Akahu docs at implementation time; do not assume.
- Don't bake the Akahu base URL as a constant — inject so `httptest` overrides.
- Don't log tokens. Don't log amounts. Don't log merchants. The redaction filter is the safety net, not the policy.
- Don't insert `raw_json` into any error chain; if a parse fails, include the txn id only.
- Treat the 24h overlap window as a fixed constant for now; making it configurable is YAGNI.
