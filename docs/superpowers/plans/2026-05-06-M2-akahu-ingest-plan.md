# M2 Akahu Ingest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Akahu on-demand sync so `finance sync` lists accounts, fetches transactions, stores them idempotently in the M1 Postgres schema, updates per-account sync state, and never leaks tokens or transaction PII through logs or errors.

**Architecture:** Keep the M1 hexagonal layout. `internal/ingest` orchestrates ports only: `TokenStore`, `AkahuClient`, account/transaction/sync-state repos, and `Clock`. `internal/akahu` is the concrete HTTP adapter and the only non-CLI package allowed to read Akahu token env vars. CLI constructs dependencies for local user `UserID(1)` and passes them into `ingest.Sync`.

**Tech Stack:** Go 1.23, `net/http`, `httptest`, `log/slog`, cobra, M1 Postgres repositories, local Docker Postgres via `make db-up`, no new database migrations.

---

## Branch

Current branch for this plan: `feature/m2-plan`, branched from updated `develop`.

Implementation branch after plan review: `feature/m2-akahu-ingest`.

## Execution Cadence

This M2 milestone is intentionally **not** executed as one long agent session. Execute exactly one task per chat/session unless the user explicitly asks to continue.

For each task:

1. Start from a clean working tree or clearly report existing changes.
2. Follow TDD: write the failing test first, verify red, implement, verify green.
3. Run the task's verification command and any affected package tests.
4. Run a spec review and code-quality review for that task.
5. Stage only that task's files and show the staged diff/stat.
6. Commit that task before starting the next task, using the task's commit message from this plan, once the user has approved committing or has already given task-by-task commit approval for the session.
7. Stop and report the task result. The next task should start in a new chat/session unless the user explicitly says to continue.

Do not batch multiple M2 tasks into one commit. If a task review finds follow-up fixes, include those fixes in the same task commit before moving on.

---

## Decisions Locked In This Plan

| Topic | Choice | Rationale |
|---|---|---|
| Akahu DTO boundary | `internal/ports.RawAccount` and `internal/ports.RawTxn` | Keeps `internal/ingest` independent from the HTTP adapter but avoids premature domain modelling of Akahu payloads. |
| Token store | `internal/akahu.EnvTokenStore` | M2-M7 single-user runtime, explicitly documented as same tokens for every user. |
| HTTP retry tests | Use injectable sleeper and jitter function | Keeps tests fast; no real `1s,2s,4s` waits. |
| Postgres repos | Continue M1 `database/sql` repo pattern | M1 intentionally deferred generated sqlc wiring to keep Go 1.23 compatibility and avoid uninstalled local tooling. |
| Sync state timestamp | Set `last_synced_at = clock.Now()` after each account completes | Simple, deterministic, and testable. |
| Health Akahu check | If Akahu tokens are configured, check Akahu account listing; if not, report DB-only health | Keeps local `finance health` useful before token setup while still verifying Akahu when configured. |
| Logging | Add redacting `slog.Handler` before introducing Akahu errors | Token safety lands before network code. |

---

## File Map

| Path | Responsibility |
|---|---|
| `internal/ports/akahu_client.go` | `RawAccount`, `RawTxn`, `AkahuClient` port |
| `internal/ports/token_store.go` | `TokenStore` port |
| `internal/akahu/types.go` | Akahu API request/response structs local to adapter |
| `internal/akahu/retry.go` | Retry/backoff policy with injectable sleeper/jitter |
| `internal/akahu/client.go` | HTTP client, auth headers, pagination, redacted errors |
| `internal/akahu/env_token_store.go` | Env token reader; single-user weakening documented |
| `internal/akahu/*_test.go` | HTTP adapter, retry, pagination, token-store tests |
| `internal/ingest/sync.go` | `Sync(ctx, userID, deps)` orchestration |
| `internal/ingest/mapper.go` | Convert raw Akahu port DTOs to M1 domain types |
| `internal/ingest/sync_test.go` | In-memory fake repos/clients/token store/clock |
| `internal/observability/redaction.go` | Slog handler wrapper and string redaction helper |
| `internal/observability/redaction_test.go` | Key-side and value-side redaction tests |
| `internal/storage/postgres/sync_state.go` | Add `ErrNotFound` handling helpers if needed |
| `internal/storage/postgres/txn.go` | Add M2 correction/idempotency integration test |
| `cmd/cli/sync.go` | `finance sync [--from YYYY-MM-DD]` |
| `cmd/cli/health.go` | Optional Akahu reachability check when tokens configured |
| `cmd/cli/root.go` | Wire sync command |
| `.env.example` | Add Akahu token placeholders |
| `docs/STATUS.md` | Mark M2 plan written after plan PR |

