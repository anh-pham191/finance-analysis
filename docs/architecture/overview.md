# Architecture Overview

> Companion to the design spec at `docs/superpowers/specs/2026-05-06-finance-analysis-design.md`. Read that first.

This document is the entry point for any agent working on the codebase. It explains the *why* behind the layout so that decisions made in any milestone stay consistent.

## Guiding principles

1. **CLI today, API-ready always.** Every piece of business logic lives behind a function or method that does not import `cobra`, `net/http`, or any concrete adapter. The CLI is a thin presentation layer; the future HTTP API will be another thin presentation layer over the same core.
2. **Single-user runtime, multi-tenant data model.** Every user-owned table has `user_id`. Every repository method takes `UserID`. Postgres RLS is on. M8 flips on the auth surface — it does not redesign anything.
3. **Ports and adapters (hexagonal).** Domain code defines interfaces it needs in `internal/ports/`: per-aggregate repos (`AccountRepo`, `TxnRepo`, `CategoryRepo`, `RuleRepo`, `AssignmentRepo`, `SyncStateRepo`), `AkahuClient`, `TokenStore`, `Clock`, and later `Authenticator`, `KeyProvider`. Adapters implement them. This keeps the domain testable with in-memory fakes and swappable in production. **Per-aggregate repos rather than one god `Repository`** — keeps interfaces cohesive, tests' fakes small, and prevents the integration interface from sprawling to ~40 methods by M4.
4. **Pure functions where possible.** Reporting and categorisation are deterministic functions of inputs. No hidden state, no clocks reached for from inside, no `time.Now()` deep in a helper — clocks and config are passed in.
5. **TDD always.** Red, green, refactor. The test is written first. Tests describe behaviour, not implementation. Tests are not modified to silence a failing run.
6. **Secrets out of the repo, out of logs, out of errors.** No exceptions. See `docs/architecture/security.md`.
7. **YAGNI ruthlessly.** No abstractions, options, or flags that aren't needed by the current milestone. Three similar lines is better than a premature abstraction.
8. **Composable over monolithic.** Small packages, one clear purpose each. If a file is hard to hold in your head, it's doing too much.

## Layer map

```
cmd/cli/           depends on → internal/{ingest,categorise,report,render}
cmd/api/  (later)  depends on → internal/{ingest,categorise,report}

internal/ingest/      depends on → internal/domain, ports
internal/categorise/  depends on → internal/domain, ports
internal/report/      depends on → internal/domain, ports
internal/render/      depends on → internal/domain (DTOs only)

internal/akahu/             implements → ports.AkahuClient, ports.TokenStore (EnvTokenStore)
internal/storage/postgres/  implements → ports.{Account,Txn,Category,Rule,Assignment,SyncState}Repo

internal/domain/    no internal deps
internal/ports/     no internal deps except domain
```

**Forbidden imports** (enforced from M1 by `internal/archtest/archtest_test.go` — fails CI on regression):
- `internal/domain/` MUST NOT import any other internal package.
- `internal/{ingest,categorise,report}/` MUST NOT import `internal/akahu/` or `internal/storage/`.
- `internal/{ingest,categorise,report}/` MUST NOT import `cobra` or `net/http`.
- No package outside `internal/akahu/` and `cmd/` may read env vars directly. Config is loaded once at the boundary and passed in.

## Where things live

| Concern | Package | Notes |
|---|---|---|
| Money, periods, txn/account/category types, `UserID` | `internal/domain` | No DB, no HTTP. Pure types and value-object behaviour. |
| Port interfaces | `internal/ports` | Defined by consumer, not provider. Per-aggregate repos. |
| Akahu sync use case | `internal/ingest` | Orchestrates ports. |
| Rule engine | `internal/categorise` | Pure functions: `Apply(txn, rules) → assignment`. |
| Reports | `internal/report` | Pure functions over repository ports. Returns DTOs with JSON tags. |
| Output rendering | `internal/render` | `io.Writer`-based renderers; one per (result, format) pair. |
| Akahu HTTP adapter | `internal/akahu` | Implements `AkahuClient` and `EnvTokenStore`. |
| Postgres adapter | `internal/storage/postgres` | Implements every repo port. SQL via `sqlc`. `withUserTx` helper enforces RLS. |
| Architecture test | `internal/archtest` | Fails CI on forbidden imports. From M1. |
| Migrations | `migrations/` | `golang-migrate`. Run as DB owner; the app role does not run migrations. |
| Categories taxonomy | `config/categories.yaml` | Loaded into `categories` table on `categorise`. Source of truth in M3. |
| Rules source of truth | `config/rules.yaml` | Loaded into `rules` table on `categorise`. M3. |
| Local Postgres | `docker-compose.yml` | Dev only. App connects as `finance_app` (non-owner, no `BYPASSRLS`). |

## Cross-cutting

- **Time:** default `Pacific/Auckland` for user-facing output, but the location is **passed in** to every function that needs it (`Period.Resolve(loc, now)` etc.) — never hardcoded inside domain or use cases. Stored as `timestamptz`. Never reach for `time.Local`. ISO 8601 weeks (Monday-start). DST transitions are wall-clock-anchored.
- **Money:** `numeric(14,2)` in DB; in Go either `decimal.Decimal` (shopspring/decimal) or a custom `Money` value type — decision: **shopspring/decimal**, used uniformly. No floats for money.
- **IDs:** Akahu IDs are strings; we keep them as strings, not UUIDs.
- **Errors:** wrap with context, `%w` for the cause. Sentinel errors only when callers branch. Never include tokens, raw response bodies (run through redaction), or `raw_json` in error chains.
- **Logging:** `log/slog`. CLI uses text handler. Don't log secrets. Don't log transaction PII (amounts, descriptions, merchants). Counts only.
- **RLS:** every Postgres repo call runs in a tx that begins with `SET LOCAL app.user_id = $userID`. There is one helper (`withUserTx`) and all queries go through it. RLS is FORCED on every user-owned table; the app connects as a non-owner role.
- **Config:** loaded once at CLI startup; passed as values into use cases. Use cases never read env or files.

## What an agent should do before changing code

1. Read the relevant milestone doc in `docs/milestones/`.
2. Read the spec section it references.
3. Write or extend a failing test that captures the behaviour.
4. Make it pass with the smallest change.
5. Refactor with the test green.
6. Run `go test ./...`, `golangci-lint run`.
7. Commit with imperative subject ≤72 chars, body explains *why*.
