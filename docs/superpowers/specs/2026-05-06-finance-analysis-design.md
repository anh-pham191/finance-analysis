# Finance Analysis — Design Spec

- **Date:** 2026-05-06 (revised after spec-hardening review on 2026-05-06)
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
│   ├── domain/           # pure types: Transaction, Category, Account, Period, Rule, Money, UserID
│   ├── ports/            # interfaces consumed by use cases (per-aggregate repos, TokenStore, AkahuClient, Clock)
│   ├── ingest/           # sync orchestration; depends only on ports/
│   ├── categorise/       # rule engine, override resolution
│   ├── report/           # summary, comparison, query — pure functions over repository ports
│   ├── render/           # Table | CSV | JSON | Markdown renderers (io.Writer-based)
│   ├── akahu/            # adapter: implements AkahuClient (HTTP → Akahu API)
│   └── storage/
│       └── postgres/     # adapter: implements all repository ports
├── migrations/           # SQL migrations (golang-migrate)
├── config/               # categories.example.yaml, rules.example.yaml, config.example.yaml
├── docker-compose.yml    # local Postgres
└── docs/
```

### Ports — per-aggregate, not a single god repository

To prevent the integration interface from sprawling to ~40 methods by M4, each aggregate gets its own port:

- `AccountRepo`, `TxnRepo`, `CategoryRepo`, `RuleRepo`, `AssignmentRepo`, `SyncStateRepo` — defined in `internal/ports/`. Each is small, cohesive, and has its own in-memory fake for unit tests.
- `AkahuClient` — `FetchTransactions(ctx, accountID, since time.Time) ([]RawTxn, error)`, `ListAccounts(ctx) ([]RawAccount, error)`.
- `TokenStore` — `AkahuTokens(ctx, userID UserID) (app, user string, err error)`.
- `Clock` — `Now() time.Time`. Production = `realClock{}`; tests inject a fixed clock.
- (M7+) `Authenticator` — `Authenticate(ctx, request *http.Request) (UserID, error)`. Two implementations: `EnvBearerAuthenticator` (M7 single-user), `SessionAuthenticator` (M8a). Designed in M7 so M8 extends, never swaps.
- (M8a+) `KeyProvider` — `MasterKey(ctx, version int) ([]byte, error)`.

CLI today calls use-case packages directly. Adding HTTP API later = `cmd/api` calling the same use cases — zero changes to domain or storage. Webhook ingestion later = a second adapter implementing the same ingest port. Postgres swap later = a second set of repo implementations.

## 4. Data model (Postgres)

All money columns are `numeric(14,2)`. NZD-only for MVP; `currency` lives on `accounts` so multi-currency is a future migration, not a redesign. All timestamps are `timestamptz`. **Every user-owned table has a `user_id` column** with an index, RLS enabled and FORCED, and FK cascades configured for hard-delete.

### Core invariants

1. **Composite primary keys including `user_id`** on every user-owned table whose natural key comes from an external system (Akahu IDs). This guarantees cross-user uniqueness even if Akahu ever reuses an ID.
2. **`ON DELETE CASCADE` from `users`** down to every owned row, so the right-to-delete operation is a single `DELETE FROM users WHERE id = $1`.
3. **Postgres RLS is enabled AND forced** on every user-owned table:
   ```sql
   ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;
   ALTER TABLE <t> FORCE ROW LEVEL SECURITY;
   CREATE POLICY <t>_tenant_isolation ON <t>
     USING (user_id = current_setting('app.user_id')::bigint)
     WITH CHECK (user_id = current_setting('app.user_id')::bigint);
   ```
   `FORCE ROW LEVEL SECURITY` is mandatory — without it, the table owner bypasses RLS and the cross-tenant tests can pass with broken isolation.
4. **The application connects as a non-owner role** (`finance_app`) without `BYPASSRLS`. Migrations run as the owner; the app does not.
5. **Every repository call runs inside an explicit `BEGIN ... COMMIT`** that begins with `SET LOCAL app.user_id = $userID`. `SET LOCAL` is transaction-scoped — outside a tx it has no effect, and connection-pool checkouts must not inherit a previous user's setting. This is enforced at one place in the repository implementation (`withUserTx` helper), and all generated SQL goes through it.

### Tables (M1)

- **`users`** — `id bigserial PK`, `email citext unique`, `display_name text`, `created_at`, `updated_at`. M1 seed migration inserts `id=1, email='local@finance-analysis'` for single-user dev mode. (No `password_hash` until M8a — added by migration then.)
- **`accounts`** — `(user_id, id) PRIMARY KEY`; `id text` (Akahu account id), `user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `name text`, `bank text`, `type text`, `currency text DEFAULT 'NZD'`, `created_at`. Index on `user_id`.
- **`transactions`** — `(user_id, id) PRIMARY KEY`; `id text` (Akahu txn id), `user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `account_id text`, FK `(user_id, account_id) REFERENCES accounts(user_id, id) ON DELETE CASCADE`, `posted_at timestamptz`, `amount numeric(14,2)`, `direction text CHECK (direction IN ('DEBIT','CREDIT'))`, `description text`, `merchant text`, `akahu_category text`, `raw_json jsonb`, `created_at`, `updated_at`. Index on `(user_id, posted_at desc)`.
- **`categories`** — `id bigserial PK`, `user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `name text`, `parent_id bigint NULL REFERENCES categories(id) ON DELETE SET NULL`, `kind text CHECK (kind IN ('income','expense','transfer'))`. Unique `(user_id, name)`. Categories are per-user, declared in `config/categories.yaml` (taxonomy is its own concern, see §6). Auto-created with a sentinel `Uncategorised` row of kind `expense` per user.
- **`rules`** — `id bigserial PK`, `user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `name text`, `priority int`, `predicate jsonb`, `category_id bigint NOT NULL REFERENCES categories(id) ON DELETE RESTRICT`, `enabled bool DEFAULT true`, `created_at`, `updated_at`. Unique `(user_id, name)` — this is the natural key, used for stable IDs across yaml reloads.
- **`category_assignments`** — `(user_id, txn_id) PRIMARY KEY`; `txn_id text`, `user_id bigint NOT NULL`, `category_id bigint NOT NULL REFERENCES categories(id) ON DELETE RESTRICT`, `source text CHECK (source IN ('RULE','MANUAL','AKAHU'))`, `rule_id bigint NULL REFERENCES rules(id) ON DELETE SET NULL`, `assigned_at timestamptz`. Composite FK `(user_id, txn_id) REFERENCES transactions(user_id, id) ON DELETE CASCADE`. The `user_id` column is denormalised but enforced consistent by the composite FK — no trigger needed.
- **`sync_state`** — `(user_id, account_id) PRIMARY KEY`, composite FK to `accounts(user_id, id) ON DELETE CASCADE`, `last_synced_at timestamptz`, `last_cursor text NULL`.

### Tables added later

- `akahu_tokens` and the `password_hash` column on `users` are added in **M8a** by migration, with `key_version` baked in from the start. They do not exist in M1–M7. (See M8a brief.)
- `sessions` (auth tokens) added in M8a.
- `audit_log` added in M8b.

### Identity & idempotency

Akahu's transaction ID is the `id` column; the PK is the composite `(user_id, id)`. The upsert is:

```sql
INSERT INTO transactions (user_id, id, ...)
VALUES ($1, $2, ...)
ON CONFLICT (user_id, id) DO UPDATE
  SET raw_json = EXCLUDED.raw_json,
      updated_at = now()
  -- created_at, posted_at, amount NOT updated; preserved