---

## Spec Coverage

| Requirement | Task(s) |
|---|---|
| `AkahuClient` port | Task 1 |
| `TokenStore` port and `EnvTokenStore` | Tasks 1, 4 |
| Retry/backoff, Retry-After, context cancellation | Task 2 |
| Akahu HTTP adapter, auth, pagination, redacted errors | Task 3 |
| Token/log/error redaction | Tasks 2, 5 |
| `ingest.Sync` orchestration | Tasks 6-8 |
| 30 day first-sync default and 24h overlap | Task 7 |
| Idempotent transaction upsert/correction behaviour | Tasks 6, 9 |
| `finance sync --from` | Task 10 |
| `finance health` Akahu reachability | Task 11 |
| Cross-tenant sync path | Task 12 |
| ANZ/Westpac smoke-test acceptance docs | Task 13 |

---

### Task 1: Add Akahu and Token Ports

**Files:**
- Create: `internal/ports/akahu_client.go`
- Create: `internal/ports/token_store.go`
- Test: compile via `go test ./internal/ports/...`

- [ ] **Step 1: Add `internal/ports/akahu_client.go`**

```go
package ports

import (
	"context"
	"encoding/json"
	"time"
)

type RawAccount struct {
	ID       string
	Name     string
	Bank     string
	Type     string
	Currency string
}

type RawTxn struct {
	ID            string
	AccountID     string
	PostedAt      time.Time
	Amount        string
	Direction     string
	Description   string
	Merchant      string
	AkahuCategory string
	RawJSON       json.RawMessage
}

type AkahuClient interface {
	ListAccounts(ctx context.Context) ([]RawAccount, error)
	FetchTransactions(ctx context.Context, accountID string, since time.Time) ([]RawTxn, error)
}
```

- [ ] **Step 2: Add `internal/ports/token_store.go`**

```go
package ports

import (
	"context"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

type TokenStore interface {
	AkahuTokens(ctx context.Context, userID domain.UserID) (app string, user string, err error)
}
```

- [ ] **Step 3: Verify**

Run: `go test ./internal/ports/...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ports/akahu_client.go internal/ports/token_store.go
git commit -m "Add Akahu ingest ports"
```

---

### Task 2: Add Redaction Before Network Errors

**Files:**
- Create: `internal/observability/redaction.go`
- Create: `internal/observability/redaction_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/observability/redaction_test.go`:

```go
package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandlerRedactsTokenKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("token test", "app_token", "app_token_abc123456789")

	got := buf.String()
	if strings.Contains(got, "app_token_abc123456789") {
		t.Fatalf("log leaked token: %s", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("log = %q, want redacted marker", got)
	}
}

func TestRedactStringRemovesBearerTokens(t *testing.T) {
	t.Parallel()

	input := "Authorization: Bearer abc123def456ghi789"
	got := RedactString(input)
	if strings.Contains(got, "abc123def456ghi789") {
		t.Fatalf("RedactString leaked bearer token: %q", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Fatalf("RedactString = %q, want bearer redaction", got)
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/observability -run TestRedact -count=1`

Expected: FAIL with undefined `NewRedactingHandler` / `RedactString`.

- [ ] **Step 3: Implement minimal redaction**

Create `internal/observability/redaction.go`:

