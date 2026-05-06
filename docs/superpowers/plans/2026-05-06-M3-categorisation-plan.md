# M3 Categorisation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add YAML-driven transaction categorisation with a declared category taxonomy, deterministic rule matching, manual overrides, convergent re-runs, and CLI commands for categorise/recat/unrecat/uncategorised.

**Architecture:** Keep `internal/categorise` pure and adapter-free. YAML loaders parse local config into categorise package types; the use case depends only on repository ports and domain types. Postgres remains the only storage adapter and continues to enforce `user_id`/RLS through M1 repository patterns.

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, regexp, cobra, M1/M2 Postgres repositories, local Docker Postgres via `make db-up`.

---

## Branch

Current branch for this plan: `feature/m3-plan`, branched from updated `develop`.

Implementation branch after plan review: `feature/m3-categorisation`.

## Execution Cadence

Execute exactly one task per chat/session unless the user explicitly asks to continue. Each task ends with red/green evidence, review gates, staged diff/stat, and a task-sized commit before the next task starts.

---

## File Map

| Path | Responsibility |
|---|---|
| `config/categories.example.yaml` | Example taxonomy with `kind` and optional parent |
| `config/rules.example.yaml` | Example rules referencing category names |
| `internal/categorise/types.go` | Config-facing `Category`, `Rule`, `Predicate`, `Assignment` types |
| `internal/categorise/loader.go` | YAML loaders and validation |
| `internal/categorise/predicate.go` | Predicate matching against `domain.Transaction` |
| `internal/categorise/apply.go` | Deterministic first-match assignment |
| `internal/categorise/categorise.go` | Reconcile categories/rules and assign non-manual transactions |
| `internal/categorise/*_test.go` | Unit tests with in-memory fakes |
| `internal/ports/category.go` | Extend category repo methods |
| `internal/ports/rule.go` | Extend rule repo methods |
| `internal/ports/assignment.go` | Extend assignment repo methods |
| `internal/ports/txn.go` | Add list methods needed by categorise/uncategorised |
| `internal/storage/postgres/{category,rule,assignment,txn}.go` | Implement new port methods |
| `internal/storage/postgres/*_test.go` | Cross-tenant and convergence integration tests |
| `cmd/cli/{categorise,recat,unrecat,uncategorised}.go` | M3 CLI commands |
| `cmd/cli/*_test.go` | CLI behaviour tests |
| `docs/STATUS.md` | Mark M3 plan/implementation status |

---

## Spec Coverage

| Requirement | Task(s) |
|---|---|
| Example YAML files | Task 1 |
| Category/rule loaders and validation | Tasks 1-2 |
| Predicate fields and AND semantics | Task 3 |
| Deterministic first-match/tiebreak | Task 4 |
| Category reconciliation, never delete categories | Task 6 |
| Rule upsert and delete-missing | Tasks 5-6 |
| Manual assignments survive | Tasks 6, 9 |
| Convergent categorise re-runs | Tasks 6, 10 |
| CLI categorise/recat/unrecat/uncategorised | Tasks 7-9 |
| Cross-tenant isolation | Task 10 |
| Real-data smoke | Task 11 |

---

### Task 1: Example YAML and Config Types

**Files:**
- Create: `config/categories.example.yaml`
- Create: `config/rules.example.yaml`
- Create: `internal/categorise/types.go`
- Create: `internal/categorise/types_test.go`

- [ ] **Step 1: Write type tests**

Create tests asserting valid category kinds are `income`, `expense`, `transfer` and assignment sources used by categorise are `RULE`, `MANUAL`, `AKAHU`.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/categorise -run TestTypes -count=1`

Expected: FAIL because `internal/categorise` does not exist.

- [ ] **Step 3: Add YAML examples**

`config/categories.example.yaml`:

```yaml
- name: Income/Salary
  kind: income
- name: Food
  kind: expense
- name: Food/Groceries
  kind: expense
  parent: Food
- name: Food/Eating-out
  kind: expense
  parent: Food
- name: Transfers
  kind: transfer
- name: Uncategorised
  kind: expense