```

Cross-user collision is impossible because the PK includes `user_id`. Within a user, re-syncing is idempotent for unchanged rows and convergent for re-enriched ones.

If Akahu ever issues a correction (different amount for the same txn id), the upsert overwrites `raw_json` but **does not touch `amount` or `posted_at`** — divergence is recorded by a M2 integration test as a known case to revisit. (Open question §15.)

## 5. Akahu sync

- **Auth:** per-user tokens via `TokenStore` port (see §3 ports).

  - **`EnvTokenStore`** (M2 → M7) — reads `AKAHU_APP_TOKEN`, `AKAHU_USER_TOKEN` from env. **Returns the same tokens regardless of `userID`.** This is an intentional weakening of the multi-tenancy invariant for single-user MVP. M8a's cross-tenant test for sync re-runs with `DBTokenStore` to validate isolation under real multi-user.
  - **`DBTokenStore`** (M8a) — reads `akahu_tokens`, decrypts with AES-GCM (see §8 token encryption).

  Tokens never appear in logs, errors, or any code path outside the `AkahuClient` constructor.

- **Mode:** on-demand pull. `finance sync` fetches per account.

- **First-sync lookback:** when `sync_state.last_synced_at` is null for an account, default lookback is **30 days**. Override with `finance sync --from 2024-01-01`. Documented in M2 brief and CLI help.

- **Late-posting overlap:** subsequent syncs query from `last_synced_at - 24h` to catch txns posted with a delay.

- **Retry & backoff:** the `AkahuClient` adapter retries on `429` and `5xx` only, up to **3 attempts**, with exponential backoff `1s, 2s, 4s` plus ±25% jitter, honouring `Retry-After` if present. `4xx` other than 429 fails fast. Context cancellation is honoured between retries. Tested with a stub server.

- **Pagination:** cursor-based. The adapter loops until `cursor.next == ""`, surfacing partial results only on context cancellation (never on a single failed page — that returns an error).

- **Adapter:** `internal/akahu/` implements `AkahuClient` over HTTP, constructed with explicit `appToken, userToken, baseURL string` arguments. Does not read env directly. Tested against `httptest` fixtures, including 429/5xx retry, timeout, malformed JSON, missing fields.

- **Use case:** `internal/ingest/Sync(ctx, userID, deps)` orchestrates: resolve user's tokens via `TokenStore` → build `AkahuClient` → list accounts → upsert accounts → for each, fetch since cursor → upsert transactions → update sync_state. Always scoped to one user. `deps` is a struct holding the per-aggregate repos, the token store, the akahu factory, and the clock.

Webhook ingestion is a future second adapter implementing the same ingest port. Not in MVP.

## 6. Categorisation

Two YAML files, two concerns:

- **`config/categories.yaml`** — taxonomy (declared once, stable). Lists every category the user wants, with `kind` (`income|expense|transfer`) and optional parent. Loaded by `finance categorise` and reconciled with the `categories` table (insert new, update kind/parent, never delete — manual via SQL if needed).
- **`config/rules.yaml`** — matching predicates. Each rule references a category by name. Rules churn over time; the loader fully replaces the user's `rules` rows on each load (upsert by `(user_id, name)`, delete missing — `ON DELETE SET NULL` on `category_assignments.rule_id` keeps assignments alive).

### `config/categories.example.yaml`

```yaml
- name: Income/Salary
  kind: income
