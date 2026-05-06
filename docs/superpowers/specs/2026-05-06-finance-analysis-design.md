# Finance Analysis — Design Spec

- **Date:** 2026-05-06
- **Author:** Anh Pham (with AI assistance)
- **Status:** Approved for planning
- **Philosophy reference:** `/Users/anhpham/Documents/Projects/script/development_rule/`

## 1. Purpose

A personal finance tool that connects to NZ bank accounts (ANZ first, Westpac later) via the Akahu personal API, ingests transactions into a local Postgres database, categorises them via user-authored rules with manual override, and produces summary, comparison, and drill-down reports from the command line.

The tool is **CLI-first** but architected so that a future HTTP API and web UI can be added without changes to the domain or storage layers.

## 2. Non-goals (MVP)

- No multi-user support. Single user, single household.
- No multi-currency. NZD only.
- No ML categorisation, no budgeting, no cashflow forecasting (M5+ candidates).
- No webhook ingestion (Akahu push). On-demand pull only.
- No web UI in this spec.

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

All money columns are `numeric(14,2)`. NZD-only for MVP; `currency` lives on `accounts` so multi-currency is a future migration, not a redesign. All timestamps are `timestamptz`. PKs are explicit (no surrogate UUIDs unless noted).

### Tables

- **`accounts`** — `id text PK` (Akahu account id), `name text`, `bank text`, `type text`, `currency text default 'NZD'`, `created_at timestamptz`.
- **`transactions`** — `id text PK` (Akahu txn id), `account_id text FK`, `posted_at timestamptz`, `amount numeric(14,2)`, `direction text check (direction in ('DEBIT','CREDIT'))`, `description text`, `merchant text`, `akahu_category text`, `raw_json jsonb`, `created_at`, `updated_at`.
- **`categories`** — `id serial PK`, `name text unique`, `parent_id int null FK categories(id)`, `kind text check (kind in ('income','expense','transfer'))`.
- **`category_assignments`** — `txn_id text PK FK transactions(id)`, `category_id int FK categories(id)`, `source text check (source in ('RULE','MANUAL','AKAHU'))`, `rule_id int null FK rules(id)`, `assigned_at timestamptz`.
- **`rules`** — `id serial PK`, `name text`, `priority int`, `predicate jsonb`, `category_id int FK`, `enabled bool default true`, `created_at`. Source of truth is `config/rules.yaml`; this table is the loaded form for joins/audit.
- **`sync_state`** — `account_id text PK FK accounts(id)`, `last_synced_at timestamptz`, `last_cursor text null`.

### Identity & idempotency

Akahu's transaction ID is the PK. Re-syncing is `INSERT ... ON CONFLICT (id) DO UPDATE` to preserve `created_at` and refresh `raw_json` if Akahu re-enriches.

## 5. Akahu sync

- **Auth:** Akahu App Token + User Token, both from env vars (`AKAHU_APP_TOKEN`, `AKAHU_USER_TOKEN`). Never in config file or repo.
- **Mode:** on-demand pull. `finance sync` fetches new transactions per account since `sync_state.last_synced_at` (with a small overlap window — e.g. 24h — to handle late-posting).
- **Adapter:** `internal/akahu/` implements `AkahuClient` over HTTP. Tested with recorded fixtures via `httptest`.
- **Use case:** `internal/ingest/Sync(ctx, repo, akahu)` orchestrates: list accounts → for each account, fetch since cursor → upsert transactions → update sync_state.

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

`~/.config/finance-analysis/config.yaml` (override with `--config`):

```yaml
database:
  url: postgres://finance:finance@localhost:5432/finance?sslmode=disable
akahu:
  app_token_env: AKAHU_APP_TOKEN
  user_token_env: AKAHU_USER_TOKEN
timezone: Pacific/Auckland
rules_file: ./config/rules.yaml
```

Secrets only via env. `.env.example` checked in; `.env` gitignored; loaded via `godotenv` in dev.

## 9. Observability

- `log/slog` text handler in CLI; JSON handler later in API.
- Levels: INFO normal, DEBUG behind `--verbose`.
- Errors wrapped with context: `fmt.Errorf("ingest: fetching txns since %s: %w", since, err)`.
- Sentinel errors only when callers need to branch.

## 10. Testing — TDD throughout

Red → Green → Refactor for every package. No production code without a failing test first. **Tests are specifications** and are never modified to make a failing run pass without explicit reasoning (per `development_rule/agents/skills/agent-safety`).

| Layer | Style | Notes |
|---|---|---|
| `domain/`, `categorise/`, `report/`, `ingest/` use cases | Pure unit tests, in-memory fakes for ports | Tests drive design |
| `storage/postgres/` | Integration against real Postgres (testcontainers-go or docker-compose) | `make test-integration` |
| `akahu/` | HTTP fixtures via `httptest` | Recorded responses |
| `cmd/cli/` | End-to-end with cobra `SetArgs`, captured stdout, testcontainers Postgres | Behaviour-level |

Coverage is not a target. Every behaviour in this spec must have a test that fails if the behaviour breaks.

## 11. Tooling

- Go 1.23+
- `cobra` for CLI
- `golang-migrate` for migrations
- `sqlc` for type-safe queries (SQL lives in `.sql` files)
- `golangci-lint`
- `testcontainers-go` for integration tests
- `godotenv` for dev `.env` loading
- No `viper` until config genuinely needs it

## 12. Milestones

Each milestone ends green: tests pass, lint passes, CLI command works against real Postgres. No half-finished milestones.

1. **M1 — Skeleton & DB.** Repo layout, Docker Postgres, migrations, domain types, `Repository` + Postgres impl, `finance migrate`.
2. **M2 — Akahu ingest.** `AkahuClient` adapter, `ingest` use case, `finance sync`, idempotent upserts, `sync_state`.
3. **M3 — Categorisation.** Rule engine, `rules.yaml` loader, assignments, `categorise`/`recat`/`uncategorised`.
4. **M4 — Reporting MVP.** `Period` parser (incl. relative + WoW/MoM/YoY), `summary`/`compare`/`txns`, renderers.
5. **M5 — Polish.** Structured logging, `--format` outputs, `.env`/config loading, README, example `rules.yaml`.
6. **M6 — Westpac.** Add account onboarding via Akahu; config-only if M2 was built right.
7. **M7 — HTTP API.** `cmd/api` reusing every use case; same DTOs → JSON.

Per-milestone docs live under `docs/milestones/M{n}-*.md` and contain the full context any agent needs to work on that milestone in isolation.

## 13. Constraints from `development_rule`

- **NZ English** in user-facing copy and docs (e.g. "categorise", "behaviour").
- **Never commit without explicit human approval.**
- **Never modify tests without permission** — tests are specifications.
- **Pause before destructive operations.**
- **Composable over monolithic.** Small, focused packages with one clear purpose.
- **Tool-agnostic at the core.** Domain/use cases bind to no provider.
- **Evolved from use.** Don't add features the user hasn't asked for. YAGNI ruthlessly.
- Commit subject in imperative mood, ≤72 chars; body explains *why*; ticket prefix when tracked.

## 14. Open questions

None blocking implementation start. Future decisions to revisit when relevant:
- Will rules need OR-semantics or rule groups? (revisit after a month of real use)
- Should manual overrides expire or stick forever? (current: stick forever)
- Should we model account hierarchies (joint vs personal)? (defer until needed)
