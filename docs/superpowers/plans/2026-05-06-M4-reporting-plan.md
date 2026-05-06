# M4 Reporting MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CLI reporting for summary, comparison, and transaction drill-down with tested period parsing, user-scoped report queries, stable JSON DTOs, and writer-based table/CSV/JSON/Markdown renderers.

**Architecture:** `internal/report` owns pure report logic over ports and domain types. `internal/render` owns `io.Writer` output formatting and never writes directly to stdout. `cmd/cli` only parses flags, opens the app database, constructs Postgres repos, calls report functions, and chooses a renderer. Postgres gets query methods only where SQL-side sorting/pagination is required.

**Tech Stack:** Go 1.23, `shopspring/decimal` via `domain.Money`, cobra, `encoding/json`, `encoding/csv`, local Docker Postgres integration tests, no new database migrations.

---

## Branch

Current branch for this plan: `feature/m4-plan`, branched from updated `develop`.

Implementation branch after plan review: `feature/m4-reporting`.

## Execution Cadence

Execute exactly one task per chat/session unless the user explicitly asks to continue. Each task ends with red/green evidence, review gates, staged diff/stat, and a task-sized commit before the next task starts.

---

## File Map

| Path | Responsibility |
|---|---|
| `internal/domain/period.go` | Period parser and `Range` resolution with injected location/now |
| `internal/domain/period_test.go` | Month/week/year/relative/explicit range/DST/ISO-week tests |
| `internal/ports/reporting.go` | Narrow report query ports if existing repos become too broad |
| `internal/storage/postgres/txn.go` | SQL-side transaction filter/sort/pagination methods |
| `internal/storage/postgres/reporting_test.go` | Cross-tenant report query integration tests |
| `internal/report/types.go` | JSON-tagged DTOs for summary, compare, txns |
| `internal/report/summary.go` | Summary totals and per-category aggregation |
| `internal/report/compare.go` | A/B category comparison, delta, percent, top N |
| `internal/report/transactions.go` | Drill-down query orchestration |
| `internal/report/*_test.go` | Unit tests with fakes |
| `internal/render/*.go` | Table/CSV/JSON/Markdown renderers for all report types |
| `internal/render/*_test.go` | Golden-ish renderer tests using stable strings |
| `cmd/cli/{summary,compare,txns}.go` | CLI commands and flag parsing |
| `cmd/cli/{summary,compare,txns}_test.go` | CLI parser/runner injection tests |
| `docs/api/reporting-dtos.md` | Stable JSON schema documentation |
| `README.md`, `docs/STATUS.md` | Smoke instructions and milestone status |

---

## Decisions Locked In

| Topic | Choice | Rationale |
|---|---|---|
| Period ranges | Half-open `[from, to)` | Matches spec and SQL filtering. |
| Timezone | CLI default `Pacific/Auckland`; report/domain accept `*time.Location` | Avoids hardcoding timezones in domain/report packages. |
| Summary/compare aggregation | In report layer over repo rows for M4 | Simpler and testable; SQL optimisation can come after behaviour is locked. |
| Transaction drill-down | Sorting/pagination in Postgres repo | Spec requires SQL-side pagination/sort for potentially large result sets. |
| Output formats | Separate renderer structs per report/format | Avoids clever generic abstractions while keeping `io.Writer` boundary. |
| JSON tags | DTOs carry snake_case tags from first implementation | Prevents M7 API churn. |
| PII | Report output may intentionally include transaction row fields for `txns`; logs/errors still must not | User-requested report output is not logging. |

---

## Spec Coverage

| Requirement | Task(s) |
|---|---|
| Period parsing and DST/ISO week tests | Task 1 |
| Summary totals/per-category including Uncategorised | Task 3 |
| Compare deltas, nil delta percent, top N | Task 4 |
| Txn filtering, SQL sorting/pagination/tiebreaker | Tasks 2, 5 |
| JSON-tagged DTOs | Task 3 |
| Renderers table/csv/json/md | Task 6 |
| CLI summary/compare/txns and `--format` | Task 7 |
| DTO schema docs | Task 8 |
| Cross-tenant report tests | Task 9 |
| Real-data smoke | Task 10 |

---

### Task 1: Period Parsing and Ranges

**Files:**
- Create: `internal/domain/period.go`
- Create: `internal/domain/period_test.go`

- [ ] **Step 1: Write failing tests**