```go
package observability

import (
	"context"
	"log/slog"
	"regexp"
)

var (
	secretKeyPattern = regexp.MustCompile(`(?i)(token|authorization|password|secret|api[_-]?key)`)
	bearerPattern    = regexp.MustCompile(`(?i)Bearer\s+\S+`)
	tokenValuePattern = regexp.MustCompile(`(?i)\b(app|user)_token\s*[:=]\s*\S+`)
)

type redactingHandler struct {
	next slog.Handler
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return redactingHandler{next: next}
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		return true
	})

	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if secretKeyPattern.MatchString(attr.Key) {
		attr.Value = slog.StringValue("***")
		return attr
	}
	if attr.Value.Kind() == slog.KindString {
		attr.Value = slog.StringValue(RedactString(attr.Value.String()))
	}
	return attr
}

func RedactString(value string) string {
	value = bearerPattern.ReplaceAllString(value, "Bearer ***")
	value = tokenValuePattern.ReplaceAllString(value, "$1_token=***")
	return value
}
```

- [ ] **Step 4: Verify green**

Run: `go test ./internal/observability -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/observability
git commit -m "Add slog redaction for secrets"
```

---

### Task 3: Implement Akahu Retry Policy

**Files:**
- Create: `internal/akahu/retry.go`
- Create: `internal/akahu/retry_test.go`

- [ ] **Step 1: Write failing tests**

Create tests for:
- `429, 429, 200` succeeds after three attempts.
- `500, 500, 500, 500` returns an error after four total attempts.
- `400` fails fast with one attempt.
- `Retry-After: 2` is honoured by the sleeper.
- Context cancellation stops before another attempt.

Use this test shape in `internal/akahu/retry_test.go`:

```go
package akahu

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDoWithRetryRetries429ThenSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	policy := testRetryPolicy()
	err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)}, nil
	})
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func testRetryPolicy() retryPolicy {
	return retryPolicy{
		maxRetries: 3,
		baseDelay:  time.Second,
		jitter:     func(time.Duration) time.Duration { return 0 },
		sleep:      func(context.Context, time.Duration) error { return nil },
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/akahu -run TestDoWithRetry -count=1`

Expected: FAIL with missing `retryPolicy`.

- [ ] **Step 3: Implement retry policy**

Implement `retryPolicy.do(ctx, fn)` with:
- Retry only status `429` and `>=500`.
- Max retries means three retries after the first attempt, four total attempts.
- Delays: `baseDelay`, `2*baseDelay`, `4*baseDelay`.
- If `Retry-After` header is present, use it instead of computed delay.
- Call `sleep(ctx, delay+jitter(delay))`.

- [ ] **Step 4: Verify green**

Run: `go test ./internal/akahu -run TestDoWithRetry -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/akahu/retry.go internal/akahu/retry_test.go
git commit -m "Add Akahu retry policy"
```

---

### Task 4: Implement Akahu HTTP Client

**Files:**
- Create: `internal/akahu/client.go`
- Create: `internal/akahu/types.go`
- Create: `internal/akahu/client_test.go`

- [ ] **Step 1: Write failing tests**

Cover:
- `ListAccounts` sends `Authorization: Bearer <userToken>` and `X-Akahu-ID: <appToken>`.
- `FetchTransactions` includes account id and `since` query parameter.
- Pagination follows `cursor.next` until empty.
- Non-2xx error redacts token-shaped response content.
- Malformed transaction response returns txn id but no raw JSON body.

- [ ] **Step 2: Define constructor API in tests**

Tests should use:

```go
client := NewClient(Config{
	AppToken:  "app_token_test",
	UserToken: "user_token_test",
	BaseURL:   server.URL,
	HTTPClient: server.Client(),
})
```

- [ ] **Step 3: Verify red**

Run: `go test ./internal/akahu -run TestClient -count=1`

Expected: FAIL with missing `NewClient` / `Config`.

- [ ] **Step 4: Implement client**

Use:
- `GET /accounts`
- `GET /accounts/{accountID}/transactions?since=<RFC3339>`
- Adapter-local response structs in `types.go`.
- `json.RawMessage` copy for `RawTxn.RawJSON`.

Map amounts defensively by decoding API amount as `json.RawMessage`, trimming quotes if it is a JSON string, otherwise using the numeric token string.

- [ ] **Step 5: Verify green**