```

`config/rules.example.yaml`:

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

- [ ] **Step 4: Add types**

Define:

```go
type CategoryKind string
const (KindIncome CategoryKind = "income"; KindExpense = "expense"; KindTransfer = "transfer")
type Category struct { Name string; Kind CategoryKind; Parent string }
type Predicate struct { Direction string; AmountMin *domain.Money; AmountMax *domain.Money; DescriptionMatches string; MerchantIn []string; MerchantMatches string; AccountIn []string; AkahuCategory string }
type Rule struct { Name string; Priority int; Predicate Predicate; Category string; Enabled *bool }
func (r Rule) IsEnabled() bool { return true when Enabled is nil, otherwise *Enabled }
type Assignment struct { Category string; Source domain.AssignmentSource; RuleName string }
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/categorise -run TestTypes -count=1`

Commit: `Add categorisation config types`

---

### Task 2: YAML Loaders and Validation

**Files:**
- Create: `internal/categorise/loader.go`
- Create: `internal/categorise/loader_test.go`

- [ ] **Step 1: Write failing loader tests**

Cover:
- valid categories YAML loads,
- missing/invalid kind errors,
- orphan parent errors,
- duplicate category names error,
- valid rules YAML loads against loaded categories,
- missing category reference errors,
- duplicate rule names error,
- bad regex errors for `description_matches`/`merchant_matches`.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/categorise -run TestLoad -count=1`

Expected: FAIL with missing `LoadCategories`/`LoadRules`.

- [ ] **Step 3: Implement**

Use `yaml.v3`; add dependency with `go get gopkg.in/yaml.v3` if not already present. Validate all references at load time. Error strings may include config names and rule/category names; no transaction PII involved.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/categorise -run TestLoad -count=1`

Commit: `Add categorisation YAML loaders`

---

### Task 3: Predicate Matching

**Files:**
- Create: `internal/categorise/predicate.go`
- Create: `internal/categorise/predicate_test.go`

- [ ] **Step 1: Write failing table tests**

One case per field:
- `direction`
- `amount_min`
- `amount_max`
- `description_matches`
- `merchant_in`
- `merchant_matches`
- `account_in`
- `akahu_category`
- combination case proving AND semantics.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/categorise -run TestPredicate -count=1`

Expected: FAIL with missing matcher.

- [ ] **Step 3: Implement**

Add:

```go
func (p Predicate) Match(txn domain.Transaction) bool
```

Compile regex in loader, or store compiled internal fields if needed. Keep runtime matching deterministic and side-effect free.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/categorise -run TestPredicate -count=1`

Commit: `Add categorisation predicates`

---

### Task 4: Rule Application

**Files:**
- Create: `internal/categorise/apply.go`
- Create: `internal/categorise/apply_test.go`

- [ ] **Step 1: Write failing tests**

Cover:
- first matching enabled rule wins,
- disabled rules skipped,
- lower priority number wins,
- equal priority tiebreaks by `name` ascending,
- no explicit match returns `false` with no category/rule assignment.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/categorise -run TestApply -count=1`

Expected: FAIL with missing `Apply`.

- [ ] **Step 3: Implement**

Add:

```go
func Apply(txn domain.Transaction, rules []Rule) (Assignment, bool)
```