Cover:
- `2026-04` resolves to April month in `Pacific/Auckland`.
- `2026` resolves calendar year.
- `2020-W53` resolves valid ISO week.
- `2026-W53` errors.
- `this-week`, `last-week`, `this-month`, `last-month`, `this-year`, `last-year` from fixed `now`.
- explicit date range helper turns `--from 2026-04-01 --to 2026-04-30` into `[2026-04-01 00:00, 2026-05-01 00:00)` in `loc`.
- DST start/end months are wall-clock anchored in `loc`.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/domain -run TestPeriod -count=1`

Expected: FAIL with missing `Period` / `Range`.

- [ ] **Step 3: Implement**

Add:

```go
type Range struct { From time.Time; To time.Time }
type Period struct { value string; explicitFrom *time.Time; explicitTo *time.Time }
func ParsePeriod(value string) (Period, error)
func ExplicitPeriod(from, to time.Time) Period
func (p Period) Resolve(loc *time.Location, now time.Time) (Range, error)
```

Explicit `to` is a date inclusive at CLI level, so `Resolve` adds one calendar day for half-open SQL range.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/domain -run TestPeriod -count=1
go test ./internal/domain -count=1
```

Commit: `Add reporting period parser`

---

### Task 2: Reporting Query Ports and Postgres Txn Filtering

**Files:**
- Modify: `internal/ports/txn.go` or create `internal/ports/reporting.go`
- Modify: `internal/storage/postgres/txn.go`
- Create/modify: `internal/storage/postgres/reporting_test.go`

- [ ] **Step 1: Write failing integration tests**

Define a drill-down filter type and tests for:
- date range `[from,to)`,
- category filter through `category_assignments`,
- merchant filter,
- account filter,
- direction filter,
- min/max amount,
- sort `amount`, `date`, `merchant` with `txn_id` ASC tiebreaker,
- `limit`/`offset`, including offset past end,
- cross-tenant isolation.

- [ ] **Step 2: Verify red**

Run:

```bash
set -a; . ./.env; set +a
go test -tags=integration ./internal/storage/postgres -run TestReportingTxnQuery -count=1
```

Expected: FAIL with missing filter/query method.

- [ ] **Step 3: Implement**

Prefer a narrow port:

```go
type TxnFilter struct {
  Range domain.Range
  CategoryID *int64
  Merchant string
  AccountID string
  Direction *domain.Direction
  Min *domain.Money
  Max *domain.Money
  Sort string
  Limit int
  Offset int
}
type TxnQueryRepo interface { ListFiltered(ctx context.Context, userID domain.UserID, filter TxnFilter) ([]domain.Transaction, error) }
```

Use parameter placeholders only. Validate sort names in Go and map to fixed SQL fragments.

- [ ] **Step 4: Verify and commit**

Run:

```bash
set -a; . ./.env; set +a
go test -tags=integration ./internal/storage/postgres -run TestReportingTxnQuery -count=1
go test ./internal/ports ./internal/archtest -count=1
```

Commit: `Add reporting transaction queries`

---

### Task 3: Summary Report

**Files:**
- Create: `internal/report/types.go`
- Create: `internal/report/summary.go`
- Create: `internal/report/summary_test.go`

- [ ] **Step 1: Write failing tests**

Use fakes returning transactions, categories, assignments. Cover:
- income/expense totals,
- net,
- per-category totals,
- Uncategorised line included,
- JSON tags on DTO fields use snake_case.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/report -run TestSummary -count=1`

Expected: FAIL with missing package/functions.

- [ ] **Step 3: Implement**

Add DTOs:

```go
type MoneyAmount string
type SummaryResult struct { Period domain.Range `json:"period"`; Income MoneyAmount `json:"income"`; Expense MoneyAmount `json:"expense"`; Net MoneyAmount `json:"net"`; Categories []CategoryTotal `json:"categories"`; HasUncategorised bool `json:"has_uncategorised"` }
type CategoryTotal struct { CategoryID int64 `json:"category_id"`; Category string `json:"category"`; Kind string `json:"kind"`; Total MoneyAmount `json:"total"` }
```

Keep deterministic category ordering by category name.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/report -run TestSummary -count=1`

Commit: `Add summary report`

---

### Task 4: Compare Report

**Files:**
- Create: `internal/report/compare.go`
- Create: `internal/report/compare_test.go`
- Modify: `internal/report/types.go`

- [ ] **Step 1: Write failing tests**

Cover:
- A/B totals per category,
- delta absolute,
- delta percent,
- nil delta percent when A is zero,
- sort by absolute delta descending,
- `Top` truncation.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/report -run TestCompare -count=1`

Expected: FAIL with missing `Compare`.

- [ ] **Step 3: Implement**

Build on summary aggregation helpers where practical, but avoid premature abstraction if it obscures tests.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/report -run TestCompare -count=1`

Commit: `Add comparison report`

---