Run: `go test ./internal/akahu -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/akahu
git commit -m "Add Akahu HTTP client"
```

---

### Task 5: Add Env Token Store

**Files:**
- Create: `internal/akahu/env_token_store.go`
- Create: `internal/akahu/env_token_store_test.go`
- Modify: `.env.example`

- [ ] **Step 1: Write failing tests**

Test:
- Missing app token returns error.
- Missing user token returns error.
- With both env vars, `AkahuTokens(ctx, userID)` returns both.
- Calling with two different users returns the same values and the test name documents this M2 weakening.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/akahu -run TestEnvTokenStore -count=1`

Expected: FAIL with missing `EnvTokenStore`.

- [ ] **Step 3: Implement**

`EnvTokenStore` may read env vars directly because `internal/akahu` is one of the allowed packages.

```go
type EnvTokenStore struct{}

func (EnvTokenStore) AkahuTokens(ctx context.Context, userID domain.UserID) (string, string, error) {
	app := os.Getenv("AKAHU_APP_TOKEN")
	user := os.Getenv("AKAHU_USER_TOKEN")
	if app == "" || user == "" {
		return "", "", errors.New("Akahu tokens are not configured")
	}
	return app, user, nil
}
```

- [ ] **Step 4: Add `.env.example` placeholders**

```env
AKAHU_APP_TOKEN=
AKAHU_USER_TOKEN=
AKAHU_BASE_URL=https://api.akahu.io/v1
```

- [ ] **Step 5: Verify**

Run: `go test ./internal/akahu -run TestEnvTokenStore -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/akahu/env_token_store.go internal/akahu/env_token_store_test.go .env.example
git commit -m "Add environment Akahu token store"
```

---

### Task 6: Add Ingest Mapper

**Files:**
- Create: `internal/ingest/mapper.go`
- Create: `internal/ingest/mapper_test.go`

- [ ] **Step 1: Write failing tests**

Cover:
- Raw account maps currency empty to `NZD`.
- Raw txn maps decimal string amount to `domain.Money`.
- Unknown direction returns an error.
- Invalid amount returns an error containing txn id but not `raw_json`.

- [ ] **Step 2: Verify red**

Run: `go test ./internal/ingest -run TestMap -count=1`

Expected: FAIL with missing mapper functions.

- [ ] **Step 3: Implement**

```go
func mapAccount(raw ports.RawAccount) domain.Account
func mapTxn(raw ports.RawTxn) (domain.Transaction, error)
```

Errors may include `raw.ID`; they must not include `raw.RawJSON`, merchant, amount, or description.

- [ ] **Step 4: Verify green**

Run: `go test ./internal/ingest -run TestMap -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/mapper.go internal/ingest/mapper_test.go
git commit -m "Add Akahu ingest mappers"
```

---

### Task 7: Add Sync Use Case with In-Memory Fakes

**Files:**
- Create: `internal/ingest/sync.go`
- Create: `internal/ingest/sync_test.go`

- [ ] **Step 1: Write failing tests**

Use in-memory fakes implementing:
- `ports.AccountRepo`
- `ports.TxnRepo`
- `ports.SyncStateRepo`
- `ports.TokenStore`
- `ports.AkahuClient`
- `ports.Clock`

Test behaviours:
- First sync lists accounts, upserts accounts, fetches transactions since `clock.Now().AddDate(0,0,-30)`, upserts txns, and writes sync state.
- Second sync uses `last_synced_at - 24h`.
- `From` override uses the explicit date for all accounts.
- Client factory receives tokens from token store.

- [ ] **Step 2: Define use-case API in tests**

```go
type Deps struct {
	Accounts AccountRepo
	Txns TxnRepo
	SyncStates SyncStateRepo
	Tokens TokenStore
	NewAkahuClient func(appToken, userToken string) ports.AkahuClient
	Clock ports.Clock
}

type Options struct {
	From *time.Time
}

