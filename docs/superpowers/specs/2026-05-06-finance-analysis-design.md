# Finance Analysis — Design Spec

- **Date:** 2026-05-06
- **Author:** Anh Pham (with AI assistance)
- **Status:** Approved for planning
- **Philosophy reference:** [anh-pham191/development-rule](https://github.com/anh-pham191/development-rule)

## 1. Purpose

A personal finance tool that connects to NZ bank accounts (ANZ first, Westpac later) via the Akahu personal API, ingests transactions into a local Postgres database, categorises them via user-authored rules with manual override, and produces summary, comparison, and drill-down reports from the command line.

The tool is **CLI-first** but architected so that a future HTTP API and web UI can be added without changes to the domain or storage layers.

It is **single-user at runtime today** but **multi-tenant by design** from day 1: every domain entity is scoped to a `user_id`, every repository method takes a `user_id`, every Akahu token is associated with a user, and no query ever returns rows across users. M8 (later) flips on registration, authentication, and per-user encrypted token storage — no schema migration or domain rewrite required.

## 2. Non-goals (MVP)

- No registration, login, or authentication UI in M1–M5. The single-user developer's identity is implicit (user id `1`, seeded by migration).
- No multi-currency. NZD only.
- No ML categorisation, no budgeting, no cashflow forecasting (post-M5 candidates).
- No webhook ingestion (Akahu push). On-demand pull only.
- No web UI in this spec.

What is **explicitly not** a non-goal: multi-tenancy. All schemas and ports are built for it from day 1. Only the auth surface is deferred.

## 3. Architecture

Hexagonal layout. Domain and use-case packages have no knowledge of CLI, HTTP, Akahu, or Postgres.

```
finance-analysis/
├── cmd/
│   ├── cli/              # MVP entry point (cobra)
│   └── api/              # later — HTTP server, same core
├── internal/
│   ├── domain/           # pure types: Transaction, Category, Account, Period, Rule
│   ├── ingest/           # sync orchestration; depends on AkahuClient port + Repository port
│   ├── categorise/       # rule engine, override resolution
│   ├── report/           # summary, comparison, query — pure functions over Repository
│   ├── render/           # Table | CSV | JSON | Markdown renderers
│   ├── akahu/            # adapter: implements AkahuClient (HTTP → Akahu API)
│   └── storage/
│       └── postgres/     # adapter: implements Repository
├── migrations/           # SQL migrations (golang-migrate)
├── config/               # rules.yaml, app config
├── docker-compose.yml    # local Postgres
└── docs/
```

### Ports

- `AkahuClient` (defined in `internal/ingest/`)
  - `FetchTransactions(ctx, accountID, since time.Time) ([]RawTxn, error)`
  - `ListAccounts(ctx) ([]RawAccount, error)`
- `Repository` (defined in `internal/domain/` or `internal/ports/`)
  - Account, Transaction, Category, CategoryAssignment, Rule, SyncState CRUD.

CLI today calls use-case packages directly. Adding HTTP API later = `cmd/api` calling the same use cases — zero changes to domain or storage. Webhook ingestion later = a second adapter implementing the same ingest port. Postgres swap later = a second `Repository` implementation.

## 4. Data model (Postgres)

All money columns are `numeric(14,2)`. NZD-only for MVP; `currency` lives on `accounts` so multi-currency is a future migration, not a redesign. All timestamps are `timestamptz`. **Every user-owned table carries a `user_id` column** with an index — even though MVP runs with one user, queries are scoped from the start.

### Tables

- **`users`** — `id bigserial PK`, `email citext unique`, `display_name text`, `password_hash text null` (filled in M8), `created_at`, `updated_at`. M1 seed migration inserts `id=1, email='local@finance-analysis'` for the single-user dev mode.
- **`akahu_tokens`** — `user_id bigint PK FK users(id)`, `app_token_ciphertext bytea`, `user_token_ciphertext bytea`, `nonce bytea`, `updated_at`. Tokens encrypted at rest with AES-GCM using a key from the `TokenStore` port (env-derived in MVP, KMS later). Never stored in plaintext, never logged.
- **`accounts`** — `id text PK` (Akahu account id), `user_id bigint FK users(id)`, `name text`, `bank text`, `type text`, `currency text default 'NZD'`, `created_at`. Composite uniqueness `(user_id, id)` is implied by `id` being globally unique from Akahu, but indexes on `user_id` for fast scoping.
- **`transactions`** — `id text PK` (Akahu txn id), `user_id bigint FK`, `account_id text FK`, `posted_at timestamptz`, `amount numeric(14,2)`, `direction text check (direction in ('DEBIT','CREDIT'))`, `description text`, `merchant text`, `akahu_category text`, `raw_json jsonb`, `created_at`, `updated_at`. Index on `(user_id, posted_at desc)`.
- **`categories`** — `id bigserial PK`, `user_id bigint FK`, `name text`, `parent_id bigint null FK categories(id)`, `kind text check (kind in ('income','expense','transfer'))`. Unique `(user_id, name)`. Categories are per-user; no shared global taxonomy.
- **`category_assignments`** — `txn_id text PK FK transactions(id)`, `user_id bigint FK` (denormalised for query scoping; FK consistency enforced via trigger or app-level invariant), `category_id bigint FK categories(id)`, `source text check (source in ('RULE','MANUAL','AKAHU'))`, `rule_id bigint null FK rules(id)`, `assigned_at timestamptz`.
- **`rules`** — `id bigserial PK`, `user_id bigint FK`, `name text`, `priority int`, `predicate jsonb`, `category_id bigint FK`, `enabled bool default true`, `created_at`. Source of truth in MVP is `config/rules.yaml` for the dev user; M8 moves rules into the DB as primary source for multi-user.
- **`sync_state`** — `(user_id, account_id) PK`, `last_synced_at timestamptz`, `last_cursor text null`.

### Row-level scoping invariant

**No query in the codebase may run without a `user_id` filter on user-owned tables.** Enforced by:

1. Every `Repository` method takes `userID UserID` as the first non-context argument. There is no overload that omits it.
2. Postgres Row-Level Security (RLS) enabled on every user-owned table, with a policy of `user_id = current_setting('app.user_id')::bigint`. The repository sets `app.user_id` per connection-borrow before issuing queries.
3. Integration tests assert that a query issued under user A cannot read or write user B's rows.

### Identity & idempotency

Akahu's transaction ID is the PK. Re-syncing is `INSERT ... ON CONFLICT (id) DO UPDATE` to preserve `created_at` and refresh `raw_json` if Akahu re-enriches. The upsert always includes `user_id` in the `WHERE` clause so a hypothetical id collision across users would error rather than overwrite.

## 5. Akahu sync

- **Auth:** per-user tokens. Resolved via a `TokenStore` port:

  ```go
  type TokenStore interface {
      AkahuTokens(ctx context.Context, userID UserID) (app, user string, err error)
  }
  ```

  Two implementations:
  - **`EnvTokenStore`** (MVP) — reads `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN` from env, returns the same tokens regardless of `userID`. Acceptable while there is one user.
  - **`DBTokenStore`** (M8) — reads `akahu_tokens`, decrypts with AES-GCM using a master key from a `KeyProvider` port (env-derived locally, KMS or similar in production).

  The use case never sees plaintext tokens beyond the `AkahuClient` call boundary. Tokens are never logged, never serialised into errors, never written to disk outside the encrypted `akahu_tokens` row.

- **Mode:** on-demand pull. `finance sync` fetches new transactions per account since `sync_state.last_synced_at` (with a small overlap window — e.g. 24h — to handle late-posting).
- **Adapter:** `internal/akahu/` implements `AkahuClient` over HTTP. Tested with recorded fixtures via `httptest`. The adapter is constructed per-request from tokens supplied by `TokenStore` — it does not read env directly.
- **Use case:** `internal/ingest/Sync(ctx, userID, repo, akahuFactory, clock)` orchestrates: resolve user's tokens → build an `AkahuClient` → list accounts → for each account, fetch since cursor → upsert transactions → update sync_state. Always scoped to one user.

Webhook ingestion is a future second adapter implementing the same ingest port. Not in MVP.

## 6. Categorisation

### Rules file

`config/rules.yaml` is the source of truth — checked into git, easy to diff/review. Loaded into the `rules` DB table on `finance categorise` so reports can join.

```yaml
- name: Salary
  priority: 10
  when:
    direction: CREDIT
    description_matches: "(?i)payroll|salary"
  category: Income/Salary

- name: Groceries
  priority: 20
  when:
    merchant_in: ["Countdown", "Pak'nSave", "New World"]
  category: Food/Groceries

- name: Transfers between own accounts
  priority: 5
  when:
    akahu_category: TRANSFER
  category: Transfers
```

### Predicate fields (MVP)

`direction`, `amount_min`, `amount_max`, `description_matches` (regex), `merchant_in` (list), `merchant_matches` (regex), `account_in` (list), `akahu_category`. AND-semantics within a rule. First match wins (lowest `priority` number = highest precedence).

### Resolution order per transaction

1. If `category_assignments.source = MANUAL` exists → use it. Stop.
2. Else evaluate rules in priority order, first match wins → store as `RULE`.
3. No match → category `Uncategorised`, source `RULE` (so re-running rules can later move it).

### Commands

- `finance categorise` — re-applies rules to all non-manual transactions (idempotent).
- `finance recat <txn-id> <category>` — sets a `MANUAL` assignment.
- `finance uncategorised` — lists txns that hit no rule (drives rule authoring).

## 7. Reporting (MVP: A + B + D)

All report logic is pure functions taking a `Repository` and returning typed result structs. CLI renders them; future API returns same structs as JSON.

### Summary — `finance summary`

`--period 2026-04` or `--from 2026-04-01 --to 2026-05-01`.
Output: total in, total out, net; broken down by category (and parent category, if hierarchy used).

### Comparison — `finance compare`

`--a <period> --b <period>` (compare A vs B).
Convenience flags:
- `--wow` → `--a this-week --b last-week`
- `--mom` → `--a this-month --b last-month`
- `--yoy` → `--a this-year --b last-year`
Output: per-category A total, B total, Δ absolute, Δ %, sorted by absolute Δ desc. `--top N` truncates.

### Drill-down — `finance txns`

Filters: `--category`, `--merchant`, `--account`, `--from/--to` or `--period`, `--direction`, `--min`, `--max`. Sort: `--sort=amount|date|merchant`.

### Period type

`internal/domain/Period` parses:
- `YYYY-MM` (calendar month)
- `YYYY-Www` (ISO week)
- `YYYY` (calendar year)
- Explicit `--from / --to` ranges
- Relative: `this-week`, `last-week`, `this-month`, `last-month`, `this-year`, `last-year`

Resolves to `[from, to)` half-open interval in `Pacific/Auckland`. One implementation, used everywhere.

### Rendering

`internal/render/` — `Table` (default), `CSV`, `JSON`, `Markdown`. CLI picks via `--format`. API later always picks JSON.

## 8. Configuration & secrets

### Config file

`~/.config/finance-analysis/config.yaml` (override with `--config`):

```yaml
database:
  url_env: DATABASE_URL          # secret stays in env; file holds only the env name
akahu:
  app_token_env: AKAHU_APP_TOKEN
  user_token_env: AKAHU_USER_TOKEN
master_key_env: FINANCE_MASTER_KEY   # 32-byte base64; required from M8, optional in M1–M5
timezone: Pacific/Auckland
rules_file: ./config/rules.yaml
```

The config file references env var **names**, never values. This keeps the file safe to commit if the user ever wants to share it.

### Secret handling rules (enforced from M1)

1. **No secret in any file checked into git.** Includes `config.yaml`, `.env`, `rules.yaml` if it leaks merchant patterns the user considers personal, fixtures with real txn data.
2. **`.env` is gitignored;** `.env.example` ships only key names.
3. **`config/rules.yaml` is gitignored;** `config/rules.example.yaml` ships as the template.
4. **Akahu fixtures captured from real account data are gitignored;** synthetic fixtures (`internal/akahu/fixtures/*.synthetic.json`) are committed.
5. **Tokens never appear in logs.** Add a redaction pass in the slog handler (`replace_attr` for keys named `token`, `authorization`, `app_token`, `user_token`, `password`, `secret`, anything matching `(?i)token`).
6. **Errors never wrap raw token values.** Adapter errors include status code + URL + body excerpt with tokens scrubbed.
7. **Database URL with password** is read from env, not the config file.
8. **Master encryption key** (`FINANCE_MASTER_KEY`) is the input to the `KeyProvider` port. In dev it's an env var; in production (multi-user), it comes from a KMS or sealed-secret system. Rotation strategy documented in `docs/architecture/security.md`.
9. **Pre-commit hook (recommended)** — `gitleaks` or equivalent runs on `git commit` to catch accidental secret commits. Documented in M5.

### Backups

Postgres dumps must be encrypted before leaving the host. Document this in M5; do not provide an unencrypted dump command in any Makefile target.

## 9. Privacy & PII

Transactions contain PII (merchant, location, amounts, behavioural patterns). The system treats them accordingly:

- **No transaction data in logs.** Sync logs report counts only (e.g. `synced 42 txns for account X`), never amounts, descriptions, or merchants.
- **No transaction data in error messages** that propagate beyond the use-case boundary. Internal errors may include a txn ID for debugging.
- **No transmission to third parties** beyond Akahu itself. No analytics, no error reporting SaaS unless the user opts in and PII is scrubbed.
- **Right to delete:** M8 includes a `DELETE /users/me` endpoint (and `finance user delete --user <id>` admin command) that hard-deletes the user, all their accounts, transactions, categories, rules, sync_state, and tokens in a single transaction. Verify with an integration test.

## 10. Observability

- `log/slog` text handler in CLI; JSON handler later in API.
- Levels: INFO normal, DEBUG behind `--verbose`.
- Errors wrapped with context: `fmt.Errorf("ingest: fetching txns since %s: %w", since, err)`.
- Sentinel errors only when callers need to branch.

## 11. Testing — TDD throughout

Red → Green → Refactor for every package. No production code without a failing test first. **Tests are specifications** and are never modified to make a failing run pass without explicit reasoning (per [development-rule](https://github.com/anh-pham191/development-rule)'s agent-safety skill).

| Layer | Style | Notes |
|---|---|---|
| `domain/`, `categorise/`, `report/`, `ingest/` use cases | Pure unit tests, in-memory fakes for ports | Tests drive design |
| `storage/postgres/` | Integration against real Postgres (testcontainers-go or docker-compose) | `make test-integration` |
| `akahu/` | HTTP fixtures via `httptest` | Recorded responses |
| `cmd/cli/` | End-to-end with cobra `SetArgs`, captured stdout, testcontainers Postgres | Behaviour-level |

Coverage is not a target. Every behaviour in this spec must have a test that fails if the behaviour breaks.

**Cross-tenant safety tests (mandatory):** every milestone that adds a Repository method also adds an integration test that:
1. Seeds two users with overlapping data shapes.
2. Calls the new method as user A.
3. Asserts no row from user B is in the result and no row of user B's was modified.

## 12. Tooling

- Go 1.23+
- `cobra` for CLI
- `golang-migrate` for migrations
- `sqlc` for type-safe queries (SQL lives in `.sql` files)
- `golangci-lint`
- `testcontainers-go` for integration tests
- `godotenv` for dev `.env` loading
- No `viper` until config genuinely needs it
- `gitleaks` for pre-commit secret scanning (M5)

## 13. Milestones

Each milestone ends green: tests pass, lint passes, CLI command works against real Postgres. No half-finished milestones. **Multi-tenancy invariants (user_id on every row, RLS, scoped repository methods, redacted logs) apply from M1 onward** — they are not deferred to M8.

1. **M1 — Skeleton & DB.** Repo layout, Docker Postgres, migrations (incl. `users` and `akahu_tokens`), seed dev user, domain types, `Repository` + Postgres impl with RLS-scoped queries, `finance migrate`.
2. **M2 — Akahu ingest.** `AkahuClient` adapter, `TokenStore` port + `EnvTokenStore`, `ingest` use case scoped to a user, `finance sync`, idempotent upserts, `sync_state`.
3. **M3 — Categorisation.** Rule engine, `rules.yaml` loader (per-user), assignments, `categorise`/`recat`/`uncategorised`.
4. **M4 — Reporting MVP.** `Period` parser (incl. relative + WoW/MoM/YoY), `summary`/`compare`/`txns`, renderers.
5. **M5 — Polish.** Structured logging with redaction, `--format` outputs, config & secret loading, `gitleaks` pre-commit, encrypted backup docs, README, example `rules.yaml`.
6. **M6 — Westpac.** Add account onboarding via Akahu; config-only if M2 was built right.
7. **M7 — HTTP API.** `cmd/api` reusing every use case; bearer-token auth (single user); same DTOs → JSON.
8. **M8 — Multi-user.** Registration, password auth (argon2id), session tokens, `DBTokenStore` with AES-GCM encryption, `KeyProvider` port (env / KMS), per-user rules in DB, signup/onboarding flow that walks the user through Akahu connection, account deletion endpoint.

Per-milestone docs live under `docs/milestones/M{n}-*.md` and contain the full context any agent needs to work on that milestone in isolation.

## 14. Constraints from [development-rule](https://github.com/anh-pham191/development-rule)

- **NZ English** in user-facing copy and docs (e.g. "categorise", "behaviour").
- **Never commit without explicit human approval.**
- **Never modify tests without permission** — tests are specifications.
- **Pause before destructive operations.**
- **Composable over monolithic.** Small, focused packages with one clear purpose.
- **Tool-agnostic at the core.** Domain/use cases bind to no provider.
- **Evolved from use.** Don't add features the user hasn't asked for. YAGNI ruthlessly.
- Commit subject in imperative mood, ≤72 chars; body explains *why*; ticket prefix when tracked.

## 15. Open questions

None blocking implementation start. Future decisions to revisit when relevant:
- Will rules need OR-semantics or rule groups? (revisit after a month of real use)
- Should manual overrides expire or stick forever? (current: stick forever)
- Should we model account hierarchies (joint vs personal)? (defer until needed)
- Where does the production master key live in M8? (KMS vs sealed-secret vs cloud secret manager — decide at M8 kickoff based on hosting choice)
- Self-hosted multi-user vs SaaS multi-user — does the user run one shared instance for friends/family, or do users self-host? (affects M8 scope; default assumption: self-hosted shared instance)