- name: Food/Groceries
  kind: expense
  parent: Food
- name: Food/Eating-out
  kind: expense
  parent: Food
- name: Transfers
  kind: transfer
- name: Uncategorised
  kind: expense   # default kind for unmatched expense-side txns
```

### `config/rules.example.yaml`

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

`direction`, `amount_min`, `amount_max`, `description_matches` (regex), `merchant_in` (list), `merchant_matches` (regex), `account_in` (list), `akahu_category`. AND-semantics within a rule.

### Resolution order per transaction

1. If `category_assignments.source = MANUAL` exists → keep it. Stop.
2. Else evaluate enabled rules in `priority` ascending order; **tiebreaker: `name` ascending** for deterministic ordering. First match wins → store as `RULE` with that `rule_id`.
3. No match → category `Uncategorised`, source `RULE`, `rule_id NULL`.

### Re-categorisation semantics ("convergent", not "idempotent")

`finance categorise` is convergent: re-running with the same inputs produces the same output. But editing rules and re-running deliberately reassigns — it isn't a no-op. Specifically:

- A rule is removed from `rules.yaml` → loader deletes the `rules` row → `category_assignments.rule_id` becomes NULL via `ON DELETE SET NULL` → the assignment is recomputed on the next `categorise` pass and either matches another rule or becomes `Uncategorised`.
- A rule is disabled (`enabled: false` in yaml) → kept as a row, skipped during evaluation, same recompute happens.
- A rule's category changes → `category_id` updates on the assignment, `assigned_at` updates only when a real change occurred (not on every run).
- `MANUAL` assignments are never touched.

`assigned_at` is updated only when `category_id` or `rule_id` actually changes — re-running on a stable state produces zero writes.

### Commands

- `finance categorise` — load `categories.yaml` and `rules.yaml`, reconcile DB, apply rules to all non-`MANUAL` transactions. Convergent.
- `finance recat <txn-id> <category>` — set a `MANUAL` assignment. Category must exist in `categories.yaml` (errors otherwise — no implicit creation, prevents typos becoming new categories).
- `finance unrecat <txn-id>` — clear a `MANUAL` assignment so the next `categorise` pass re-evaluates it.
- `finance uncategorised` — list txns currently in `Uncategorised` (drives rule authoring).

## 7. Reporting (MVP: A + B + D)

All report logic is pure functions taking the per-aggregate repo ports they need, plus a `*time.Location` and a `Clock`. They return typed result structs. CLI renders them via `io.Writer`-based renderers; future API returns the same structs as JSON (with `json:"..."` tags from M4 onward).

### Summary — `finance summary`

`--period 2026-04` or `--from 2026-04-01 --to 2026-05-01`.
Output: total in, total out, net; broken down by category (and parent category). `Uncategorised` rendered as a normal category line; CLI prints a footer warning if `Uncategorised` total > 0 with a hint to run `finance uncategorised`.

### Comparison — `finance compare`

`--a <period> --b <period>` (compare A vs B).
Convenience flags:
- `--wow` → `--a this-week --b last-week`
- `--mom` → `--a this-month --b last-month`
- `--yoy` → `--a this-year --b last-year`

Output: per-category A total, B total, Δ absolute, Δ %, sorted by absolute Δ desc. `--top N` truncates. Δ % is `null` (JSON) / `—` (table) when A is zero — documented in `docs/api/reporting-dtos.md`.

### Drill-down — `finance txns`

Filters: `--category`, `--merchant`, `--account`, `--from/--to` or `--period`, `--direction`, `--min`, `--max`. Sort: `--sort=amount|date|merchant`; tiebreaker is always `txn_id` ascending. Pagination: `--limit` (default 100) and `--offset` (default 0). All sorting and pagination happens in the repo (SQL `LIMIT/OFFSET`/`ORDER BY`), not in the report function.

### Period type

`internal/domain/Period` parses:
- `YYYY-MM` (calendar month)
- `YYYY-Www` (ISO 8601 week, Monday-start)
- `YYYY` (calendar year)
- Explicit `--from / --to` ranges — both interpreted as **dates** in the configured timezone, half-open: `--from 2026-04-01 --to 2026-04-30` becomes `[2026-04-01 00:00, 2026-05-01 00:00)` in the configured tz.
- Relative: `this-week`, `last-week`, `this-month`, `last-month`, `this-year`, `last-year`. ISO weeks. "This week" = current ISO week (Mon–Sun).

`Resolve(loc *time.Location, now time.Time)` returns a `Range{From, To time.Time}`. Timezone is **passed in**, not hardcoded — the use case supplies it from config (default `Pacific/Auckland`). DST transitions are wall-clock-anchored: `--period 2026-04` is 30 calendar days regardless of the DST shift. Tests enumerate concrete cases:

- ISO week 53 in years that have one (2020, 2026 does not have W53 — error).
- DST start in NZ (last Sunday of September) and end (first Sunday of April).
- Year boundaries when ISO week 1 starts in the previous calendar year.

### Rendering

`internal/render/Renderer[T]` (Go 1.18+ generics where it helps; or one interface per result type if generics are clumsy):

```go
type SummaryRenderer interface {
    Render(w io.Writer, result report.SummaryResult) error
}
type CompareRenderer interface { Render(w io.Writer, result report.CompareResult) error }
type TxnsRenderer    interface { Render(w io.Writer, result []report.TxnRow) error }
```

Implementations: `Table`, `CSV`, `JSON`, `Markdown`. CLI picks via `--format`. API later always picks JSON. **All DTOs in `report` carry `json:"..."` tags from M4** so the JSON renderer (and M7 API) emit stable, snake_case field names.

## 8. Configuration & secrets

### Config file

`~/.config/finance-analysis/config.yaml` (override with `--config`):

```yaml
database:
  url_env: DATABASE_URL          # secret stays in env; file holds only the env name