func Sync(ctx context.Context, userID domain.UserID, deps Deps, opts Options) error
```

Use actual package-qualified names in implementation.

- [ ] **Step 3: Verify red**

Run: `go test ./internal/ingest -run TestSync -count=1`

Expected: FAIL with missing `Sync`.

- [ ] **Step 4: Implement minimal sync**

Algorithm:
1. Resolve tokens.
2. Build Akahu client.
3. `ListAccounts`.
4. For each account:
   - Upsert mapped account.
   - Load sync state.
   - Since = `opts.From` if set; otherwise `last_synced_at - 24h`; otherwise `clock.Now().AddDate(0,0,-30)`.
   - Fetch transactions.
   - Map and upsert each transaction.
   - Upsert sync state with `last_synced_at = clock.Now()`.

- [ ] **Step 5: Verify green**

Run: `go test ./internal/ingest -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ingest
git commit -m "Add Akahu sync use case"
```

---

### Task 8: Confirm Transaction Upsert Correction Behaviour

**Files:**
- Modify: `internal/storage/postgres/repos_test.go`
- Modify: `internal/storage/postgres/txn.go` only if test fails for the wrong reason

- [ ] **Step 1: Add failing/guarding integration test**

Add a test that:
1. Inserts transaction `txn-correction` with amount `10.00`, posted date `2026-05-01`, raw JSON `{"version":1}`.
2. Upserts same id with amount `99.00`, posted date `2026-05-02`, raw JSON `{"version":2}`.
3. Reads it back.
4. Asserts amount remains `10.00`, posted date remains `2026-05-01`, raw JSON becomes `{"version":2}`.

- [ ] **Step 2: Run**

Run: `set -a; . ./.env; set +a; go test -tags=integration ./internal/storage/postgres -run TestTxnRepoPreservesStableFieldsOnCorrection -count=1`

Expected: PASS if M1 already implemented this correctly; FAIL if not.

- [ ] **Step 3: Fix only if needed**

Do not update `amount`, `posted_at`, or `created_at` on conflict. Only update `raw_json` and `updated_at`.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/repos_test.go internal/storage/postgres/txn.go
git commit -m "Test transaction correction upsert behaviour"
```

---

### Task 9: Wire `finance sync`

**Files:**
- Create: `cmd/cli/sync.go`
- Create: `cmd/cli/sync_test.go`
- Modify: `cmd/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Tests:
- `finance sync --from 2026-01-02` parses date and calls injected runner with that date.
- Invalid date returns an error containing `--from must be YYYY-MM-DD`.
- No `--from` passes nil.

Introduce a package-level `syncRunner` function variable for test injection:

```go
var syncRunner = runSync
```

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run TestSyncCommand -count=1`

Expected: FAIL with missing sync command.

- [ ] **Step 3: Implement command**

`newSyncCommand(stdout, stderr io.Writer)`:
- `Use: "sync"`
- flag `--from`
- parse date with layout `2006-01-02`
- call `syncRunner(cmd.Context(), syncOptions{From: parsed})`

`runSync`:
- Opens app DB from `DATABASE_URL_APP` / `DATABASE_URL`.
- Constructs Postgres account/txn/sync repos.
- Constructs `akahu.EnvTokenStore`.
- Constructs Akahu client using tokens and `AKAHU_BASE_URL` default `https://api.akahu.io/v1`.
- Calls `ingest.Sync` with `domain.UserID(1)`.

- [ ] **Step 4: Verify green**

Run: `go test ./cmd/cli -run TestSyncCommand -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/sync.go cmd/cli/sync_test.go cmd/cli/root.go
git commit -m "Add sync CLI command"
```

---

### Task 10: Extend Health with Optional Akahu Check

**Files:**
- Modify: `cmd/cli/health.go`
- Modify: `cmd/cli/health_test.go`

- [ ] **Step 1: Write failing tests**

Tests:
- Without Akahu tokens, health still checks DB only and prints `ok`.
- With Akahu tokens, health calls injected Akahu reachability checker.
- If Akahu checker fails, health returns non-zero error.

- [ ] **Step 2: Verify red**

Run: `go test ./cmd/cli -run TestHealth -count=1`

Expected: FAIL for missing Akahu checker injection.

- [ ] **Step 3: Implement**

Add package variable:

```go
var checkAkahuHealth = func(ctx context.Context) error {
	// Construct client from env tokens/base URL and call ListAccounts.
}
```

