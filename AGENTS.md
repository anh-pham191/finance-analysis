# AGENTS.md

Entry point for AI agents (Claude Code, Cursor, Copilot, Aider, etc.) working on this repository.

## Read these in order, every time

1. [`docs/STATUS.md`](docs/STATUS.md) — what state the project is in and what action is expected next. **Always check this first.**
2. [`docs/superpowers/specs/2026-05-06-finance-analysis-design.md`](docs/superpowers/specs/2026-05-06-finance-analysis-design.md) — full design spec; source of truth for what we're building and why.
3. [`docs/architecture/overview.md`](docs/architecture/overview.md) — the *why* behind the package layout.
4. [`docs/architecture/security.md`](docs/architecture/security.md) — multi-tenancy invariants, secret handling, encryption, deletion.
5. The milestone you're working on: [`docs/milestones/M{n}-*.md`](docs/milestones/) — full context to act in isolation.

## Project at a glance

- **Language:** Go (1.23+)
- **Module path:** `github.com/anh-pham191/finance-analysis`
- **Database:** Postgres 16 (local via `docker-compose`)
- **Layout:** hexagonal — `cmd/` (delivery), `internal/domain` (pure types), `internal/{ingest,categorise,report,render}` (use cases), `internal/{akahu,storage/postgres}` (adapters)
- **Single-user runtime today, multi-tenant data model from day 1.** See spec §4 and security doc.
- **TDD throughout.** Red → green → refactor.

## Hard rules (non-negotiable)

These are derived from [anh-pham191/development-rule](https://github.com/anh-pham191/development-rule). Read that repo for full context. The rules below are the local enforcement contract.

### Safety

1. **Never commit without explicit human approval.** Stage and show the diff; wait for approval.
2. **Never modify tests without permission.** Tests are specifications. Failing test ≠ broken test.
3. **Pause before destructive operations.** `rm -rf`, `git reset --hard`, `git push --force`, `DROP`, `DELETE` without `WHERE`, dropping branches, force-pushing — all require explicit approval each time. Prior approval does not extend to future operations.
4. **No skipping hooks** (`--no-verify`, `--no-gpg-sign`) unless the user explicitly asks.
5. **Investigate before discarding** unfamiliar files, branches, or configuration. They may be in-progress work.

### Security

6. **No secret in any committed file.** Includes `.env`, real Akahu tokens, master keys, password hashes in fixtures, real txn data in fixtures.
7. **Tokens never appear in logs or error messages.** Slog redaction is mandatory; verify via test.
8. **No transaction PII in logs at any level.** Counts only. Amounts, descriptions, merchants stay in DB.
9. **Outside `cmd/` and `internal/akahu/`, no package reads env vars directly.** Config is loaded once at the boundary and passed in.

### Multi-tenancy invariants (apply from M1, even with one user)

10. **Every user-owned table has `user_id` and `ON DELETE CASCADE` from `users`.** Migrations that omit either are wrong.
11. **Every repository method takes `userID UserID` as the first non-context arg.** Per-aggregate ports (`AccountRepo`, `TxnRepo`, etc.) — not a single `Repository`. No overload omits `userID`.
12. **Postgres RLS is FORCED on every user-owned table.** Without `FORCE`, table owners bypass. The app connects as `finance_app` (non-owner, no `BYPASSRLS`).
13. **Every repo call runs inside `withUserTx`.** That helper is the single point that opens a tx and `SET LOCAL app.user_id = $userID`. Bypassing it silently breaks RLS.
14. **Every new repository method ships with a cross-tenant integration test** asserting user A cannot read or write user B's rows. See spec §11.
15. **`internal/archtest`** enforces the package import graph from M1. Don't disable it; fix the import.

### Code quality

16. **YAGNI ruthlessly.** No abstractions, options, or flags that aren't needed by the current milestone. Three similar lines is better than a premature abstraction.
17. **Composable over monolithic.** One clear purpose per package. If a file is hard to hold in your head, it's doing too much.
18. **No comments unless the *why* is non-obvious.** Don't narrate what the code does.
19. **No dead code, no half-finished implementations,** no backwards-compat shims for things that don't yet exist.

### Workflow

20. **TDD.** Write the failing test first. Make it pass with the smallest change. Refactor with the test green.
21. **Branch off `develop`, PR into `develop`.** Never commit directly to `main` or `develop`. Branch names are `feature/<slug>`, `fix/<slug>`, `docs/<slug>`, or `chore/<slug>`. Releases happen via `develop → main` PRs. Full convention: [`docs/process/branching.md`](docs/process/branching.md).
22. **Red flags that mean "stop and ask":** the task seems to require editing an existing test to pass; the task seems to require relaxing a security invariant; the task seems to need a new top-level package outside the layout in `docs/architecture/overview.md`; the task seems to span multiple milestones at once; the task seems to require disabling `archtest`.
23. **NZ English** in docs and user-facing copy ("categorise", "behaviour", "colour").

### Implementation cadence

- Execute milestone implementation plans **one task per chat/session** by default. Do not run a 10+ task milestone as one long continuous agent session unless the user explicitly asks for that.
- Each task ends with: red/green verification evidence, affected package tests, review gates where applicable, staged diff/stat, then a task-sized commit after human approval. If the user has explicitly approved "commit after each task" for the active session, that approval applies only to that session.
- Do not start the next plan task until the current task has been committed or the user explicitly chooses to leave it uncommitted.
- Keep commits task-sized. If review feedback changes a task, include the fixes in that same task commit before moving on.

## Forbidden imports

Enforced from M1 by `internal/archtest/archtest_test.go` (fails CI on regression):

- `internal/domain/` MUST NOT import any other internal package.
- `internal/ports/` MUST NOT import any internal package other than `internal/domain/`.
- `internal/{ingest,categorise,report}/` MUST NOT import `internal/akahu/` or `internal/storage/`.
- `internal/{ingest,categorise,report,render}/` MUST NOT import `cobra` or `net/http`.
- No package outside `cmd/` and `internal/akahu/` may read environment variables directly.

## Commit conventions

- Subject in imperative mood, ≤ 72 characters. ("Add Akahu sync command", not "Added" or "Adds".)
- Body explains *why*, not *what*. Wrap at 72.
- No ticket prefix needed for this personal project; plain imperative subjects are fine.
- Co-Authored-By trailer when an AI assistant materially contributed.

## What to do when you arrive

1. Read [`docs/STATUS.md`](docs/STATUS.md) to find out what's next.
2. Read the milestone doc for the current milestone.
3. Re-read this file's hard rules and [`docs/process/branching.md`](docs/process/branching.md).
4. Confirm the plan with the user before writing code (use the `superpowers:writing-plans` skill if available, or otherwise propose a plan in chat).
5. **Branch off `develop`** with `feature/<slug>` (or `fix/<slug>` etc.).
6. Write the failing test. Then the code. Then refactor.
7. Stage changes; **do not commit** until the user approves.
8. Push the branch and open a **PR against `develop`** following the description template in `docs/process/branching.md`.

## What NOT to do

- Don't start editing code without confirming with the user which milestone you're on.
- Don't combine work from multiple milestones in a single change.
- Don't add a new dependency without flagging it explicitly in chat (size, transitive surface, why other deps don't suffice).
- Don't change the schema without a migration, and don't write a migration that omits `user_id` on user-owned tables.
- Don't log a token. Don't log an amount. Don't log a merchant.