akahu:
  app_token_env: AKAHU_APP_TOKEN
  user_token_env: AKAHU_USER_TOKEN
timezone: Pacific/Auckland
categories_file: ./config/categories.yaml
rules_file: ./config/rules.yaml
# master_key_env added in M8a (encryption not used before then)
```

The config file references env var **names**, never values.

### Precedence

For a given setting: **command-line flag** > **environment variable** > **config file** > **built-in default**. Documented in `--help` and in the M5 polish brief. A required setting that is unresolved (e.g. `DATABASE_URL` not set, no flag, no default) errors at startup with a message naming the setting and how to provide it.

### Secret handling rules (enforced from M1)

1. **No secret in any file checked into git.** Includes `config.yaml`, `.env`, `rules.yaml` (may leak merchant patterns), `categories.yaml` (may leak category structure), Akahu fixtures with real txn data.
2. **`.env` is gitignored;** `.env.example` ships only key names.
3. **`config/{rules,categories,config}.yaml` are gitignored;** corresponding `*.example.yaml` files ship as templates.
4. **Akahu fixtures captured from real account data are gitignored;** synthetic fixtures (`internal/akahu/fixtures/*.synthetic.json`) are committed.
5. **Tokens never appear in logs.** Slog handler does redaction in two layers: (a) `replace_attr` clears values for keys matching `(?i)token|authorization|password|secret|api[_-]?key`; (b) a value-side scanner replaces token-shaped substrings (`(?i)\bbearer\s+\S+`, `(?i)\bapp_token=\S+`, etc.) inside any string-typed attr. Tested with a golden test that captures real adapter error strings and asserts no token leaks through.
6. **Errors never wrap raw token values.** Adapter errors include status code + URL + body excerpt run through the redaction filter before wrapping.
7. **`raw_json` from Akahu never enters logs or error messages.** It is database-only. Adapters parse and discard, or store and move on. Asserted by a unit test that grep's the error-formatting paths for `raw_json` references.
8. **Database URL with password** is read from env, not the config file.
9. **Master encryption key** (introduced in M8a) is the input to the `KeyProvider` port. In dev it's an env var; in production multi-user, KMS or sealed-secret. Rotation strategy in `docs/architecture/security.md`.
10. **Pre-commit hook** — `gitleaks` runs on `git commit` to catch accidental secret commits. Configured in M5.

### Token encryption (M8a onward)

When `akahu_tokens` is introduced in M8a:

- Each ciphertext column has its **own nonce column** (`app_token_nonce bytea`, `user_token_nonce bytea`). **Never reuse a nonce with the same key across two plaintexts** — that breaks AES-GCM catastrophically.
- Nonces are 96-bit cryptographically-random, generated fresh on every encryption.
- Schema includes `key_version int NOT NULL DEFAULT 1` to support online rotation.
- `KeyProvider.MasterKey(ctx, version)` lets the decrypt path try the current key, then fall back to the previous key during rotation. Decryption is the only place where multi-version awareness lives.

### Backups

Postgres dumps must be encrypted before leaving the host. The Makefile provides `make backup` that pipes `pg_dump` through `age -r <recipient>`; recipient key location documented in M5. No unencrypted dump target exists.

## 9. Privacy & PII

Transactions contain PII (merchant, location, amounts, behavioural patterns). The system treats them accordingly:

- **No transaction data in logs at any level.** Sync logs report counts only (`synced 42 txns for account ANZ-***`). No amounts, no descriptions, no merchants. Account IDs are masked beyond the bank prefix.
- **No transaction data in error messages** that propagate beyond the use-case boundary. Internal errors may include an opaque txn ID for debugging.
- **`raw_json` is database-only.** See §8.7.
- **No transmission to third parties** beyond Akahu. No analytics, no error reporting SaaS unless the user opts in and PII is scrubbed first.
- **Right to delete:** M8b includes `DELETE /users/me` and `finance admin delete-user --id <n>` that hard-delete the user. Because every user-owned table cascades from `users(id)`, this is a single `DELETE FROM users WHERE id = $1` inside one transaction. Verify with an integration test seeding two users, deleting one, asserting zero rows for the deleted user across every table.

## 10. Observability

- `log/slog` text handler in CLI; JSON handler later in API.
- Levels: INFO normal, DEBUG behind `--verbose`.
- Errors wrapped with context: `fmt.Errorf("ingest: fetching txns since %s: %w", since, err)`.
- Sentinel errors only when callers need to branch.
- `finance --version` prints the build version.
- `finance health` runs a DB ping and an Akahu reachability check, returns non-zero on failure. Useful for cron and for the M5 README verification step.

## 11. Testing — TDD throughout

Red → Green → Refactor for every package. No production code without a failing test first. **Tests are specifications** and are never modified to make a failing run pass without explicit reasoning (per [development-rule](https://github.com/anh-pham191/development-rule)'s agent-safety skill).

| Layer | Style | Notes |
|---|---|---|
| `domain/`, `categorise/`, `report/`, `ingest/` use cases | Pure unit tests, in-memory fakes for ports | Tests drive design |
| `storage/postgres/` | Integration against real Postgres via testcontainers-go | `make test-integration` |
| `akahu/` | HTTP fixtures via `httptest`, plus retry/timeout/malformed-response cases | Recorded responses |
| `cmd/cli/` | End-to-end with cobra `SetArgs`, captured stdout, testcontainers Postgres | Behaviour-level |

Coverage is not a target. Every behaviour in this spec must have a test that fails if the behaviour breaks.

**Cross-tenant safety tests (mandatory):** every milestone that adds a repository method also adds an integration test that:
1. Seeds two users with overlapping data shapes.
2. Calls the new method as user A.
3. Asserts no row from user B is in the result and no row of user B's was modified.

**Architecture test (mandatory from M1):** `internal/archtest/archtest_test.go` walks the package import graph and fails if any forbidden import (§3) appears. Cheap insurance against the layout drifting.

## 12. Tooling

- Go 1.23+
- `cobra` for CLI
- `golang-migrate` for migrations
- `sqlc` for type-safe queries (SQL lives in `.sql` files)
- `golangci-lint`
- `testcontainers-go` for integration tests
- `godotenv` for dev `.env` loading
- `shopspring/decimal` for money (chosen in `docs/architecture/overview.md`)
- `age` for encrypted backups
- `gitleaks` for pre-commit secret scanning (M5)
- No `viper` until config genuinely needs it

## 13. Milestones

Each milestone ends green: tests pass, lint passes, CLI command works against real Postgres. No half-finished milestones. **Multi-tenancy invariants (user_id on every row, FORCE RLS, scoped repository methods, redacted logs) apply from M1 onward** — they are not deferred.

1. **M1 — Skeleton & DB.** Repo layout, Docker Postgres, migrations (users + accounts + transactions + categories + rules + category_assignments + sync_state) with FORCE RLS, dedicated `finance_app` non-owner role, seed dev user, domain types incl. `UserID` and `Money`, per-aggregate repo ports, Postgres impls with `withUserTx` helper, archtest, `finance migrate`, `finance --version`, `finance health`.
2. **M2 — Akahu ingest.** `AkahuClient` adapter with retry/backoff/jitter, `EnvTokenStore`, `Sync(ctx, userID, ...)` use case, `finance sync`, idempotent upserts, `sync_state`, 30d default first-sync lookback, redaction tests for tokens in logs and errors. **Westpac smoke-test acceptance:** at end of M2, the user connects Westpac in their Akahu dashboard and re-runs `finance sync`; M2 acceptance includes "Westpac transactions appear without code changes" — not a separate milestone.
3. **M3 — Categorisation.** `categories.yaml` + `rules.yaml` loaders (separate concerns), rule engine (table-driven), reconciliation, `categorise`/`recat`/`unrecat`/`uncategorised`. Convergent re-runs.
4. **M4 — Reporting MVP.** `Period` parser with injected `*time.Location` (DST-correct), `summary`/`compare`/`txns` (paginated, sorted, with tiebreaker), `io.Writer`-based renderers with JSON tags on DTOs.
5. **M5 — Polish.** Structured logging with redaction, friendly error catalogue (numbered list of N specific scenarios with expected messages, all tested), `--format` outputs polished, config & secret loading per §8 precedence, `gitleaks` pre-commit, `make backup` (encrypted), README with verification step ending in `finance summary --period this-month`.
6. ~~M6 — Westpac~~ — **demoted to a smoke-test acceptance criterion under M2** (above).
7. **M7 — HTTP API.** `cmd/api` reusing every use case, `Authenticator` port + `EnvBearerAuthenticator` (single static token), CSRF/CORS/rate-limit deferral documented explicitly so middleware ordering survives M8 extension. Same DTOs → JSON.
8. **M8a — Auth & encrypted token store.** `users.password_hash` migration (argon2id, OWASP params documented), `sessions` table (token hash stored, never plaintext), `SessionAuthenticator`, registration / login / logout endpoints, `akahu_tokens` migration with `key_version` and per-ciphertext nonces, `KeyProvider` port + `EnvKeyProvider`, `DBTokenStore`, onboarding endpoints (`POST /akahu/tokens`, `DELETE /akahu/tokens`). Cross-tenant tests for every endpoint.
9. **M8b — Rotation, rules-in-DB, audit, deletion.** `finance admin rotate-keys`, rules become DB-primary (yaml becomes export/import only), `audit_log` table + writes on auth/token/delete events, `DELETE /users/me` + `finance admin delete-user`, `govulncheck` in CI.

Per-milestone docs live under `docs/milestones/M{n}-*.md` and contain the full context any agent needs to work on that milestone in isolation.

## 14. Constraints from [development-rule](https://github.com/anh-pham191/development-rule)

- **NZ English** in user-facing copy and docs (e.g. "categorise", "behaviour").
- **Never commit without explicit human approval.**
- **Never modify tests without permission** — tests are specifications.
- **Pause before destructive operations.**
- **Composable over monolithic.** Small, focused packages with one clear purpose.
- **Tool-agnostic at the core.** Domain/use cases bind to no provider.
- **Evolved from use.** Don't add features the user hasn't asked for. YAGNI ruthlessly.
- Commit subject in imperative mood, ≤72 chars; body explains *why*.
- All work via PR into `develop`; releases via `develop → main` PR (see `docs/process/branching.md`).

## 15. Open questions

None blocking implementation start. Future decisions to revisit when relevant:

- Will rules need OR-semantics or rule groups? (revisit after a month of real use)
- Should manual overrides expire or stick forever? (current: stick forever)
- Should we model account hierarchies (joint vs personal)? (defer until needed)
- Where does the production master key live in M8b? (KMS vs sealed-secret vs env on hardened host — decide at M8a/b kickoff based on hosting choice)
- Self-hosted multi-user vs SaaS multi-user — does the user run one shared instance for friends/family, or do users self-host? (default: self-hosted shared instance)
- When Akahu issues a transaction correction (different amount for the same `id`), do we overwrite, version, or audit? Current: keep `amount` stable, refresh `raw_json`. Revisit if this happens in practice.
- When Akahu returns an account no longer present locally (e.g. user un-linked it), do we delete, hide, or keep? Current: keep, mark `enabled=false` once that flag exists.
- Whether to vendor `golang-migrate` as a library or shell out — decide during M1 plan.