### Task 5: Transaction Drill-Down Report

**Files:**
- Create: `internal/report/transactions.go`
- Create: `internal/report/transactions_test.go`
- Modify: `internal/report/types.go`

- [ ] **Step 1: Write failing tests**

Cover:
- filter maps to repo filter,
- limit default 100 when zero,
- offset passed through,
- sort passed through,
- DTO JSON tags for rows.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/report -run TestTransactions -count=1`

Expected: FAIL with missing `Transactions`.

- [ ] **Step 3: Implement**

Return `[]TxnRow` with fields: `txn_id`, `posted_at`, `account_id`, `category`, `direction`, `amount`, `merchant`, `description`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/report -run TestTransactions -count=1`

Commit: `Add transaction drill-down report`

---

### Task 6: Renderers

**Files:**
- Create: `internal/render/types.go`
- Create: `internal/render/summary.go`
- Create: `internal/render/compare.go`
- Create: `internal/render/transactions.go`
- Create: `internal/render/*_test.go`

- [ ] **Step 1: Write failing renderer tests**

For each result type and format:
- JSON emits stable snake_case fields.
- CSV has header row.
- Markdown has pipe table.
- Table format is human-readable and writes to provided `io.Writer`.
- Summary table includes footer warning when `HasUncategorised`.
- Compare renders nil delta percent as `—` in table/markdown and empty CSV cell.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/render -count=1`

Expected: FAIL with missing package/types.

- [ ] **Step 3: Implement**

Use writer-only output. No `fmt.Println` or `os.Stdout`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/render -count=1`

Commit: `Add report renderers`

---

### Task 7: Reporting CLI Commands

**Files:**
- Create: `cmd/cli/summary.go`
- Create: `cmd/cli/compare.go`
- Create: `cmd/cli/txns.go`
- Create: `cmd/cli/{summary,compare,txns}_test.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Cover:
- `summary --period 2026-04 --format json`,
- `compare --mom`,
- `compare --wow`, `compare --yoy`,
- `txns --category Food/Groceries --period last-month --limit 50 --sort amount`,
- invalid `--format` errors,
- runner injection receives parsed periods/filters.

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run 'TestSummary|TestCompare|TestTxns' -count=1`

Expected: FAIL with missing commands.

- [ ] **Step 3: Implement**

Open DB like existing commands, construct repos, default timezone to `Pacific/Auckland`, resolve periods, call report package, select renderer by `--format=table|csv|json|md`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/cli -run 'TestSummary|TestCompare|TestTxns' -count=1`

Commit: `Add reporting CLI commands`

---

### Task 8: DTO Schema Docs

**Files:**
- Create: `docs/api/reporting-dtos.md`

- [ ] **Step 1: Add schema doc**

Document JSON examples for:
- `SummaryResult`
- `CompareResult`
- `TxnRow`

Include `delta_percent: null` when A is zero and note all monetary fields are strings.

- [ ] **Step 2: Verify examples**

Run: `go test ./internal/render -run TestJSON -count=1`

Expected: PASS with examples aligned to renderer output.

- [ ] **Step 3: Commit**

Commit: `Document reporting DTOs`

---

### Task 9: Cross-Tenant Reporting Integration

**Files:**
- Create: `internal/report/report_integration_test.go`

- [ ] **Step 1: Write integration tests**

Seed two users with same category/txn shapes. Assert:
- summary for user A excludes user B,
- compare for user A excludes user B,
- txns for user A excludes user B,
- app-role unfiltered counts under `app.user_id` see only current tenant.

- [ ] **Step 2: Verify**

Run:

```bash
set -a; . ./.env; set +a
go test -tags=integration ./internal/report -count=1
```

- [ ] **Step 3: Commit**

Commit: `Test reporting tenant isolation`

---

### Task 10: Final Verification and Smoke Docs

**Files:**
- Modify: `README.md`
- Modify: `docs/STATUS.md`

- [ ] **Step 1: Add README smoke commands**

Add:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli summary --period this-month
go run ./cmd/cli compare --mom
go run ./cmd/cli txns --period this-month --limit 20 --sort date
```

- [ ] **Step 2: Update STATUS**

Mark M4 final verification / complete as appropriate. Do not claim real-data manual spot-check unless performed.

- [ ] **Step 3: Final verification**

Run:

```bash
make test
set -a; . ./.env; set +a; make test-integration
make lint
```

- [ ] **Step 4: Commit**

Commit: `Verify M4 reporting workflow`

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-06-M4-reporting-plan.md`.

Preferred execution: one-task-per-session subagent-driven execution. For each session, execute one task only, run review gates, commit that task, and stop.