Return `false` with no category/rule assignment only when no explicit rule matched; the use case maps that to `Uncategorised`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/categorise -run TestApply -count=1`

Commit: `Add categorisation rule application`

---

### Task 5: Extend Ports and Postgres Repositories

**Files:**
- Modify: `internal/ports/{category,rule,assignment,txn}.go`
- Modify: `internal/storage/postgres/{category,rule,assignment,txn}.go`
- Modify/Create: `internal/storage/postgres/*_test.go`

- [ ] **Step 1: Write failing integration tests**

Add tests for:
- category upsert by name and list/find-by-name,
- rule upsert by name, list enabled/all, delete missing by names,
- assignment get by txn, upsert only on change, clear manual, list uncategorised,
- txn list for categorisation,
- cross-tenant isolation for every new repo method.

- [ ] **Step 2: Verify red**

Run: `set -a; . ./.env; set +a; go test -tags=integration ./internal/storage/postgres -run 'TestCategory|TestRule|TestAssignment|TestTxn' -count=1`

Expected: FAIL with missing methods.

- [ ] **Step 3: Implement**

Every method takes `userID domain.UserID` as first non-context arg and uses `withUserTx`. Return `ports.ErrNotFound` for missing rows.

- [ ] **Step 4: Verify and commit**

Run: `set -a; . ./.env; set +a; go test -tags=integration ./internal/storage/postgres -count=1`

Commit: `Extend Postgres repos for categorisation`

---

### Task 6: Categorise Use Case

**Files:**
- Create: `internal/categorise/categorise.go`
- Create: `internal/categorise/categorise_test.go`

- [ ] **Step 1: Write failing use-case tests with fakes**

Cover:
- first run creates assignments,
- second run with same inputs writes zero assignment changes,
- rule category edit updates assignment and `assigned_at`,
- removed rule is pruned and affected txn falls to another rule or `Uncategorised`,
- `MANUAL` assignment survives rule edit/deletion.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/categorise -run TestCategorise -count=1`

Expected: FAIL with missing `Categorise`.

- [ ] **Step 3: Implement**

Define:

```go
type Deps struct { Categories ports.CategoryRepo; Rules ports.RuleRepo; Assignments ports.AssignmentRepo; Txns ports.TxnRepo; Clock ports.Clock }
type Config struct { Categories []Category; Rules []Rule }
func Categorise(ctx context.Context, userID domain.UserID, deps Deps, cfg Config) error
```

Only write assignment rows when category/rule changes. Never touch `MANUAL`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/categorise -run TestCategorise -count=1`

Commit: `Add categorisation use case`

---

### Task 7: CLI `finance categorise`

**Files:**
- Create: `cmd/cli/categorise.go`
- Create: `cmd/cli/categorise_test.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Cover:
- default config paths `config/categories.yaml` and `config/rules.yaml`,
- custom `--categories` and `--rules` paths,
- loader error surfaces clearly,
- runner injection called with parsed config.

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run TestCategoriseCommand -count=1`

Expected: FAIL with missing command.

- [ ] **Step 3: Implement**

Wire DB/repos like `sync` command. Use local `UserID(1)`. Do not read env outside `cmd/cli`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/cli -run TestCategoriseCommand -count=1`

Commit: `Add categorise CLI command`

---

### Task 8: CLI Manual Override Commands

**Files:**
- Create: `cmd/cli/recat.go`
- Create: `cmd/cli/unrecat.go`
- Create: `cmd/cli/recat_test.go`
- Create: `cmd/cli/unrecat_test.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Cover:
- `recat <txn-id> <category>` sets `MANUAL`,
- unknown category errors clearly and includes candidate category names,
- `unrecat <txn-id>` clears `MANUAL` so next categorise can re-evaluate.

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run 'TestRecat|TestUnrecat' -count=1`

Expected: FAIL with missing commands.

- [ ] **Step 3: Implement**

Use category loader for declared categories; never implicitly create category names from CLI input.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/cli -run 'TestRecat|TestUnrecat' -count=1`

Commit: `Add manual recategorisation commands`

---

### Task 9: CLI `uncategorised`

**Files:**
- Create: `cmd/cli/uncategorised.go`
- Create: `cmd/cli/uncategorised_test.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Cover:
- only transactions assigned to `Uncategorised` are listed,
- output avoids token data and does not include raw JSON,
- empty list prints a clear message.

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run TestUncategorised -count=1`

Expected: FAIL with missing command.

- [ ] **Step 3: Implement**

Keep output simple for M3; richer table/CSV/JSON comes in M4.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/cli -run TestUncategorised -count=1`

Commit: `Add uncategorised CLI command`

---

### Task 10: End-to-End Integration and Smoke

**Files:**
- Create: `internal/categorise/categorise_integration_test.go`
- Modify: `README.md`
- Modify: `docs/STATUS.md`

- [ ] **Step 1: Add integration tests**

Use local Docker DB:
- seed user, account, txns,
- reconcile config,
- categorise twice and prove second run changes zero rows,
- manual assignment survives,
- cross-tenant categories/rules/assignments are isolated.

- [ ] **Step 2: Verify integration**

Run: `set -a; . ./.env; set +a; make test-integration`

Expected: PASS.

- [ ] **Step 3: Add real-data smoke instructions**

README commands:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli categorise
go run ./cmd/cli uncategorised
```

Add note: write one rule, rerun, uncategorised count should shrink.

- [ ] **Step 4: Final verification**

Run:

```bash
make test
set -a; . ./.env; set +a; make test-integration
make lint
```

- [ ] **Step 5: Commit**

Commit: `Verify M3 categorisation workflow`

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-06-M3-categorisation-plan.md`.

Preferred execution: one-task-per-session subagent-driven execution. For each session, execute one task only, run review gates, commit that task, and stop.