In health command:
- DB ping remains mandatory.
- If both `AKAHU_APP_TOKEN` and `AKAHU_USER_TOKEN` exist, call Akahu checker.
- If missing, do not fail health.

- [ ] **Step 4: Verify green**

Run: `go test ./cmd/cli -run TestHealth -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/health.go cmd/cli/health_test.go
git commit -m "Extend health with Akahu check"
```

---

### Task 11: Add Cross-Tenant Sync Integration Test

**Files:**
- Create: `internal/ingest/sync_integration_test.go`

- [ ] **Step 1: Write integration test**

Use local Docker DB DSNs like M1 integration tests. Seed two users, use fake Akahu clients returning same raw account/txn IDs for both users, run `ingest.Sync` for user A and user B, then assert:
- User A sees only user A rows through repo calls.
- User B sees only user B rows through repo calls.
- Same Akahu IDs exist under both users due composite keys.

- [ ] **Step 2: Verify**

Run: `set -a; . ./.env; set +a; go test -tags=integration ./internal/ingest -run TestSyncCrossTenant -count=1`

Expected: PASS after Task 7 is implemented.

- [ ] **Step 3: Commit**

```bash
git add internal/ingest/sync_integration_test.go
git commit -m "Test sync cross-tenant isolation"
```

---

### Task 12: Real Smoke-Test Instructions

**Files:**
- Modify: `README.md`
- Modify: `docs/STATUS.md`

- [ ] **Step 1: Add smoke-test section to README**

Add commands:

```bash
cp .env.example .env
# Fill AKAHU_APP_TOKEN and AKAHU_USER_TOKEN in .env.
set -a; . ./.env; set +a
make db-up
make migrate
go run ./cmd/cli sync
```

Add acceptance note:
- ANZ account connected in Akahu dashboard.
- Westpac account connected in Akahu dashboard.
- Re-run `go run ./cmd/cli sync`; no code change required.

- [ ] **Step 2: Update `docs/STATUS.md` during implementation**

Set M2 implementation to `🚧 in progress` when implementation starts. Mark complete only after smoke tests and PR merge.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/STATUS.md
git commit -m "Document M2 Akahu smoke test"
```

---

### Task 13: Final Verification

**Files:**
- No planned source changes unless verification finds an issue.

- [ ] **Step 1: Run unit tests**

Run: `make test`

Expected: PASS.

- [ ] **Step 2: Run integration tests**

Run:

```bash
set -a; . ./.env; set +a
make db-up
make migrate
make test-integration
```

Expected: PASS.

- [ ] **Step 3: Run Akahu adapter tests**

Run: `go test ./internal/akahu -count=1`

Expected: PASS.

- [ ] **Step 4: Run ingest tests**

Run: `go test ./internal/ingest -count=1`

Expected: PASS.

- [ ] **Step 5: Run lint if installed**

Run: `make lint`

Expected: PASS, or record local blocker if `golangci-lint` is unavailable.

- [ ] **Step 6: Manual smoke**

With real Akahu tokens in `.env`:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli sync
go run ./cmd/cli sync
```

Expected:
- First run populates accounts and transactions.
- Second run creates no duplicate rows.
- No token appears in stdout/stderr.

- [ ] **Step 7: Westpac smoke acceptance**

After the user connects Westpac in Akahu dashboard:

```bash
set -a; . ./.env; set +a
go run ./cmd/cli sync
```

Expected: Westpac accounts/transactions appear without code changes.

---

## Placeholder Scan

- No `akahu_tokens` table in M2.
- No categorisation/rules loader in M2.
- No env reads outside `cmd/cli` and `internal/akahu`.
- No token values in committed `.env.example`.
- No transaction amount, merchant, description, or raw JSON in log/error examples.
- Plan intentionally continues M1 `database/sql` repos rather than forcing sqlc generation midstream.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-06-M2-akahu-ingest-plan.md`.

Preferred execution:

**One-task-per-session subagent-driven execution.**

For each session, execute one task only, run its review gates, commit that task, and stop. The next chat/session resumes from the next unchecked task.
