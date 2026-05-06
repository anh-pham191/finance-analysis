# Architecture Overview

> Companion to the design spec at `docs/superpowers/specs/2026-05-06-finance-analysis-design.md`. Read that first.

This document is the entry point for any agent working on the codebase. It explains the *why* behind the layout so that decisions made in any milestone stay consistent.

## Guiding principles

1. **CLI today, API-ready always.** Every piece of business logic lives behind a function or method that does not import `cobra`, `net/http`, or any concrete adapter. The CLI is a thin presentation layer; the future HTTP API will be another thin presentation layer over the same core.
2. **Single-user runtime, multi-tenant data model.** Every user-owned table has `user_id`. Every repository method takes `UserID`. Postgres RLS is on. M8 flips on the auth surface — it does not redesign anything.
3. **Ports and adapters (hexagonal).** Domain code defines interfaces it needs (`Repository`, `AkahuClient`, `TokenStore`, `KeyProvider`). Adapters implement them. This keeps the domain testable with in-memory fakes and swappable in production.
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

internal/akahu/             implements → ingest.AkahuClient
internal/storage/postgres/  implements → domain/ports.Repository

internal/domain/    no internal deps
```

**Forbidden imports** (enforced by code review and `go list` inspection in CI later):
- `internal/domain/` MUST NOT import any other internal package.
- `internal/{ingest,categorise,report}/` MUST NOT import `internal/akahu/` or `internal/storage/`.
- `internal/{ingest,categorise,report}/` MUST NOT import `cobra` or `net/http`.
- No package outside `internal/akahu/` and `cmd/` may read env vars directly. Config is loaded once at the boundary and passed in.

## Where things live

| Concern | Package | Notes |
|---|---|---|
| Money, periods, txn/account/category types | `internal/domain` | No DB, no HTTP. Pure types and value-object behaviour. |
| Port interfaces | `internal/domain/ports` (or co-located with each use case) | Defined by consumer, not provider. |
| Akahu sync use case | `internal/ingest` | Orchestrates `AkahuClient` + `Repository`. |
| Rule engine | `internal/categorise` | Pure functions: `Apply(txn, rules) → assignment`. |
| Reports | `internal/report` | Pure functions over `Repository`. Returns DTOs. |
| Output rendering | `internal/render` | Table/CSV/JSON/Markdown. |
| Akahu HTTP adapter | `internal/akahu` | Implements `ingest.AkahuClient`. |
| Postgres adapter | `internal/storage/postgres` | Implements `Repository`. SQL via `sqlc`. |
| Migrations | `migrations/` | `golang-migrate`. |
| Rules source of truth | `config/rules.yaml` | Loaded into `rules` table on `categorise`. |
| Local Postgres | `docker-compose.yml` | Dev only. |

## Cross-cutting

- **Time:** `Pacific/Auckland` everywhere user-facing. Stored as `timestamptz`. Pass `time.Time` and `*time.Location` explicitly; never reach for `time.Local`.
- **Money:** `numeric(14,2)` in DB; in Go either `decimal.Decimal` (shopspring/decimal) or a custom `Money` value type — decision: **shopspring/decimal**, used uniformly. No floats for money.
- **IDs:** Akahu IDs are strings; we keep them as strings, not UUIDs.
- **Errors:** wrap with context, `%w` for the cause. Sentinel errors only when callers branch.
- **Logging:** `log/slog`. CLI uses text handler. Don't log secrets, ever.
- **Config:** loaded once at CLI startup; passed as values into use cases. Use cases never read env or files.

## What an agent should do before changing code

1. Read the relevant milestone doc in `docs/milestones/`.
2. Read the spec section it references.
3. Write or extend a failing test that captures the behaviour.
4. Make it pass with the smallest change.
5. Refactor with the test green.
6. Run `go test ./...`, `golangci-lint run`.
7. Commit with imperative subject ≤72 chars, body explains *why*.
